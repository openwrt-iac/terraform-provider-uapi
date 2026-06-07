package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	ephschema "github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/openwrt-iac/terraform-provider-uapi/internal/client"
)

var (
	_ ephemeral.EphemeralResource              = &tokenEphemeral{}
	_ ephemeral.EphemeralResourceWithConfigure = &tokenEphemeral{}
	_ ephemeral.EphemeralResourceWithClose     = &tokenEphemeral{}
)

// tokenEphemeral mints a uapi bearer token for the duration of the run and
// revokes it on Close. Ephemeral because a token is a short-lived credential
// that should never be persisted in Terraform state.
type tokenEphemeral struct{ client *client.Client }

func NewTokenEphemeral() ephemeral.EphemeralResource { return &tokenEphemeral{} }

type tokenEphemeralModel struct {
	Name             types.String `tfsdk:"name"`
	Scopes           types.List   `tfsdk:"scopes"`
	ExpiresInSeconds types.Int64  `tfsdk:"expires_in_seconds"`
	AllowedCIDRs     types.List   `tfsdk:"allowed_cidrs"`
	ID               types.String `tfsdk:"id"`
	Token            types.String `tfsdk:"token"`
}

func (e *tokenEphemeral) Metadata(_ context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_token"
}

func (e *tokenEphemeral) Configure(_ context.Context, req ephemeral.ConfigureRequest, resp *ephemeral.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	if c, ok := req.ProviderData.(*client.Client); ok {
		e.client = c
	}
}

func (e *tokenEphemeral) Schema(_ context.Context, _ ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = ephschema.Schema{
		Description: "Mints a uapi bearer token for the lifetime of the operation and revokes it afterward. " +
			"The cleartext token is never written to state.",
		Attributes: map[string]ephschema.Attribute{
			"name": ephschema.StringAttribute{
				Required:    true,
				Description: "Token name (matches [A-Za-z0-9_][A-Za-z0-9_-]{0,62}). Doubles as the token id.",
			},
			"scopes": ephschema.ListAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: "Scopes for the new token (must be a subset of the caller's scopes).",
			},
			"expires_in_seconds": ephschema.Int64Attribute{
				Optional:    true,
				Description: "Token lifetime in seconds.",
			},
			"allowed_cidrs": ephschema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Source CIDRs the token is restricted to (empty = any).",
			},
			"id": ephschema.StringAttribute{
				Computed:    true,
				Description: "Token id (equal to name; used to revoke on close).",
			},
			"token": ephschema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "The cleartext bearer token. Returned exactly once.",
			},
		},
	}
}

func (e *tokenEphemeral) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var m tokenEphemeralModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := map[string]any{}
	ds := newDiagsink(&resp.Diagnostics)
	putStr(body, "name", m.Name)
	putList(ctx, body, "scopes", m.Scopes, ds.d)
	putInt64(body, "expires_in_seconds", m.ExpiresInSeconds)
	putList(ctx, body, "allowed_cidrs", m.AllowedCIDRs, ds.d)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, _, err := e.client.Post(ctx, "/tokens", body, "")
	if err != nil {
		resp.Diagnostics.AddError("Error minting token", err.Error())
		return
	}
	m.ID = strVal(obj, "name")
	m.Token = strVal(obj, "bearer")
	resp.Diagnostics.Append(resp.Result.Set(ctx, &m)...)
	idJSON, _ := json.Marshal(m.ID.ValueString())
	resp.Diagnostics.Append(resp.Private.SetKey(ctx, "token_id", idJSON)...)
}

func (e *tokenEphemeral) Close(ctx context.Context, req ephemeral.CloseRequest, resp *ephemeral.CloseResponse) {
	raw, diags := req.Private.GetKey(ctx, "token_id")
	resp.Diagnostics.Append(diags...)
	if len(raw) == 0 {
		return
	}
	var id string
	if err := json.Unmarshal(raw, &id); err != nil || id == "" {
		return
	}
	if err := e.client.Delete(ctx, "/tokens/"+id, ""); err != nil {
		resp.Diagnostics.AddError("Error revoking token", err.Error())
	}
}
