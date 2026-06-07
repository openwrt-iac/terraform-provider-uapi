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

const packageFeedCollection = "packages/feeds"

var (
	_ resource.Resource                = &packageFeedResource{}
	_ resource.ResourceWithConfigure   = &packageFeedResource{}
	_ resource.ResourceWithImportState = &packageFeedResource{}
)

type packageFeedResource struct {
	client *client.Client
}

func NewPackageFeedResource() resource.Resource {
	return &packageFeedResource{}
}

type packageFeedModel struct {
	ID           types.String `tfsdk:"id"`
	Managed      types.Bool   `tfsdk:"managed"`
	Name         types.String `tfsdk:"name"`
	URL          types.String `tfsdk:"url"`
	Filename     types.String `tfsdk:"filename"`
	Enabled      types.Bool   `tfsdk:"enabled"`
	UpdateStatus types.String `tfsdk:"update_status"`
}

func (r *packageFeedResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_package_feed"
}

func (r *packageFeedResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *packageFeedResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A package feed (apk repository). The feeds endpoint has no update, so changing name or url replaces the resource.",
		Attributes: map[string]schema.Attribute{
			"id":      computedIDAttribute(),
			"managed": managedAttribute(),
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "Feed name (^[A-Za-z0-9_.-]+$); becomes <name>.list.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"url": schema.StringAttribute{
				Required:      true,
				Description:   "HTTP(S) URL of the package repository.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"filename": schema.StringAttribute{
				Computed:    true,
				Description: "Feed list filename on the router.",
			},
			"enabled": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the feed is enabled.",
			},
			"update_status": schema.StringAttribute{
				Computed:    true,
				Description: "Status of the last feed update.",
			},
		},
	}
}

func (r *packageFeedResource) read(_ context.Context, obj map[string]any, m *packageFeedModel) {
	m.ID = strVal(obj, "id")
	m.Managed = boolVal(obj, "managed")
	m.Name = strVal(obj, "name")
	m.URL = strVal(obj, "url")
	m.Filename = strVal(obj, "filename")
	m.Enabled = boolVal(obj, "enabled")
	m.UpdateStatus = strVal(obj, "update_status")
}

func (r *packageFeedResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan packageFeedModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := map[string]any{"name": plan.Name.ValueString(), "url": plan.URL.ValueString()}
	obj, _, err := r.client.Post(ctx, "/"+packageFeedCollection, body, "")
	if err != nil {
		resp.Diagnostics.AddError("Error creating package feed", err.Error())
		return
	}
	r.read(ctx, obj, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *packageFeedResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state packageFeedModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, _, found, err := r.client.GetObject(ctx, "/"+packageFeedCollection+"/"+state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading package feed", err.Error())
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
func (r *packageFeedResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan packageFeedModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, _, found, err := r.client.GetObject(ctx, "/"+packageFeedCollection+"/"+plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading package feed", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	r.read(ctx, obj, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *packageFeedResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state packageFeedModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/"+packageFeedCollection+"/"+state.ID.ValueString(), ""); err != nil {
		resp.Diagnostics.AddError("Error deleting package feed", err.Error())
	}
}

func (r *packageFeedResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
