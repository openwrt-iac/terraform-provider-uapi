package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/raspbeguy/terraform-provider-uapi/internal/client"
)

const firewallZoneCollection = "firewall/zones"

var (
	_ resource.Resource                = &firewallZoneResource{}
	_ resource.ResourceWithConfigure   = &firewallZoneResource{}
	_ resource.ResourceWithImportState = &firewallZoneResource{}
)

type firewallZoneResource struct {
	client *client.Client
}

func NewFirewallZoneResource() resource.Resource {
	return &firewallZoneResource{}
}

type firewallZoneModel struct {
	ID      types.String `tfsdk:"id"`
	Managed types.Bool   `tfsdk:"managed"`
	Name    types.String `tfsdk:"name"`
	Input   types.String `tfsdk:"input"`
	Output  types.String `tfsdk:"output"`
	Forward types.String `tfsdk:"forward"`
	Network types.List   `tfsdk:"network"`
	Masq    types.Bool   `tfsdk:"masq"`
	MTUFix  types.Bool   `tfsdk:"mtu_fix"`
	Family  types.String `tfsdk:"family"`
}

func (r *firewallZoneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_firewall_zone"
}

func (r *firewallZoneResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *firewallZoneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A firewall zone (uci firewall.zone).",
		Attributes: map[string]schema.Attribute{
			"id":      computedIDAttribute(),
			"managed": managedAttribute(),
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Zone name. Used by rules/redirects to reference this zone.",
			},
			"input":   optionalComputedString("Default policy for input traffic: ACCEPT, REJECT, or DROP. Defaults to REJECT."),
			"output":  optionalComputedString("Default policy for output traffic. Defaults to REJECT."),
			"forward": optionalComputedString("Default policy for forwarded traffic. Defaults to REJECT."),
			"network": optionalComputedStringList("Network interfaces covered by this zone."),
			"masq":    optionalComputedBool("Enable masquerading (NAT). Defaults to false."),
			"mtu_fix": optionalComputedBool("Enable MSS clamping. Defaults to false."),
			"family":  optionalComputedString("Address family: any, ipv4, or ipv6. Defaults to any."),
		},
	}
}

func (r *firewallZoneResource) body(ctx context.Context, m firewallZoneModel, diags *diagsink) map[string]any {
	out := map[string]any{}
	putStr(out, "name", m.Name)
	putStr(out, "input", m.Input)
	putStr(out, "output", m.Output)
	putStr(out, "forward", m.Forward)
	putList(ctx, out, "network", m.Network, diags.d)
	putBool(out, "masq", m.Masq)
	putBool(out, "mtu_fix", m.MTUFix)
	putStr(out, "family", m.Family)
	return out
}

func (r *firewallZoneResource) read(ctx context.Context, obj map[string]any, m *firewallZoneModel, diags *diagsink) {
	m.ID = strVal(obj, "id")
	m.Managed = boolVal(obj, "managed")
	m.Name = strVal(obj, "name")
	m.Input = strVal(obj, "input")
	m.Output = strVal(obj, "output")
	m.Forward = strVal(obj, "forward")
	m.Network = diags.list(listVal(ctx, obj, "network"))
	m.Masq = boolVal(obj, "masq")
	m.MTUFix = boolVal(obj, "mtu_fix")
	m.Family = strVal(obj, "family")
}

func (r *firewallZoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan firewallZoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ds := newDiagsink(&resp.Diagnostics)
	body := r.body(ctx, plan, ds)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, err := r.client.Post(ctx, "/"+firewallZoneCollection, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating firewall zone", err.Error())
		return
	}
	r.read(ctx, obj, &plan, ds)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *firewallZoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state firewallZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, found, err := r.client.GetObject(ctx, "/"+firewallZoneCollection+"/"+state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading firewall zone", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	ds := newDiagsink(&resp.Diagnostics)
	r.read(ctx, obj, &state, ds)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *firewallZoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan firewallZoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ds := newDiagsink(&resp.Diagnostics)
	body := r.body(ctx, plan, ds)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, err := r.client.Put(ctx, "/"+firewallZoneCollection+"/"+plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating firewall zone", err.Error())
		return
	}
	r.read(ctx, obj, &plan, ds)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *firewallZoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state firewallZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/"+firewallZoneCollection+"/"+state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting firewall zone", err.Error())
	}
}

func (r *firewallZoneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := resolveImportID(ctx, r.client, firewallZoneCollection, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing firewall zone", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
