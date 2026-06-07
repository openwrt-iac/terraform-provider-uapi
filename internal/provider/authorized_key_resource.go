package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/openwrt-iac/terraform-provider-uapi/internal/client"
)

const authorizedKeyCollection = "system/authorized_keys"

var (
	_ resource.Resource                = &authorizedKeyResource{}
	_ resource.ResourceWithConfigure   = &authorizedKeyResource{}
	_ resource.ResourceWithImportState = &authorizedKeyResource{}
)

type authorizedKeyResource struct {
	client *client.Client
}

func NewAuthorizedKeyResource() resource.Resource {
	return &authorizedKeyResource{}
}

type authorizedKeyModel struct {
	ID      types.String `tfsdk:"id"`
	Key     types.String `tfsdk:"key"`
	Type    types.String `tfsdk:"type"`
	Comment types.String `tfsdk:"comment"`
}

func (r *authorizedKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_authorized_key"
}

func (r *authorizedKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *authorizedKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An SSH authorized key line for a router login. There is no per-key update: changing the key replaces it.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Stable id: sha256 prefix of the public-key blob.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"key": schema.StringAttribute{
				Required:      true,
				Description:   "Full authorized_keys line: '<type> <base64> [comment]'. This is a public key, so it is not sensitive. uapi never returns the blob, so it is not recoverable on import.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"type": schema.StringAttribute{
				Computed:    true,
				Description: "SSH key type (e.g. ssh-ed25519, ssh-rsa, ecdsa-sha2-nistp256).",
			},
			"comment": schema.StringAttribute{
				Computed:    true,
				Description: "Optional comment (trailing text on the key line).",
			},
		},
	}
}

func (r *authorizedKeyResource) read(obj map[string]any, m *authorizedKeyModel) {
	m.ID = strVal(obj, "id")
	m.Type = strVal(obj, "type")
	m.Comment = strVal(obj, "comment")
}

func (r *authorizedKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan authorizedKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, _, err := r.client.Post(ctx, "/"+authorizedKeyCollection, map[string]any{"key": plan.Key.ValueString()}, "")
	if err != nil {
		writeErr(&resp.Diagnostics, "creating", "authorized key", err)
		return
	}
	r.read(obj, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *authorizedKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state authorizedKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, _, found, err := r.client.GetObject(ctx, "/"+authorizedKeyCollection+"/"+state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading authorized key", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	r.read(obj, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *authorizedKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan authorizedKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *authorizedKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state authorizedKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/"+authorizedKeyCollection+"/"+state.ID.ValueString(), ""); err != nil {
		writeErr(&resp.Diagnostics, "deleting", "authorized key", err)
	}
}

func (r *authorizedKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
