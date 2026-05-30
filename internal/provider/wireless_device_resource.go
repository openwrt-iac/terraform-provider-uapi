package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/raspbeguy/terraform-provider-uapi/internal/client"
)

const wirelessDeviceCollection = "wireless/devices"

var (
	_ resource.Resource                = &wirelessDeviceResource{}
	_ resource.ResourceWithConfigure   = &wirelessDeviceResource{}
	_ resource.ResourceWithImportState = &wirelessDeviceResource{}
)

type wirelessDeviceResource struct {
	client *client.Client
}

func NewWirelessDeviceResource() resource.Resource {
	return &wirelessDeviceResource{}
}

type wirelessDeviceModel struct {
	ID       types.String `tfsdk:"id"`
	Managed  types.Bool   `tfsdk:"managed"`
	Type     types.String `tfsdk:"type"`
	Band     types.String `tfsdk:"band"`
	Channel  types.String `tfsdk:"channel"`
	HTMode   types.String `tfsdk:"htmode"`
	Country  types.String `tfsdk:"country"`
	TxPower  types.String `tfsdk:"txpower"`
	Disabled types.Bool   `tfsdk:"disabled"`
}

func (r *wirelessDeviceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_wireless_device"
}

func (r *wirelessDeviceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *wirelessDeviceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A wireless radio (uci wireless.wifi-device).",
		Attributes: map[string]schema.Attribute{
			"id":      computedIDAttribute(),
			"managed": managedAttribute(),
			"type": schema.StringAttribute{
				Required:    true,
				Description: "Driver type: mac80211 or broadcom.",
			},
			"band":     optionalComputedString("Frequency band: 2g, 5g, 6g, or 60g."),
			"channel":  optionalComputedString("Channel number or 'auto'."),
			"htmode":   optionalComputedString("HT/VHT/HE mode (e.g. HT20, VHT80)."),
			"country":  optionalComputedString("Regulatory country code."),
			"txpower":  optionalComputedString("Transmit power in dBm."),
			"disabled": optionalComputedBool("Disable the radio. Defaults to false."),
		},
	}
}

func (r *wirelessDeviceResource) body(_ context.Context, m wirelessDeviceModel) map[string]any {
	out := map[string]any{}
	putStr(out, "type", m.Type)
	putStr(out, "band", m.Band)
	putStr(out, "channel", m.Channel)
	putStr(out, "htmode", m.HTMode)
	putStr(out, "country", m.Country)
	putStr(out, "txpower", m.TxPower)
	putBool(out, "disabled", m.Disabled)
	return out
}

func (r *wirelessDeviceResource) read(_ context.Context, obj map[string]any, m *wirelessDeviceModel) {
	m.ID = strVal(obj, "id")
	m.Managed = boolVal(obj, "managed")
	m.Type = strVal(obj, "type")
	m.Band = strVal(obj, "band")
	m.Channel = strVal(obj, "channel")
	m.HTMode = strVal(obj, "htmode")
	m.Country = strVal(obj, "country")
	m.TxPower = strVal(obj, "txpower")
	m.Disabled = boolVal(obj, "disabled")
}

func (r *wirelessDeviceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan wirelessDeviceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, err := r.client.Post(ctx, "/"+wirelessDeviceCollection, r.body(ctx, plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating wireless device", err.Error())
		return
	}
	r.read(ctx, obj, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *wirelessDeviceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state wirelessDeviceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, found, err := r.client.GetObject(ctx, "/"+wirelessDeviceCollection+"/"+state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading wireless device", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	r.read(ctx, obj, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *wirelessDeviceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan wirelessDeviceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, err := r.client.Put(ctx, "/"+wirelessDeviceCollection+"/"+plan.ID.ValueString(), r.body(ctx, plan))
	if err != nil {
		resp.Diagnostics.AddError("Error updating wireless device", err.Error())
		return
	}
	r.read(ctx, obj, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *wirelessDeviceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state wirelessDeviceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/"+wirelessDeviceCollection+"/"+state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting wireless device", err.Error())
	}
}

func (r *wirelessDeviceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := resolveImportID(ctx, r.client, wirelessDeviceCollection, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing wireless device", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
