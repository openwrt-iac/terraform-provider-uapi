package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/openwrt-iac/terraform-provider-uapi/internal/client"
)

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
		Description: "Active IPv4 DHCP leases reported by the router at runtime.",
		Attributes: map[string]dsschema.Attribute{
			"leases": dsschema.ListNestedAttribute{
				Computed:    true,
				Description: "Active DHCP leases.",
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: map[string]dsschema.Attribute{
						"expires_at": dsComputedInt64("Unix epoch seconds when the lease expires."),
						"mac":        dsComputedString("Client MAC address."),
						"ip":         dsComputedString("Assigned IP address."),
						"hostname":   dsComputedString("Client hostname, if known."),
						"duid":       dsComputedString("Client DUID, if known."),
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
