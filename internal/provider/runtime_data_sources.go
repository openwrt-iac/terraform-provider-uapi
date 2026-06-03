package provider

import (
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Runtime blocks are live ubus-derived state, exposed only on data sources (not
// resources): they are read-only observed state, never desired config.

type ifaceAddrModel struct {
	Address types.String `tfsdk:"address"`
	Mask    types.Int64  `tfsdk:"mask"`
}

type ifaceRouteModel struct {
	Target  types.String `tfsdk:"target"`
	Mask    types.Int64  `tfsdk:"mask"`
	Nexthop types.String `tfsdk:"nexthop"`
	Source  types.String `tfsdk:"source"`
}

type networkInterfaceRuntimeModel struct {
	Up          types.Bool        `tfsdk:"up"`
	Pending     types.Bool        `tfsdk:"pending"`
	Available   types.Bool        `tfsdk:"available"`
	L3Device    types.String      `tfsdk:"l3_device"`
	Uptime      types.Int64       `tfsdk:"uptime"`
	IPv4Address []ifaceAddrModel  `tfsdk:"ipv4_address"`
	IPv6Address []ifaceAddrModel  `tfsdk:"ipv6_address"`
	IPv6Prefix  []ifaceAddrModel  `tfsdk:"ipv6_prefix"`
	Route       []ifaceRouteModel `tfsdk:"route"`
}

type wirelessInterfaceRuntimeModel struct {
	Ifname         types.String `tfsdk:"ifname"`
	BSSID          types.String `tfsdk:"bssid"`
	Channel        types.Int64  `tfsdk:"channel"`
	Frequency      types.Int64  `tfsdk:"frequency"`
	Signal         types.Int64  `tfsdk:"signal"`
	Noise          types.Int64  `tfsdk:"noise"`
	TxpowerActual  types.Int64  `tfsdk:"txpower_actual"`
	AssoclistCount types.Int64  `tfsdk:"assoclist_count"`
}

func addrList(raw any) []ifaceAddrModel {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]ifaceAddrModel, 0, len(arr))
	for _, e := range arr {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, ifaceAddrModel{Address: strVal(m, "address"), Mask: int64Val(m, "mask")})
	}
	return out
}

func parseNetworkInterfaceRuntime(obj map[string]any) *networkInterfaceRuntimeModel {
	rt, ok := obj["runtime"].(map[string]any)
	if !ok {
		rt = map[string]any{}
	}
	rm := &networkInterfaceRuntimeModel{
		Up:          boolVal(rt, "up"),
		Pending:     boolVal(rt, "pending"),
		Available:   boolVal(rt, "available"),
		L3Device:    strVal(rt, "l3_device"),
		Uptime:      int64Val(rt, "uptime"),
		IPv4Address: addrList(rt["ipv4_address"]),
		IPv6Address: addrList(rt["ipv6_address"]),
		IPv6Prefix:  addrList(rt["ipv6_prefix"]),
	}
	if arr, ok := rt["route"].([]any); ok {
		for _, e := range arr {
			m, ok := e.(map[string]any)
			if !ok {
				continue
			}
			rm.Route = append(rm.Route, ifaceRouteModel{
				Target:  strVal(m, "target"),
				Mask:    int64Val(m, "mask"),
				Nexthop: strVal(m, "nexthop"),
				Source:  strVal(m, "source"),
			})
		}
	}
	return rm
}

func parseWirelessInterfaceRuntime(obj map[string]any) *wirelessInterfaceRuntimeModel {
	rt, ok := obj["runtime"].(map[string]any)
	if !ok {
		rt = map[string]any{}
	}
	return &wirelessInterfaceRuntimeModel{
		Ifname:         strVal(rt, "ifname"),
		BSSID:          strVal(rt, "bssid"),
		Channel:        int64Val(rt, "channel"),
		Frequency:      int64Val(rt, "frequency"),
		Signal:         int64Val(rt, "signal"),
		Noise:          int64Val(rt, "noise"),
		TxpowerActual:  int64Val(rt, "txpower_actual"),
		AssoclistCount: int64Val(rt, "assoclist_count"),
	}
}

func networkInterfaceRuntimeAttribute() dsschema.SingleNestedAttribute {
	addr := dsschema.NestedAttributeObject{Attributes: map[string]dsschema.Attribute{
		"address": dsschema.StringAttribute{Computed: true, Description: "Address."},
		"mask":    dsschema.Int64Attribute{Computed: true, Description: "Prefix length."},
	}}
	return dsschema.SingleNestedAttribute{
		Computed:    true,
		Description: "Live ubus-derived runtime state (read-only; reflects actual operation, not config).",
		Attributes: map[string]dsschema.Attribute{
			"up":           dsschema.BoolAttribute{Computed: true, Description: "Whether the interface is up."},
			"pending":      dsschema.BoolAttribute{Computed: true, Description: "Whether the interface is mid-setup."},
			"available":    dsschema.BoolAttribute{Computed: true, Description: "Whether the interface is available."},
			"l3_device":    dsschema.StringAttribute{Computed: true, Description: "Actual L3 kernel device."},
			"uptime":       dsschema.Int64Attribute{Computed: true, Description: "Seconds since the interface came up."},
			"ipv4_address": dsschema.ListNestedAttribute{Computed: true, Description: "Assigned IPv4 addresses.", NestedObject: addr},
			"ipv6_address": dsschema.ListNestedAttribute{Computed: true, Description: "Assigned IPv6 addresses.", NestedObject: addr},
			"ipv6_prefix":  dsschema.ListNestedAttribute{Computed: true, Description: "Delegated IPv6 prefixes.", NestedObject: addr},
			"route": dsschema.ListNestedAttribute{Computed: true, Description: "Active routes.", NestedObject: dsschema.NestedAttributeObject{Attributes: map[string]dsschema.Attribute{
				"target":  dsschema.StringAttribute{Computed: true, Description: "Destination network."},
				"mask":    dsschema.Int64Attribute{Computed: true, Description: "Destination prefix length."},
				"nexthop": dsschema.StringAttribute{Computed: true, Description: "Next-hop gateway."},
				"source":  dsschema.StringAttribute{Computed: true, Description: "Preferred source address."},
			}}},
		},
	}
}

// The interface/wireless data-source structs themselves are generated (they
// embed the resource fields, whose types track the spec); this file keeps only
// the stable runtime sub-models, parsers, and the nested-attribute schema.

func wirelessInterfaceRuntimeAttribute() dsschema.SingleNestedAttribute {
	return dsschema.SingleNestedAttribute{
		Computed:    true,
		Description: "Live iwinfo-derived runtime state (read-only).",
		Attributes: map[string]dsschema.Attribute{
			"ifname":          dsschema.StringAttribute{Computed: true, Description: "Kernel wireless device name."},
			"bssid":           dsschema.StringAttribute{Computed: true, Description: "BSSID."},
			"channel":         dsschema.Int64Attribute{Computed: true, Description: "Operating channel."},
			"frequency":       dsschema.Int64Attribute{Computed: true, Description: "Operating frequency (MHz)."},
			"signal":          dsschema.Int64Attribute{Computed: true, Description: "Signal level (dBm)."},
			"noise":           dsschema.Int64Attribute{Computed: true, Description: "Noise floor (dBm)."},
			"txpower_actual":  dsschema.Int64Attribute{Computed: true, Description: "Actual transmit power (dBm)."},
			"assoclist_count": dsschema.Int64Attribute{Computed: true, Description: "Number of associated clients."},
		},
	}
}
