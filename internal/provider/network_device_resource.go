package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/raspbeguy/terraform-provider-uapi/internal/client"
)

const networkDeviceCollection = "network/devices"

var (
	_ resource.Resource                = &networkDeviceResource{}
	_ resource.ResourceWithConfigure   = &networkDeviceResource{}
	_ resource.ResourceWithImportState = &networkDeviceResource{}
)

type networkDeviceResource struct {
	client *client.Client
}

func NewNetworkDeviceResource() resource.Resource {
	return &networkDeviceResource{}
}

type networkDeviceModel struct {
	ID      types.String `tfsdk:"id"`
	Managed types.Bool   `tfsdk:"managed"`
	Name    types.String `tfsdk:"name"`
	Type    types.String `tfsdk:"type"`
	Ports   types.List   `tfsdk:"ports"`
	VID     types.String `tfsdk:"vid"`
	Ifname  types.String `tfsdk:"ifname"`
	MTU     types.String `tfsdk:"mtu"`
	MacAddr types.String `tfsdk:"macaddr"`
	IPv6    types.Bool   `tfsdk:"ipv6"`
}

func (r *networkDeviceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network_device"
}

func (r *networkDeviceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *networkDeviceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A network device such as a bridge or VLAN (uci network.device).",
		Attributes: map[string]schema.Attribute{
			"id":      computedIDAttribute(),
			"managed": managedAttribute(),
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Device name (e.g. br-lan).",
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "Device type: bridge, 8021q, 8021ad, macvlan, veth, tun, or tap.",
			},
			"ports":   optionalComputedStringList("Member interfaces (required when type is bridge)."),
			"vid":     optionalComputedString("VLAN id (required when type is 8021q)."),
			"ifname":  optionalComputedString("Base interface name for VLAN/macvlan devices."),
			"mtu":     optionalComputedString("Device MTU."),
			"macaddr": optionalComputedString("Override MAC address."),
			"ipv6":    optionalComputedBool("Enable IPv6 on the device. Defaults to true."),
		},
	}
}

func (r *networkDeviceResource) body(ctx context.Context, m networkDeviceModel, diags *diagsink) map[string]any {
	out := map[string]any{}
	putStr(out, "name", m.Name)
	putStr(out, "type", m.Type)
	putList(ctx, out, "ports", m.Ports, diags.d)
	putStr(out, "vid", m.VID)
	putStr(out, "ifname", m.Ifname)
	putStr(out, "mtu", m.MTU)
	putStr(out, "macaddr", m.MacAddr)
	putBool(out, "ipv6", m.IPv6)
	return out
}

func (r *networkDeviceResource) read(ctx context.Context, obj map[string]any, m *networkDeviceModel, diags *diagsink) {
	m.ID = strVal(obj, "id")
	m.Managed = boolVal(obj, "managed")
	m.Name = strVal(obj, "name")
	m.Type = strVal(obj, "type")
	m.Ports = diags.list(listVal(ctx, obj, "ports"))
	m.VID = strVal(obj, "vid")
	m.Ifname = strVal(obj, "ifname")
	m.MTU = strVal(obj, "mtu")
	m.MacAddr = strVal(obj, "macaddr")
	m.IPv6 = boolVal(obj, "ipv6")
}

func (r *networkDeviceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan networkDeviceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ds := newDiagsink(&resp.Diagnostics)
	body := r.body(ctx, plan, ds)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, err := r.client.Post(ctx, "/"+networkDeviceCollection, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating network device", err.Error())
		return
	}
	r.read(ctx, obj, &plan, ds)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *networkDeviceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state networkDeviceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, found, err := r.client.GetObject(ctx, "/"+networkDeviceCollection+"/"+state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading network device", err.Error())
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

func (r *networkDeviceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan networkDeviceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ds := newDiagsink(&resp.Diagnostics)
	body := r.body(ctx, plan, ds)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, err := r.client.Put(ctx, "/"+networkDeviceCollection+"/"+plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating network device", err.Error())
		return
	}
	r.read(ctx, obj, &plan, ds)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *networkDeviceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state networkDeviceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/"+networkDeviceCollection+"/"+state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting network device", err.Error())
	}
}

func (r *networkDeviceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := resolveImportID(ctx, r.client, networkDeviceCollection, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing network device", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
