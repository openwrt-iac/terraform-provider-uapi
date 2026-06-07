package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/openwrt-iac/terraform-provider-uapi/internal/client"
)

var (
	_ datasource.DataSource              = &whoamiDataSource{}
	_ datasource.DataSourceWithConfigure = &whoamiDataSource{}
)

type whoamiDataSource struct{ client *client.Client }

func NewWhoamiDataSource() datasource.DataSource { return &whoamiDataSource{} }

type whoamiModel struct {
	TokenID      types.String `tfsdk:"token_id"`
	Scopes       types.List   `tfsdk:"scopes"`
	SourceIP     types.String `tfsdk:"source_ip"`
	ExpiresAt    types.Int64  `tfsdk:"expires_at"`
	AllowedCIDRs types.List   `tfsdk:"allowed_cidrs"`
	LastUsedAt   types.Int64  `tfsdk:"last_used_at"`
	LastUsedIP   types.String `tfsdk:"last_used_ip"`
}

func (d *whoamiDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_whoami"
}

func (d *whoamiDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *whoamiDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Identity and scopes of the bearer token the provider is configured with.",
		Attributes: map[string]dsschema.Attribute{
			"token_id":      dsComputedString("Name of the calling token."),
			"scopes":        dsComputedStringList("Scopes granted to the token."),
			"source_ip":     dsComputedString("Source IP of this request as seen by the router."),
			"expires_at":    dsComputedInt64("Unix epoch seconds when the token expires, if set."),
			"allowed_cidrs": dsComputedStringList("Source CIDRs the token is restricted to."),
			"last_used_at":  dsComputedInt64("Unix epoch seconds of the token's previous use, if any."),
			"last_used_ip":  dsComputedString("Source IP of the token's previous use, if any."),
		},
	}
}

func (d *whoamiDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	obj, _, found, err := d.client.GetObject(ctx, "/auth/whoami")
	if err != nil {
		resp.Diagnostics.AddError("Error reading whoami", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Error reading whoami", "endpoint returned not found")
		return
	}
	scopes, sd := listVal(ctx, obj, "scopes")
	resp.Diagnostics.Append(sd...)
	cidrs, cd := listVal(ctx, obj, "allowed_cidrs")
	resp.Diagnostics.Append(cd...)
	out := whoamiModel{
		TokenID:      strVal(obj, "token_id"),
		Scopes:       scopes,
		SourceIP:     strVal(obj, "source_ip"),
		ExpiresAt:    int64Val(obj, "expires_at"),
		AllowedCIDRs: cidrs,
		LastUsedAt:   int64Val(obj, "last_used_at"),
		LastUsedIP:   strVal(obj, "last_used_ip"),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
