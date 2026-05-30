package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/raspbeguy/terraform-provider-uapi/internal/client"
)

const firewallRedirectCollection = "firewall/redirects"

var (
	_ resource.Resource                = &firewallRedirectResource{}
	_ resource.ResourceWithConfigure   = &firewallRedirectResource{}
	_ resource.ResourceWithImportState = &firewallRedirectResource{}
)

type firewallRedirectResource struct {
	client *client.Client
}

func NewFirewallRedirectResource() resource.Resource {
	return &firewallRedirectResource{}
}

type firewallRedirectModel struct {
	ID      types.String           `tfsdk:"id"`
	Managed types.Bool             `tfsdk:"managed"`
	Name    types.String           `tfsdk:"name"`
	Target  types.String           `tfsdk:"target"`
	Enabled types.Bool             `tfsdk:"enabled"`
	Match   *firewallRedirectMatch `tfsdk:"match"`
}

type firewallRedirectMatch struct {
	SrcZone  types.String `tfsdk:"src_zone"`
	DestZone types.String `tfsdk:"dest_zone"`
	SrcIP    types.List   `tfsdk:"src_ip"`
	SrcPort  types.List   `tfsdk:"src_port"`
	SrcDport types.List   `tfsdk:"src_dport"`
	DestIP   types.List   `tfsdk:"dest_ip"`
	DestPort types.List   `tfsdk:"dest_port"`
	Proto    types.List   `tfsdk:"proto"`
	Family   types.String `tfsdk:"family"`
}

func (r *firewallRedirectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_firewall_redirect"
}

func (r *firewallRedirectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *firewallRedirectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A firewall redirect / port forward (uci firewall.redirect).",
		Attributes: map[string]schema.Attribute{
			"id":      computedIDAttribute(),
			"managed": managedAttribute(),
			"name": schema.StringAttribute{
				Optional:    true,
				Description: "Optional human-readable redirect name.",
			},
			"target":  optionalComputedString("Redirect type: DNAT or SNAT. Defaults to DNAT."),
			"enabled": optionalComputedBool("Whether the redirect is active. Defaults to true."),
			"match": schema.SingleNestedAttribute{
				Required:    true,
				Description: "Match conditions for the redirect.",
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
					"src_port":  optionalComputedStringList("Source ports."),
					"src_dport": optionalComputedStringList("Incoming (destination) ports to redirect."),
					"dest_ip":   optionalComputedStringList("Internal destination IP addresses."),
					"dest_port": optionalComputedStringList("Internal destination ports."),
					"proto":     optionalComputedStringList("Protocols: tcp, udp, icmp, icmpv6, esp, ah, any, all."),
					"family":    optionalComputedString("Address family: any, ipv4, or ipv6. Defaults to any."),
				},
			},
		},
	}
}

func (r *firewallRedirectResource) body(ctx context.Context, m firewallRedirectModel, diags *diagsink) map[string]any {
	out := map[string]any{}
	putStr(out, "name", m.Name)
	putStr(out, "target", m.Target)
	putBool(out, "enabled", m.Enabled)
	match := map[string]any{}
	if m.Match != nil {
		putStr(match, "src_zone", m.Match.SrcZone)
		putStr(match, "dest_zone", m.Match.DestZone)
		putList(ctx, match, "src_ip", m.Match.SrcIP, diags.d)
		putList(ctx, match, "src_port", m.Match.SrcPort, diags.d)
		putList(ctx, match, "src_dport", m.Match.SrcDport, diags.d)
		putList(ctx, match, "dest_ip", m.Match.DestIP, diags.d)
		putList(ctx, match, "dest_port", m.Match.DestPort, diags.d)
		putList(ctx, match, "proto", m.Match.Proto, diags.d)
		putStr(match, "family", m.Match.Family)
	}
	out["match"] = match
	return out
}

func (r *firewallRedirectResource) read(ctx context.Context, obj map[string]any, m *firewallRedirectModel, diags *diagsink) {
	m.ID = strVal(obj, "id")
	m.Managed = boolVal(obj, "managed")
	m.Name = strVal(obj, "name")
	m.Target = strVal(obj, "target")
	m.Enabled = boolVal(obj, "enabled")

	match, _ := obj["match"].(map[string]any)
	if match == nil {
		match = map[string]any{}
	}
	mm := &firewallRedirectMatch{
		SrcZone:  strVal(match, "src_zone"),
		DestZone: strVal(match, "dest_zone"),
		Family:   strVal(match, "family"),
	}
	mm.SrcIP = diags.list(listVal(ctx, match, "src_ip"))
	mm.SrcPort = diags.list(listVal(ctx, match, "src_port"))
	mm.SrcDport = diags.list(listVal(ctx, match, "src_dport"))
	mm.DestIP = diags.list(listVal(ctx, match, "dest_ip"))
	mm.DestPort = diags.list(listVal(ctx, match, "dest_port"))
	mm.Proto = diags.list(listVal(ctx, match, "proto"))
	m.Match = mm
}

func (r *firewallRedirectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan firewallRedirectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ds := newDiagsink(&resp.Diagnostics)
	body := r.body(ctx, plan, ds)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, err := r.client.Post(ctx, "/"+firewallRedirectCollection, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating firewall redirect", err.Error())
		return
	}
	r.read(ctx, obj, &plan, ds)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *firewallRedirectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state firewallRedirectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, found, err := r.client.GetObject(ctx, "/"+firewallRedirectCollection+"/"+state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading firewall redirect", err.Error())
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

func (r *firewallRedirectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan firewallRedirectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ds := newDiagsink(&resp.Diagnostics)
	body := r.body(ctx, plan, ds)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, err := r.client.Put(ctx, "/"+firewallRedirectCollection+"/"+plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating firewall redirect", err.Error())
		return
	}
	r.read(ctx, obj, &plan, ds)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *firewallRedirectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state firewallRedirectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/"+firewallRedirectCollection+"/"+state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting firewall redirect", err.Error())
	}
}

func (r *firewallRedirectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := resolveImportID(ctx, r.client, firewallRedirectCollection, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing firewall redirect", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
