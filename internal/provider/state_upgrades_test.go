package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// runUpgrade drives a resource's 0 -> 1 upgrader over raw prior-state JSON and
// returns the upgraded state for inspection.
func runUpgrade(t *testing.T, r resource.ResourceWithUpgradeState, priorJSON string) tfsdk.State {
	t.Helper()
	ctx := context.Background()

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	if schemaResp.Schema.Version != 1 {
		t.Fatalf("schema version = %d, want 1: an upgrader without a version bump never runs",
			schemaResp.Schema.Version)
	}

	upgraders := r.UpgradeState(ctx)
	u, ok := upgraders[0]
	if !ok {
		t.Fatal("no 0 -> 1 state upgrader registered")
	}

	req := resource.UpgradeStateRequest{RawState: &tfprotov6.RawState{JSON: []byte(priorJSON)}}
	resp := &resource.UpgradeStateResponse{
		State: tfsdk.State{
			Schema: schemaResp.Schema,
			Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil),
		},
	}
	u.StateUpgrader(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("upgrade failed: %v", resp.Diagnostics.Errors())
	}
	return resp.State
}

func attrString(t *testing.T, s tfsdk.State, steps ...any) string {
	t.Helper()
	var v string
	p := tftypes.NewAttributePath()
	for _, step := range steps {
		p = p.WithAttributeName(step.(string))
	}
	raw, _, err := tftypes.WalkAttributePath(s.Raw, p)
	if err != nil {
		t.Fatalf("walking %v: %v", steps, err)
	}
	val, ok := raw.(tftypes.Value)
	if !ok {
		t.Fatalf("%v is not a value", steps)
	}
	if val.IsNull() {
		return "<null>"
	}
	if err := val.As(&v); err != nil {
		t.Fatalf("reading %v: %v", steps, err)
	}
	return v
}

// State written by provider 2.x: every redirect match selector is a list. This
// is the shape that made 3.0.0 abort at plan time.
func TestUpgradeFirewallRedirect_from2xLists(t *testing.T) {
	r := &firewallRedirectResource{}
	s := runUpgrade(t, r, `{
		"id":"r1","managed":true,"etag":"\"v1\"","target":"DNAT","enabled":true,
		"match":{"src_zone":"wan","proto":["tcp"],"src_dport":["8443"],
		         "dest_ip":["192.168.1.50"],"dest_port":["443"],"src_ip":[],"src_dip":null}
	}`)
	for _, tc := range []struct{ field, want string }{
		{"src_dport", "8443"},
		{"dest_ip", "192.168.1.50"},
		{"dest_port", "443"},
		{"src_ip", "<null>"},  // empty list means no value
		{"src_dip", "<null>"}, // explicit null stays null
	} {
		if got := attrString(t, s, "match", tc.field); got != tc.want {
			t.Errorf("match.%s = %q, want %q", tc.field, got, tc.want)
		}
	}
	// etag is not set by read(); losing it would silently drop optimistic concurrency.
	if got := attrString(t, s, "etag"); got != `"v1"` {
		t.Errorf("etag = %q, want %q", got, `"v1"`)
	}
	if got := attrString(t, s, "id"); got != "r1" {
		t.Errorf("id = %q, want r1", got)
	}
}

// State written by 3.0.0 itself is already scalar but still stamped version 0,
// so the same upgrader runs over it. It must be a no-op, not a failure.
func TestUpgradeFirewallRedirect_from300ScalarsIsNoop(t *testing.T) {
	r := &firewallRedirectResource{}
	s := runUpgrade(t, r, `{
		"id":"r1","managed":true,"etag":"\"v2\"","target":"DNAT",
		"match":{"src_zone":"wan","proto":["tcp"],"src_dport":"8443","dest_port":"443"}
	}`)
	if got := attrString(t, s, "match", "src_dport"); got != "8443" {
		t.Errorf("src_dport = %q, want 8443", got)
	}
	if got := attrString(t, s, "match", "dest_port"); got != "443" {
		t.Errorf("dest_port = %q, want 443", got)
	}
}

// dhcp_host.tag went the other way, and a release earlier: string through 2.4.x,
// list from 2.5.0, neither with a version bump.
func TestUpgradeDhcpHost_tagStringToList(t *testing.T) {
	r := &dhcpHostResource{}
	s := runUpgrade(t, r, `{"id":"h1","managed":true,"etag":"\"v1\"","ip":"192.168.1.5",
		"macs":["02:00:00:00:00:01"],"tag":"laptop"}`)
	p := tftypes.NewAttributePath().WithAttributeName("tag")
	raw, _, err := tftypes.WalkAttributePath(s.Raw, p)
	if err != nil {
		t.Fatalf("walking tag: %v", err)
	}
	var list []tftypes.Value
	if err := raw.(tftypes.Value).As(&list); err != nil {
		t.Fatalf("tag is not a list after upgrade: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("tag = %v, want one element", list)
	}
	var got string
	if err := list[0].As(&got); err != nil || got != "laptop" {
		t.Errorf("tag[0] = %q (err %v), want laptop", got, err)
	}
}

// A 2.5.0+ host already stores a list; the upgrader must leave it alone.
func TestUpgradeDhcpHost_tagListIsNoop(t *testing.T) {
	r := &dhcpHostResource{}
	s := runUpgrade(t, r, `{"id":"h1","managed":true,"etag":"\"v1\"","ip":"192.168.1.5",
		"macs":["02:00:00:00:00:01"],"tag":["laptop","wired"]}`)
	p := tftypes.NewAttributePath().WithAttributeName("tag")
	raw, _, _ := tftypes.WalkAttributePath(s.Raw, p)
	var list []tftypes.Value
	if err := raw.(tftypes.Value).As(&list); err != nil {
		t.Fatalf("tag is not a list: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("tag = %v, want both elements preserved", list)
	}
}

// uci stores multiple tags as one space-separated option and uapi answers with the
// split list, so a 2.4.x state can hold "lan guest" in a single string.
func TestUpgradeDhcpHost_tagSpaceSeparated(t *testing.T) {
	s := runUpgrade(t, &dhcpHostResource{}, `{"id":"h1","managed":true,"etag":"\"v1\"",
		"ip":"192.168.1.5","macs":["02:00:00:00:00:01"],"tag":"lan guest"}`)
	p := tftypes.NewAttributePath().WithAttributeName("tag")
	raw, _, _ := tftypes.WalkAttributePath(s.Raw, p)
	var list []tftypes.Value
	if err := raw.(tftypes.Value).As(&list); err != nil {
		t.Fatalf("tag is not a list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("tag = %v, want two tags split out of the scalar", list)
	}
	for i, want := range []string{"lan", "guest"} {
		var got string
		if err := list[i].As(&got); err != nil || got != want {
			t.Errorf("tag[%d] = %q, want %q", i, got, want)
		}
	}
}

// Truncating a multi-value selector is correct but must not be silent.
func TestUpgradeFirewallRedirect_warnsOnTruncation(t *testing.T) {
	r := &firewallRedirectResource{}
	ctx := context.Background()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	resp := &resource.UpgradeStateResponse{State: tfsdk.State{
		Schema: schemaResp.Schema,
		Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil),
	}}
	r.UpgradeState(ctx)[0].StateUpgrader(ctx, resource.UpgradeStateRequest{
		RawState: &tfprotov6.RawState{JSON: []byte(`{"id":"r1","managed":true,"etag":"\"v1\"","target":"DNAT",
			"match":{"src_zone":"wan","dest_ip":["192.168.1.50","192.168.1.51"]}}`)},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("upgrade failed: %v", resp.Diagnostics.Errors())
	}
	if resp.Diagnostics.WarningsCount() != 1 {
		t.Fatalf("expected one truncation warning, got %d", resp.Diagnostics.WarningsCount())
	}
	if got := resp.Diagnostics.Warnings()[0].Detail(); !strings.Contains(got, "match.dest_ip held 2 values") {
		t.Errorf("warning should name the field and count, got %q", got)
	}
}
