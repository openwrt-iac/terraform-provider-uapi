package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/raspbeguy/terraform-provider-uapi/internal/client"
)

const networkInterfaceCollection = "network/interfaces"

var (
	_ resource.Resource                = &networkInterfaceResource{}
	_ resource.ResourceWithConfigure   = &networkInterfaceResource{}
	_ resource.ResourceWithImportState = &networkInterfaceResource{}
)

type networkInterfaceResource struct {
	client *client.Client
}

func NewNetworkInterfaceResource() resource.Resource {
	return &networkInterfaceResource{}
}

type networkInterfaceModel struct {
	ID        types.String `tfsdk:"id"`
	Managed   types.Bool   `tfsdk:"managed"`
	Device    types.String `tfsdk:"device"`
	Proto     types.String `tfsdk:"proto"`
	IPAddr    types.String `tfsdk:"ipaddr"`
	Netmask   types.String `tfsdk:"netmask"`
	Gateway   types.String `tfsdk:"gateway"`
	DNS       types.List   `tfsdk:"dns"`
	IP6Assign types.String `tfsdk:"ip6assign"`
	MTU       types.String `tfsdk:"mtu"`
	Auto      types.Bool   `tfsdk:"auto"`
}

func (r *networkInterfaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network_interface"
}

func (r *networkInterfaceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *networkInterfaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A network interface (uci network.interface). " +
			"Caution: reloading an interface that backs your management connection can lock you out; " +
			"uapi only observes the init script exit code, not runtime convergence.",
		Attributes: map[string]schema.Attribute{
			"id":      computedIDAttribute(),
			"managed": managedAttribute(),
			"device": schema.StringAttribute{
				Optional:    true,
				Description: "Underlying device this interface binds to.",
			},
			"proto": schema.StringAttribute{
				Required:    true,
				Description: "Protocol: static, dhcp, dhcpv6, pppoe, none, ppp, or wwan.",
			},
			"ipaddr":    optionalComputedString("IPv4 address (required when proto is static)."),
			"netmask":   optionalComputedString("IPv4 netmask."),
			"gateway":   optionalComputedString("Default gateway."),
			"dns":       optionalComputedStringList("DNS servers."),
			"ip6assign": optionalComputedString("IPv6 prefix assignment length."),
			"mtu":       optionalComputedString("Interface MTU."),
			"auto":      optionalComputedBool("Bring the interface up automatically. Defaults to true."),
		},
	}
}

func (r *networkInterfaceResource) body(ctx context.Context, m networkInterfaceModel, diags *diagsink) map[string]any {
	out := map[string]any{}
	putStr(out, "device", m.Device)
	putStr(out, "proto", m.Proto)
	putStr(out, "ipaddr", m.IPAddr)
	putStr(out, "netmask", m.Netmask)
	putStr(out, "gateway", m.Gateway)
	putList(ctx, out, "dns", m.DNS, diags.d)
	putStr(out, "ip6assign", m.IP6Assign)
	putStr(out, "mtu", m.MTU)
	putBool(out, "auto", m.Auto)
	return out
}

func (r *networkInterfaceResource) read(ctx context.Context, obj map[string]any, m *networkInterfaceModel, diags *diagsink) {
	m.ID = strVal(obj, "id")
	m.Managed = boolVal(obj, "managed")
	m.Device = strVal(obj, "device")
	m.Proto = strVal(obj, "proto")
	m.IPAddr = strVal(obj, "ipaddr")
	m.Netmask = strVal(obj, "netmask")
	m.Gateway = strVal(obj, "gateway")
	m.DNS = diags.list(listVal(ctx, obj, "dns"))
	m.IP6Assign = strVal(obj, "ip6assign")
	m.MTU = strVal(obj, "mtu")
	m.Auto = boolVal(obj, "auto")
}

func (r *networkInterfaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan networkInterfaceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ds := newDiagsink(&resp.Diagnostics)
	body := r.body(ctx, plan, ds)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, err := r.client.Post(ctx, "/"+networkInterfaceCollection, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating network interface", err.Error())
		return
	}
	r.read(ctx, obj, &plan, ds)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *networkInterfaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state networkInterfaceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, found, err := r.client.GetObject(ctx, "/"+networkInterfaceCollection+"/"+state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading network interface", err.Error())
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

func (r *networkInterfaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan networkInterfaceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ds := newDiagsink(&resp.Diagnostics)
	body := r.body(ctx, plan, ds)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, err := r.client.Put(ctx, "/"+networkInterfaceCollection+"/"+plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating network interface", err.Error())
		return
	}
	r.read(ctx, obj, &plan, ds)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *networkInterfaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state networkInterfaceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/"+networkInterfaceCollection+"/"+state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting network interface", err.Error())
	}
}

func (r *networkInterfaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := resolveImportID(ctx, r.client, networkInterfaceCollection, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing network interface", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
