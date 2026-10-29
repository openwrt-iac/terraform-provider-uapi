package provider

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFieldScopePatches(t *testing.T) {
	s := fieldScope{Field: "proto", Known: []string{"static", "dhcp"}}
	cases := []struct {
		name           string
		planned, prior types.String
		want           bool
	}{
		{"unmodelled and unchanged", types.StringValue("6in4"), types.StringValue("6in4"), true},
		{"modelled", types.StringValue("static"), types.StringValue("static"), false},
		{"switch to unmodelled", types.StringValue("6in4"), types.StringValue("static"), false},
		{"switch away from unmodelled", types.StringValue("static"), types.StringValue("6in4"), false},
		{"unknown", types.StringUnknown(), types.StringValue("6in4"), false},
		{"null", types.StringNull(), types.StringNull(), false},
	}
	for _, c := range cases {
		if got := s.patches(c.planned, c.prior); got != c.want {
			t.Errorf("%s: patches = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestMergeDelta(t *testing.T) {
	prior := map[string]any{"proto": "6in4", "mtu": int64(1280), "ip6prefix": []string{"2001:db8::/48"}, "dns": []string{"192.0.2.53"}}
	planned := map[string]any{"proto": "6in4", "mtu": int64(1400), "ip6prefix": []string{"2001:db8::/48"}}
	want := map[string]any{"mtu": int64(1400), "dns": nil}
	if got := mergeDelta(planned, prior); !reflect.DeepEqual(got, want) {
		t.Errorf("mergeDelta = %v, want %v", got, want)
	}
}
