package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/openwrt-iac/terraform-provider-uapi/internal/client"
)

var (
	_ datasource.DataSource              = &wireguardKernelPeersDataSource{}
	_ datasource.DataSourceWithConfigure = &wireguardKernelPeersDataSource{}
)

type wireguardKernelPeersDataSource struct{ client *client.Client }

func NewWireguardKernelPeersDataSource() datasource.DataSource {
	return &wireguardKernelPeersDataSource{}
}

type wireguardKernelPeersModel struct {
	Peers            []wireguardKernelPeerModel `tfsdk:"peers"`
	UnreadInterfaces types.List                 `tfsdk:"unread_interfaces"`
}

type wireguardKernelPeerModel struct {
	ID                  types.String `tfsdk:"id"`
	Interface           types.String `tfsdk:"interface"`
	PublicKey           types.String `tfsdk:"public_key"`
	Endpoint            types.String `tfsdk:"endpoint"`
	AllowedIPs          types.List   `tfsdk:"allowed_ips"`
	PersistentKeepalive types.Int64  `tfsdk:"persistent_keepalive"`
	LatestHandshake     types.Int64  `tfsdk:"latest_handshake"`
	RxBytes             types.Int64  `tfsdk:"rx_bytes"`
	TxBytes             types.Int64  `tfsdk:"tx_bytes"`
}

func (d *wireguardKernelPeersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_wireguard_kernel_peers"
}

func (d *wireguardKernelPeersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *wireguardKernelPeersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "WireGuard peers the kernel holds that no uci section claims, so a client lists the drift " +
			"without walking every peer. Read-only; uapi reads it with one `wg show <interface> dump` per interface.",
		Attributes: map[string]dsschema.Attribute{
			"peers": dsschema.ListNestedAttribute{
				Computed:    true,
				Description: "Unclaimed kernel peers.",
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: map[string]dsschema.Attribute{
						"id":                   dsComputedString("Stable id of the peer on its interface: the first 12 hex characters of the sha256 of the interface name and the public key."),
						"interface":            dsComputedString("The interface that holds the peer, up and with proto=wireguard."),
						"public_key":           dsComputedString("The public key the kernel holds the peer under, as wg prints it."),
						"endpoint":             dsComputedString("Endpoint of the peer in the kernel, as an address and port."),
						"allowed_ips":          dsschema.ListAttribute{ElementType: types.StringType, Computed: true, Description: "Allowed IPs of the peer in the kernel, in its canonical form."},
						"persistent_keepalive": dsComputedInt64("Keepalive interval of the peer in the kernel, in seconds; 0 when it is off."),
						"latest_handshake":     dsComputedInt64("Time of the latest handshake with the peer, in seconds since the epoch; 0 when there was none."),
						"rx_bytes":             dsComputedInt64("Bytes the kernel received from the peer since it added the peer."),
						"tx_bytes":             dsComputedInt64("Bytes the kernel sent to the peer since it added the peer."),
					},
				},
			},
			"unread_interfaces": dsComputedStringList("Up interfaces with proto=wireguard whose kernel peers uapi could not read, " +
				"because `wg show <interface> dump` failed or printed a shape that uapi does not read. `peers` holds no " +
				"entry for them, so only an empty list says that `peers` covers every up interface. A read warns while " +
				"the list is not empty. Needs uapi 3.5.0 or later, which reports them in the `X-Kernel-Unread` header: " +
				"an older server never does, and the list then reads empty without that guarantee."),
		},
	}
}

func (d *wireguardKernelPeersDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	list, unread, err := d.client.GetListHeader(ctx, "/network/wireguard_kernel_peers", "X-Kernel-Unread")
	if err != nil {
		resp.Diagnostics.AddError("Error reading wireguard kernel peers", err.Error())
		return
	}
	if len(unread) > 0 {
		resp.Diagnostics.AddWarning("uapi could not read the kernel peers of "+strings.Join(unread, ", "),
			"`wg show <interface> dump` failed or printed a shape that uapi does not read, so `peers` lists no "+
				"unclaimed peer of these interfaces, whatever their kernel holds. Run wg show on the router to see it.")
	}
	unreadList, ud := types.ListValueFrom(ctx, types.StringType, append([]string{}, unread...))
	resp.Diagnostics.Append(ud...)
	out := wireguardKernelPeersModel{
		Peers:            make([]wireguardKernelPeerModel, 0, len(list)),
		UnreadInterfaces: unreadList,
	}
	for _, obj := range list {
		out.Peers = append(out.Peers, wireguardKernelPeerModel{
			ID:                  strVal(obj, "id"),
			Interface:           strVal(obj, "interface"),
			PublicKey:           strVal(obj, "public_key"),
			Endpoint:            strVal(obj, "endpoint"),
			AllowedIPs:          kernelStringList(obj["allowed_ips"]),
			PersistentKeepalive: int64Val(obj, "persistent_keepalive"),
			LatestHandshake:     int64Val(obj, "latest_handshake"),
			RxBytes:             int64Val(obj, "rx_bytes"),
			TxBytes:             int64Val(obj, "tx_bytes"),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
