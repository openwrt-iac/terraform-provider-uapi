package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/raspbeguy/terraform-provider-uapi/internal/client"
)

const wirelessInterfaceCollection = "wireless/interfaces"

var (
	_ resource.Resource                = &wirelessInterfaceResource{}
	_ resource.ResourceWithConfigure   = &wirelessInterfaceResource{}
	_ resource.ResourceWithImportState = &wirelessInterfaceResource{}
)

type wirelessInterfaceResource struct {
	client *client.Client
}

func NewWirelessInterfaceResource() resource.Resource {
	return &wirelessInterfaceResource{}
}

type wirelessInterfaceModel struct {
	ID         types.String `tfsdk:"id"`
	Managed    types.Bool   `tfsdk:"managed"`
	Device     types.String `tfsdk:"device"`
	Network    types.String `tfsdk:"network"`
	Mode       types.String `tfsdk:"mode"`
	SSID       types.String `tfsdk:"ssid"`
	Encryption types.String `tfsdk:"encryption"`
	Disabled   types.Bool   `tfsdk:"disabled"`
	Hidden     types.Bool   `tfsdk:"hidden"`
	Isolate    types.Bool   `tfsdk:"isolate"`
	Key        types.String `tfsdk:"key"`
	HasKey     types.Bool   `tfsdk:"has_key"`
}

func (r *wirelessInterfaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_wireless_interface"
}

func (r *wirelessInterfaceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *wirelessInterfaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A wireless interface / SSID (uci wireless.wifi-iface).",
		Attributes: map[string]schema.Attribute{
			"id":      computedIDAttribute(),
			"managed": managedAttribute(),
			"device": schema.StringAttribute{
				Required:    true,
				Description: "Wireless radio id this interface belongs to.",
			},
			"network":    optionalComputedString("Network interface this SSID is bridged to."),
			"mode":       optionalComputedString("Operating mode: ap, sta, adhoc, wds, monitor, or mesh. Defaults to ap."),
			"ssid":       optionalComputedString("Network name (SSID)."),
			"encryption": optionalComputedString("Encryption suite (none, psk2, sae, wpa3, ...). Defaults to none."),
			"disabled":   optionalComputedBool("Disable this SSID. Defaults to false."),
			"hidden":     optionalComputedBool("Hide the SSID. Defaults to false."),
			"isolate":    optionalComputedBool("Isolate clients on this SSID. Defaults to false."),
			"key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Encryption passphrase. Write-only: the API never returns it, so it is not refreshed from the router. Required when encryption needs a key.",
			},
			"has_key": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether a key is configured on the router (the cleartext is never returned).",
			},
		},
	}
}

func (r *wirelessInterfaceResource) body(_ context.Context, m wirelessInterfaceModel) map[string]any {
	out := map[string]any{}
	putStr(out, "device", m.Device)
	putStr(out, "network", m.Network)
	putStr(out, "mode", m.Mode)
	putStr(out, "ssid", m.SSID)
	putStr(out, "encryption", m.Encryption)
	putBool(out, "disabled", m.Disabled)
	putBool(out, "hidden", m.Hidden)
	putBool(out, "isolate", m.Isolate)
	putStr(out, "key", m.Key)
	return out
}

// read refreshes everything except key, which the API never returns and which
// must therefore be preserved from the prior plan/state value.
func (r *wirelessInterfaceResource) read(_ context.Context, obj map[string]any, m *wirelessInterfaceModel) {
	m.ID = strVal(obj, "id")
	m.Managed = boolVal(obj, "managed")
	m.Device = strVal(obj, "device")
	m.Network = strVal(obj, "network")
	m.Mode = strVal(obj, "mode")
	m.SSID = strVal(obj, "ssid")
	m.Encryption = strVal(obj, "encryption")
	m.Disabled = boolVal(obj, "disabled")
	m.Hidden = boolVal(obj, "hidden")
	m.Isolate = boolVal(obj, "isolate")
	hasKey := boolVal(obj, "has_key")
	if hasKey.IsNull() {
		hasKey = types.BoolValue(false)
	}
	m.HasKey = hasKey
}

func (r *wirelessInterfaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan wirelessInterfaceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, err := r.client.Post(ctx, "/"+wirelessInterfaceCollection, r.body(ctx, plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating wireless interface", err.Error())
		return
	}
	r.read(ctx, obj, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *wirelessInterfaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state wirelessInterfaceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, found, err := r.client.GetObject(ctx, "/"+wirelessInterfaceCollection+"/"+state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading wireless interface", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	r.read(ctx, obj, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *wirelessInterfaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan wirelessInterfaceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, err := r.client.Put(ctx, "/"+wirelessInterfaceCollection+"/"+plan.ID.ValueString(), r.body(ctx, plan))
	if err != nil {
		resp.Diagnostics.AddError("Error updating wireless interface", err.Error())
		return
	}
	r.read(ctx, obj, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *wirelessInterfaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state wirelessInterfaceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/"+wirelessInterfaceCollection+"/"+state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting wireless interface", err.Error())
	}
}

func (r *wirelessInterfaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := resolveImportID(ctx, r.client, wirelessInterfaceCollection, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing wireless interface", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
