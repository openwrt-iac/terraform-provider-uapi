package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/raspbeguy/terraform-provider-uapi/internal/client"
)

var (
	_ datasource.DataSource              = &firewallRuleDataSource{}
	_ datasource.DataSourceWithConfigure = &firewallRuleDataSource{}
)

type firewallRuleDataSource struct{ client *client.Client }

func NewFirewallRuleDataSource() datasource.DataSource { return &firewallRuleDataSource{} }

func (d *firewallRuleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_firewall_rule"
}

func (d *firewallRuleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *firewallRuleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Look up a firewall rule by id.",
		Attributes: map[string]dsschema.Attribute{
			"id":      dsIDAttribute(),
			"managed": dsschema.BoolAttribute{Computed: true},
			"name":    dsschema.StringAttribute{Computed: true},
			"target":  dsschema.StringAttribute{Computed: true},
			"enabled": dsschema.BoolAttribute{Computed: true},
			"match": dsschema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]dsschema.Attribute{
					"src_zone":  dsschema.StringAttribute{Computed: true},
					"dest_zone": dsschema.StringAttribute{Computed: true},
					"src_ip":    dsStringListAttribute(),
					"dest_ip":   dsStringListAttribute(),
					"src_port":  dsStringListAttribute(),
					"dest_port": dsStringListAttribute(),
					"proto":     dsStringListAttribute(),
					"family":    dsschema.StringAttribute{Computed: true},
				},
			},
		},
	}
}

func (d *firewallRuleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m firewallRuleModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, found, err := d.client.GetObject(ctx, "/"+firewallRuleCollection+"/"+m.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading firewall rule", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Firewall rule not found", "No firewall rule with id "+m.ID.ValueString())
		return
	}
	ds := newDiagsink(&resp.Diagnostics)
	(&firewallRuleResource{}).read(ctx, obj, &m, ds)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

var (
	_ datasource.DataSource              = &firewallZoneDataSource{}
	_ datasource.DataSourceWithConfigure = &firewallZoneDataSource{}
)

type firewallZoneDataSource struct{ client *client.Client }

func NewFirewallZoneDataSource() datasource.DataSource { return &firewallZoneDataSource{} }

func (d *firewallZoneDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_firewall_zone"
}

func (d *firewallZoneDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *firewallZoneDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Look up a firewall zone by id.",
		Attributes: map[string]dsschema.Attribute{
			"id":      dsIDAttribute(),
			"managed": dsschema.BoolAttribute{Computed: true},
			"name":    dsschema.StringAttribute{Computed: true},
			"input":   dsschema.StringAttribute{Computed: true},
			"output":  dsschema.StringAttribute{Computed: true},
			"forward": dsschema.StringAttribute{Computed: true},
			"network": dsStringListAttribute(),
			"masq":    dsschema.BoolAttribute{Computed: true},
			"mtu_fix": dsschema.BoolAttribute{Computed: true},
			"family":  dsschema.StringAttribute{Computed: true},
		},
	}
}

func (d *firewallZoneDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m firewallZoneModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, found, err := d.client.GetObject(ctx, "/"+firewallZoneCollection+"/"+m.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading firewall zone", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Firewall zone not found", "No firewall zone with id "+m.ID.ValueString())
		return
	}
	ds := newDiagsink(&resp.Diagnostics)
	(&firewallZoneResource{}).read(ctx, obj, &m, ds)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

var (
	_ datasource.DataSource              = &firewallRedirectDataSource{}
	_ datasource.DataSourceWithConfigure = &firewallRedirectDataSource{}
)

type firewallRedirectDataSource struct{ client *client.Client }

func NewFirewallRedirectDataSource() datasource.DataSource { return &firewallRedirectDataSource{} }

func (d *firewallRedirectDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_firewall_redirect"
}

func (d *firewallRedirectDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *firewallRedirectDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Look up a firewall redirect by id.",
		Attributes: map[string]dsschema.Attribute{
			"id":      dsIDAttribute(),
			"managed": dsschema.BoolAttribute{Computed: true},
			"name":    dsschema.StringAttribute{Computed: true},
			"target":  dsschema.StringAttribute{Computed: true},
			"enabled": dsschema.BoolAttribute{Computed: true},
			"match": dsschema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]dsschema.Attribute{
					"src_zone":  dsschema.StringAttribute{Computed: true},
					"dest_zone": dsschema.StringAttribute{Computed: true},
					"src_ip":    dsStringListAttribute(),
					"src_port":  dsStringListAttribute(),
					"src_dport": dsStringListAttribute(),
					"dest_ip":   dsStringListAttribute(),
					"dest_port": dsStringListAttribute(),
					"proto":     dsStringListAttribute(),
					"family":    dsschema.StringAttribute{Computed: true},
				},
			},
		},
	}
}

func (d *firewallRedirectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m firewallRedirectModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, found, err := d.client.GetObject(ctx, "/"+firewallRedirectCollection+"/"+m.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading firewall redirect", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Firewall redirect not found", "No firewall redirect with id "+m.ID.ValueString())
		return
	}
	ds := newDiagsink(&resp.Diagnostics)
	(&firewallRedirectResource{}).read(ctx, obj, &m, ds)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func dsIDAttribute() dsschema.StringAttribute {
	return dsschema.StringAttribute{Required: true, Description: "Resource id to look up."}
}

func dsStringListAttribute() dsschema.ListAttribute {
	return dsschema.ListAttribute{ElementType: types.StringType, Computed: true}
}
