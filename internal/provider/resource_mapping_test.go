package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func strList(t *testing.T, vals ...string) types.List {
	t.Helper()
	elems := make([]attr.Value, len(vals))
	for i, v := range vals {
		elems[i] = types.StringValue(v)
	}
	l, d := types.ListValue(types.StringType, elems)
	if d.HasError() {
		t.Fatalf("building list: %v", d)
	}
	return l
}

func TestFirewallRuleBody(t *testing.T) {
	ctx := context.Background()
	m := firewallRuleModel{
		Name:    types.StringNull(),
		Target:  types.StringValue("ACCEPT"),
		Enabled: types.BoolValue(true),
		Match: &firewallRuleMatch{
			SrcZone:  types.StringValue("wan"),
			DestZone: types.StringNull(),
			SrcIP:    types.ListNull(types.StringType),
			DestIP:   strList(t, "10.0.0.0/8"),
			SrcPort:  types.ListUnknown(types.StringType),
			DestPort: strList(t, "22"),
			Proto:    strList(t, "tcp"),
			Family:   types.StringUnknown(),
		},
	}
	var diags diag.Diagnostics
	body := (&firewallRuleResource{}).body(ctx, m, newDiagsink(&diags))
	if diags.HasError() {
		t.Fatalf("diags: %v", diags)
	}

	if _, ok := body["name"]; ok {
		t.Error("null name should be omitted")
	}
	if body["target"] != "ACCEPT" || body["enabled"] != true {
		t.Errorf("top-level fields wrong: %#v", body)
	}
	match, ok := body["match"].(map[string]any)
	if !ok {
		t.Fatalf("match missing or wrong type: %#v", body["match"])
	}
	if match["src_zone"] != "wan" {
		t.Errorf("src_zone = %v", match["src_zone"])
	}
	if _, ok := match["dest_zone"]; ok {
		t.Error("null dest_zone should be omitted")
	}
	if _, ok := match["src_ip"]; ok {
		t.Error("null src_ip should be omitted")
	}
	if _, ok := match["src_port"]; ok {
		t.Error("unknown src_port should be omitted")
	}
	if _, ok := match["family"]; ok {
		t.Error("unknown family should be omitted")
	}
	if dp, _ := match["dest_port"].([]string); len(dp) != 1 || dp[0] != "22" {
		t.Errorf("dest_port = %v", match["dest_port"])
	}
}

func TestFirewallRuleReadIgnoresUnknownFields(t *testing.T) {
	ctx := context.Background()
	obj := map[string]any{
		"id":           "r_1",
		"managed":      true,
		"target":       "ACCEPT",
		"enabled":      true,
		"future_field": "from a newer uapi, must be ignored",
		"match": map[string]any{
			"src_zone":  "wan",
			"proto":     []any{"tcp"},
			"dest_port": []any{"22"},
			"family":    "any",
		},
	}
	var m firewallRuleModel
	var diags diag.Diagnostics
	(&firewallRuleResource{}).read(ctx, obj, &m, newDiagsink(&diags))
	if diags.HasError() {
		t.Fatalf("diags: %v", diags)
	}

	if m.ID.ValueString() != "r_1" || !m.Managed.ValueBool() {
		t.Errorf("id/managed wrong: %+v", m)
	}
	if m.Match == nil || m.Match.SrcZone.ValueString() != "wan" {
		t.Fatalf("match.src_zone wrong: %+v", m.Match)
	}
	if m.Match.Family.ValueString() != "any" {
		t.Errorf("family = %v", m.Match.Family)
	}
	if !m.Match.DestZone.IsNull() {
		t.Errorf("dest_zone should be null, got %v", m.Match.DestZone)
	}
	// Missing list comes back as an empty, non-null list.
	if m.Match.SrcIP.IsNull() || len(m.Match.SrcIP.Elements()) != 0 {
		t.Errorf("src_ip should be empty list, got %v", m.Match.SrcIP)
	}
}

// The wireless key is write-only: read must never overwrite the planned value
// (the API never returns it), but must still surface has_key.
func TestWirelessInterfaceReadPreservesKey(t *testing.T) {
	ctx := context.Background()
	m := wirelessInterfaceModel{Key: types.StringValue("super-secret")}
	obj := map[string]any{
		"id":      "w_1",
		"managed": true,
		"ssid":    "home",
		"has_key": true,
	}
	(&wirelessInterfaceResource{}).read(ctx, obj, &m, newDiagsink(&diag.Diagnostics{}))

	if m.Key.ValueString() != "super-secret" {
		t.Errorf("key must be preserved, got %v", m.Key)
	}
	if !m.HasKey.ValueBool() {
		t.Errorf("has_key should be true, got %v", m.HasKey)
	}
	if m.Ssid.ValueString() != "home" {
		t.Errorf("ssid = %v", m.Ssid)
	}
}

func TestWirelessInterfaceReadDefaultsHasKey(t *testing.T) {
	ctx := context.Background()
	var m wirelessInterfaceModel
	(&wirelessInterfaceResource{}).read(ctx, map[string]any{"id": "w_1"}, &m, newDiagsink(&diag.Diagnostics{}))
	if m.HasKey.IsNull() || m.HasKey.ValueBool() {
		t.Errorf("has_key should default to false, got %v", m.HasKey)
	}
}
