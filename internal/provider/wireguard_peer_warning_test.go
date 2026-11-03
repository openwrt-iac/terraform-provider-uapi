package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// The warning fires only when uapi READ the kernel and found the peer absent:
// an empty runtime object is "not read" (the interface is down, or wg failed)
// and must stay silent, and a disabled peer is not applied by wireguard.sh, so
// its absence is by design.
func TestWireguardPeerNotInKernelWarning(t *testing.T) {
	cases := []struct {
		name    string
		obj     map[string]any
		want    int
		summary string
	}{
		{"no runtime key", map[string]any{"disabled": false}, 0, ""},
		{"runtime not read (empty object)", map[string]any{"runtime": map[string]any{}, "disabled": false}, 0, ""},
		{"in kernel", map[string]any{"runtime": map[string]any{"in_kernel": true}, "disabled": false}, 0, ""},
		{"in_kernel not a bool", map[string]any{"runtime": map[string]any{"in_kernel": "x"}, "disabled": false}, 0, ""},
		{"absent but disabled", map[string]any{"runtime": map[string]any{"in_kernel": false}, "disabled": true}, 0, ""},
		{"absent and enabled", map[string]any{"runtime": map[string]any{"in_kernel": false}, "disabled": false}, 1, "Kernel holds no peer under this public key"},
		{"absent, disabled unset reads as enabled", map[string]any{"runtime": map[string]any{"in_kernel": false}}, 1, "Kernel holds no peer under this public key"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var d diag.Diagnostics
			warnWireguardPeerNotInKernel(c.obj, &d)
			if d.HasError() {
				t.Fatalf("warnings only, got errors: %v", d.Errors())
			}
			got := d.Warnings()
			if len(got) != c.want {
				t.Fatalf("want %d warnings, got %d: %v", c.want, len(got), got)
			}
			if c.want > 0 && got[0].Summary() != c.summary {
				t.Errorf("summary = %q, want %q", got[0].Summary(), c.summary)
			}
		})
	}
}
