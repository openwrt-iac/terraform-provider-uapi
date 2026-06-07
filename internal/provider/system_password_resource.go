package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/openwrt-iac/terraform-provider-uapi/internal/client"
)

var (
	_ resource.Resource              = &systemPasswordResource{}
	_ resource.ResourceWithConfigure = &systemPasswordResource{}
)

type systemPasswordResource struct {
	client *client.Client
}

func NewSystemPasswordResource() resource.Resource {
	return &systemPasswordResource{}
}

type systemPasswordModel struct {
	ID                types.String `tfsdk:"id"`
	User              types.String `tfsdk:"user"`
	PasswordWO        types.String `tfsdk:"password_wo"`
	PasswordWOVersion types.String `tfsdk:"password_wo_version"`
}

func (r *systemPasswordResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_system_password"
}

func (r *systemPasswordResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *systemPasswordResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Sets the password of a local Unix user. The password is a write-only value: it is sent to uapi on apply and never stored in state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Resource identity; equals the user name.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"user": schema.StringAttribute{
				Required:      true,
				Description:   "Local Unix user to update; usually 'root'.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"password_wo": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				WriteOnly:   true,
				Description: "New password (min 8 chars). Write-only: never stored in state and never echoed back by uapi.",
			},
			"password_wo_version": schema.StringAttribute{
				Required:    true,
				Description: "Change this to re-apply the password (write-only values are not stored, so a trigger is needed).",
			},
		},
	}
}

func (r *systemPasswordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config systemPasswordModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, _, err := r.client.Post(ctx, "/system/password", map[string]any{
		"user":     plan.User.ValueString(),
		"password": config.PasswordWO.ValueString(),
	}, ""); err != nil {
		writeErr(&resp.Diagnostics, "setting", "system password", err)
		return
	}
	plan.ID = plan.User
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *systemPasswordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state systemPasswordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *systemPasswordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, config systemPasswordModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, _, err := r.client.Post(ctx, "/system/password", map[string]any{
		"user":     plan.User.ValueString(),
		"password": config.PasswordWO.ValueString(),
	}, ""); err != nil {
		writeErr(&resp.Diagnostics, "setting", "system password", err)
		return
	}
	plan.ID = plan.User
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *systemPasswordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}
