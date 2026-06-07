package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/openwrt-iac/terraform-provider-uapi/internal/client"
)

const packageCollection = "packages/installed"

var (
	_ resource.Resource                = &packageResource{}
	_ resource.ResourceWithConfigure   = &packageResource{}
	_ resource.ResourceWithImportState = &packageResource{}
)

type packageResource struct {
	client *client.Client
}

func NewPackageResource() resource.Resource {
	return &packageResource{}
}

type packageModel struct {
	ID         types.String `tfsdk:"id"`
	Managed    types.Bool   `tfsdk:"managed"`
	Name       types.String `tfsdk:"name"`
	Version    types.String `tfsdk:"version"`
	Installed  types.Bool   `tfsdk:"installed"`
	PreExisted types.Bool   `tfsdk:"pre_existed"`
}

func (r *packageResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_package"
}

func (r *packageResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *packageResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An installed apk package. The packages endpoint has no update, so changing name replaces the resource.",
		Attributes: map[string]schema.Attribute{
			"id":      computedIDAttribute(),
			"managed": managedAttribute(),
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "apk package name (^[A-Za-z0-9_+.-]+$).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"version": schema.StringAttribute{
				Computed:    true,
				Description: "Installed package version.",
			},
			"installed": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the package is installed.",
			},
			"pre_existed": schema.BoolAttribute{
				Computed: true,
				Description: "Whether the package was already installed before Terraform managed it. " +
					"When true, `terraform destroy` does NOT uninstall it (it was not installed by this resource).",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *packageResource) read(_ context.Context, obj map[string]any, m *packageModel) {
	m.ID = strVal(obj, "id")
	m.Managed = boolVal(obj, "managed")
	m.Name = strVal(obj, "name")
	m.Version = strVal(obj, "version")
	m.Installed = boolVal(obj, "installed")
}

func (r *packageResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan packageModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Record whether the package pre-existed so Delete can avoid uninstalling
	// something Terraform did not install. POST is idempotent on an already-
	// installed package, so we install regardless.
	_, _, found, err := r.client.GetObject(ctx, "/"+packageCollection+"/"+plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error checking package", err.Error())
		return
	}
	preExisted := found
	obj, _, err := r.client.Post(ctx, "/"+packageCollection, map[string]any{"name": plan.Name.ValueString()}, "")
	if err != nil {
		resp.Diagnostics.AddError("Error installing package", err.Error())
		return
	}
	r.read(ctx, obj, &plan)
	plan.PreExisted = types.BoolValue(preExisted)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *packageResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state packageModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, _, found, err := r.client.GetObject(ctx, "/"+packageCollection+"/"+state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading package", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	r.read(ctx, obj, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update has no wire endpoint: every caller-owned field forces replacement, so
// this only re-reads current state and is effectively never invoked.
func (r *packageResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan packageModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, _, found, err := r.client.GetObject(ctx, "/"+packageCollection+"/"+plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading package", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	r.read(ctx, obj, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *packageResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state packageModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Do not uninstall a package that was already present before Terraform
	// managed it; destroying the resource just stops managing it.
	if state.PreExisted.ValueBool() {
		return
	}
	if err := r.client.Delete(ctx, "/"+packageCollection+"/"+state.ID.ValueString(), ""); err != nil {
		resp.Diagnostics.AddError("Error removing package", err.Error())
	}
}

func (r *packageResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
