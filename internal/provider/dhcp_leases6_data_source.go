package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/openwrt-iac/terraform-provider-uapi/internal/client"
)

var (
	_ datasource.DataSource              = &dhcpLeases6DataSource{}
	_ datasource.DataSourceWithConfigure = &dhcpLeases6DataSource{}
)

type dhcpLeases6DataSource struct{ client *client.Client }

func NewDHCPLeases6DataSource() datasource.DataSource { return &dhcpLeases6DataSource{} }

type dhcpLeases6Model struct {
	Leases []dhcpLease6Model `tfsdk:"leases"`
}

type dhcpLease6Model struct {
	DUID         types.String `tfsdk:"duid"`
	IAID         types.String `tfsdk:"iaid"`
	Hostname     types.String `tfsdk:"hostname"`
	Interface    types.String `tfsdk:"interface"`
	IAType       types.String `tfsdk:"ia_type"`
	IP           types.String `tfsdk:"ip"`
	PrefixLength types.Int64  `tfsdk:"prefix_length"`
	ExpiresAt    types.Int64  `tfsdk:"expires_at"`
}

func (d *dhcpLeases6DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dhcp_leases6"
}

func (d *dhcpLeases6DataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *dhcpLeases6DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "The current active DHCPv6 leases reported by the router at runtime.",
		Attributes: map[string]dsschema.Attribute{
			"leases": dsschema.ListNestedAttribute{
				Computed:    true,
				Description: "Active DHCPv6 leases.",
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: map[string]dsschema.Attribute{
						"duid":          dsComputedString("Client DUID (hex)."),
						"iaid":          dsComputedString("Identity Association ID (hex)."),
						"hostname":      dsComputedString("Client hostname, if known."),
						"interface":     dsComputedString("Server-side interface that issued the lease."),
						"ia_type":       dsComputedString("Identity Association type (IA_NA, IA_PD)."),
						"ip":            dsComputedString("Assigned IPv6 address or prefix."),
						"prefix_length": dsschema.Int64Attribute{Computed: true, Description: "Prefix length for IA_PD; null for IA_NA."},
						"expires_at":    dsschema.Int64Attribute{Computed: true, Description: "Unix epoch seconds when the lease expires; null for non-numeric values like 'forever'."},
					},
				},
			},
		},
	}
}

func (d *dhcpLeases6DataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	list, err := d.client.GetList(ctx, "/dhcp/leases6")
	if err != nil {
		resp.Diagnostics.AddError("Error reading dhcp leases6", err.Error())
		return
	}
	out := dhcpLeases6Model{Leases: make([]dhcpLease6Model, 0, len(list))}
	for _, obj := range list {
		out.Leases = append(out.Leases, dhcpLease6Model{
			DUID:         strVal(obj, "duid"),
			IAID:         strVal(obj, "iaid"),
			Hostname:     strVal(obj, "hostname"),
			Interface:    strVal(obj, "interface"),
			IAType:       strVal(obj, "ia_type"),
			IP:           strVal(obj, "ip"),
			PrefixLength: int64Val(obj, "prefix_length"),
			ExpiresAt:    int64Val(obj, "expires_at"),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
