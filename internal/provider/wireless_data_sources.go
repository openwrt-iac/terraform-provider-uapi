package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	"github.com/raspbeguy/terraform-provider-uapi/internal/client"
)

var (
	_ datasource.DataSource              = &wirelessDeviceDataSource{}
	_ datasource.DataSourceWithConfigure = &wirelessDeviceDataSource{}
)

type wirelessDeviceDataSource struct{ client *client.Client }

func NewWirelessDeviceDataSource() datasource.DataSource { return &wirelessDeviceDataSource{} }

func (d *wirelessDeviceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_wireless_device"
}

func (d *wirelessDeviceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *wirelessDeviceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Look up a wireless radio by id.",
		Attributes: map[string]dsschema.Attribute{
			"id":       dsIDAttribute(),
			"managed":  dsschema.BoolAttribute{Computed: true},
			"type":     dsschema.StringAttribute{Computed: true},
			"band":     dsschema.StringAttribute{Computed: true},
			"channel":  dsschema.StringAttribute{Computed: true},
			"htmode":   dsschema.StringAttribute{Computed: true},
			"country":  dsschema.StringAttribute{Computed: true},
			"txpower":  dsschema.StringAttribute{Computed: true},
			"disabled": dsschema.BoolAttribute{Computed: true},
		},
	}
}

func (d *wirelessDeviceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m wirelessDeviceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, found, err := d.client.GetObject(ctx, "/"+wirelessDeviceCollection+"/"+m.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading wireless device", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Wireless device not found", "No wireless device with id "+m.ID.ValueString())
		return
	}
	(&wirelessDeviceResource{}).read(ctx, obj, &m)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

var (
	_ datasource.DataSource              = &wirelessInterfaceDataSource{}
	_ datasource.DataSourceWithConfigure = &wirelessInterfaceDataSource{}
)

type wirelessInterfaceDataSource struct{ client *client.Client }

func NewWirelessInterfaceDataSource() datasource.DataSource { return &wirelessInterfaceDataSource{} }

func (d *wirelessInterfaceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_wireless_interface"
}

func (d *wirelessInterfaceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *wirelessInterfaceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Look up a wireless interface (SSID) by id. The encryption key is never returned.",
		Attributes: map[string]dsschema.Attribute{
			"id":         dsIDAttribute(),
			"managed":    dsschema.BoolAttribute{Computed: true},
			"device":     dsschema.StringAttribute{Computed: true},
			"network":    dsschema.StringAttribute{Computed: true},
			"mode":       dsschema.StringAttribute{Computed: true},
			"ssid":       dsschema.StringAttribute{Computed: true},
			"encryption": dsschema.StringAttribute{Computed: true},
			"disabled":   dsschema.BoolAttribute{Computed: true},
			"hidden":     dsschema.BoolAttribute{Computed: true},
			"isolate":    dsschema.BoolAttribute{Computed: true},
			"key":        dsschema.StringAttribute{Computed: true, Sensitive: true, Description: "Always null; the API never returns the key."},
			"has_key":    dsschema.BoolAttribute{Computed: true},
		},
	}
}

func (d *wirelessInterfaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m wirelessInterfaceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, found, err := d.client.GetObject(ctx, "/"+wirelessInterfaceCollection+"/"+m.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading wireless interface", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Wireless interface not found", "No wireless interface with id "+m.ID.ValueString())
		return
	}
	(&wirelessInterfaceResource{}).read(ctx, obj, &m)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
