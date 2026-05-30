package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/raspbeguy/terraform-provider-uapi/internal/client"
)

var (
	_ datasource.DataSource              = &dhcpHostDataSource{}
	_ datasource.DataSourceWithConfigure = &dhcpHostDataSource{}
)

type dhcpHostDataSource struct{ client *client.Client }

func NewDHCPHostDataSource() datasource.DataSource { return &dhcpHostDataSource{} }

func (d *dhcpHostDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dhcp_host"
}

func (d *dhcpHostDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *dhcpHostDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Look up a static DHCP lease by id.",
		Attributes: map[string]dsschema.Attribute{
			"id":        dsIDAttribute(),
			"managed":   dsschema.BoolAttribute{Computed: true},
			"name":      dsschema.StringAttribute{Computed: true},
			"mac":       dsschema.StringAttribute{Computed: true},
			"ip":        dsschema.StringAttribute{Computed: true},
			"leasetime": dsschema.StringAttribute{Computed: true},
			"tag":       dsschema.StringAttribute{Computed: true},
			"dns":       dsschema.BoolAttribute{Computed: true},
		},
	}
}

func (d *dhcpHostDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m dhcpHostModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, found, err := d.client.GetObject(ctx, "/"+dhcpHostCollection+"/"+m.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading dhcp host", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("DHCP host not found", "No dhcp host with id "+m.ID.ValueString())
		return
	}
	(&dhcpHostResource{}).read(ctx, obj, &m)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

var (
	_ datasource.DataSource              = &dhcpLeasesDataSource{}
	_ datasource.DataSourceWithConfigure = &dhcpLeasesDataSource{}
)

type dhcpLeasesDataSource struct{ client *client.Client }

func NewDHCPLeasesDataSource() datasource.DataSource { return &dhcpLeasesDataSource{} }

type dhcpLeasesModel struct {
	Leases []dhcpLeaseModel `tfsdk:"leases"`
}

type dhcpLeaseModel struct {
	ExpiresAt types.Int64  `tfsdk:"expires_at"`
	MAC       types.String `tfsdk:"mac"`
	IP        types.String `tfsdk:"ip"`
	Hostname  types.String `tfsdk:"hostname"`
	DUID      types.String `tfsdk:"duid"`
}

func (d *dhcpLeasesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dhcp_leases"
}

func (d *dhcpLeasesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *dhcpLeasesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "The current active DHCP leases (IPv4) reported by the router at runtime.",
		Attributes: map[string]dsschema.Attribute{
			"leases": dsschema.ListNestedAttribute{
				Computed: true,
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: map[string]dsschema.Attribute{
						"expires_at": dsschema.Int64Attribute{Computed: true, Description: "Unix epoch seconds when the lease expires."},
						"mac":        dsschema.StringAttribute{Computed: true},
						"ip":         dsschema.StringAttribute{Computed: true},
						"hostname":   dsschema.StringAttribute{Computed: true},
						"duid":       dsschema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *dhcpLeasesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	list, err := d.client.GetList(ctx, "/dhcp/leases")
	if err != nil {
		resp.Diagnostics.AddError("Error reading dhcp leases", err.Error())
		return
	}
	out := dhcpLeasesModel{Leases: make([]dhcpLeaseModel, 0, len(list))}
	for _, obj := range list {
		out.Leases = append(out.Leases, dhcpLeaseModel{
			ExpiresAt: int64Val(obj, "expires_at"),
			MAC:       strVal(obj, "mac"),
			IP:        strVal(obj, "ip"),
			Hostname:  strVal(obj, "hostname"),
			DUID:      strVal(obj, "duid"),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
