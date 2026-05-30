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

	"github.com/raspbeguy/terraform-provider-uapi/internal/client"
)

const firewallRuleCollection = "firewall/rules"

var (
	_ resource.Resource                = &firewallRuleResource{}
	_ resource.ResourceWithConfigure   = &firewallRuleResource{}
	_ resource.ResourceWithImportState = &firewallRuleResource{}
)

type firewallRuleResource struct {
	client *client.Client
}

func NewFirewallRuleResource() resource.Resource {
	return &firewallRuleResource{}
}

type firewallRuleModel struct {
	ID      types.String       `tfsdk:"id"`
	Managed types.Bool         `tfsdk:"managed"`
	Name    types.String       `tfsdk:"name"`
	Target  types.String       `tfsdk:"target"`
	Enabled types.Bool         `tfsdk:"enabled"`
	Match   *firewallRuleMatch `tfsdk:"match"`
}

type firewallRuleMatch struct {
	SrcZone  types.String `tfsdk:"src_zone"`
	DestZone types.String `tfsdk:"dest_zone"`
	SrcIP    types.List   `tfsdk:"src_ip"`
	DestIP   types.List   `tfsdk:"dest_ip"`
	SrcPort  types.List   `tfsdk:"src_port"`
	DestPort types.List   `tfsdk:"dest_port"`
	Proto    types.List   `tfsdk:"proto"`
	Family   types.String `tfsdk:"family"`
}

func (r *firewallRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_firewall_rule"
}

func (r *firewallRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *firewallRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A firewall rule (uci firewall.rule).",
		Attributes: map[string]schema.Attribute{
			"id":      computedIDAttribute(),
			"managed": managedAttribute(),
			"name": schema.StringAttribute{
				Optional:    true,
				Description: "Optional human-readable rule name.",
			},
			"target": schema.StringAttribute{
				Required:    true,
				Description: "Rule action: ACCEPT, REJECT, DROP, NOTRACK, or MARK.",
			},
			"enabled": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether the rule is active. Defaults to true.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"match": schema.SingleNestedAttribute{
				Required:    true,
				Description: "Match conditions for the rule.",
				Attributes: map[string]schema.Attribute{
					"src_zone": schema.StringAttribute{
						Required:    true,
						Description: "Source firewall zone name. Must reference an existing zone.",
					},
					"dest_zone": schema.StringAttribute{
						Optional:    true,
						Description: "Destination firewall zone name.",
					},
					"src_ip":    optionalComputedStringList("Source IP addresses or CIDRs."),
					"dest_ip":   optionalComputedStringList("Destination IP addresses or CIDRs."),
					"src_port":  optionalComputedStringList("Source ports."),
					"dest_port": optionalComputedStringList("Destination ports."),
					"proto":     optionalComputedStringList("Protocols: tcp, udp, icmp, icmpv6, esp, ah, any, all."),
					"family": schema.StringAttribute{
						Optional:      true,
						Computed:      true,
						Description:   "Address family: any, ipv4, or ipv6. Defaults to any.",
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
					},
				},
			},
		},
	}
}

func (r *firewallRuleResource) body(ctx context.Context, m firewallRuleModel, diags *diagsink) map[string]any {
	out := map[string]any{}
	putStr(out, "name", m.Name)
	putStr(out, "target", m.Target)
	putBool(out, "enabled", m.Enabled)
	match := map[string]any{}
	if m.Match != nil {
		putStr(match, "src_zone", m.Match.SrcZone)
		putStr(match, "dest_zone", m.Match.DestZone)
		putList(ctx, match, "src_ip", m.Match.SrcIP, diags.d)
		putList(ctx, match, "dest_ip", m.Match.DestIP, diags.d)
		putList(ctx, match, "src_port", m.Match.SrcPort, diags.d)
		putList(ctx, match, "dest_port", m.Match.DestPort, diags.d)
		putList(ctx, match, "proto", m.Match.Proto, diags.d)
		putStr(match, "family", m.Match.Family)
	}
	out["match"] = match
	return out
}

func (r *firewallRuleResource) read(ctx context.Context, obj map[string]any, m *firewallRuleModel, diags *diagsink) {
	m.ID = strVal(obj, "id")
	m.Managed = boolVal(obj, "managed")
	m.Name = strVal(obj, "name")
	m.Target = strVal(obj, "target")
	m.Enabled = boolVal(obj, "enabled")

	match, _ := obj["match"].(map[string]any)
	if match == nil {
		match = map[string]any{}
	}
	mm := &firewallRuleMatch{
		SrcZone:  strVal(match, "src_zone"),
		DestZone: strVal(match, "dest_zone"),
		Family:   strVal(match, "family"),
	}
	mm.SrcIP = diags.list(listVal(ctx, match, "src_ip"))
	mm.DestIP = diags.list(listVal(ctx, match, "dest_ip"))
	mm.SrcPort = diags.list(listVal(ctx, match, "src_port"))
	mm.DestPort = diags.list(listVal(ctx, match, "dest_port"))
	mm.Proto = diags.list(listVal(ctx, match, "proto"))
	m.Match = mm
}

func (r *firewallRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan firewallRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ds := newDiagsink(&resp.Diagnostics)
	body := r.body(ctx, plan, ds)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, err := r.client.Post(ctx, "/"+firewallRuleCollection, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating firewall rule", err.Error())
		return
	}
	r.read(ctx, obj, &plan, ds)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *firewallRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state firewallRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, found, err := r.client.GetObject(ctx, "/"+firewallRuleCollection+"/"+state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading firewall rule", err.Error())
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

func (r *firewallRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan firewallRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ds := newDiagsink(&resp.Diagnostics)
	body := r.body(ctx, plan, ds)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, err := r.client.Put(ctx, "/"+firewallRuleCollection+"/"+plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating firewall rule", err.Error())
		return
	}
	r.read(ctx, obj, &plan, ds)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *firewallRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state firewallRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/"+firewallRuleCollection+"/"+state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting firewall rule", err.Error())
	}
}

func (r *firewallRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := resolveImportID(ctx, r.client, firewallRuleCollection, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing firewall rule", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
