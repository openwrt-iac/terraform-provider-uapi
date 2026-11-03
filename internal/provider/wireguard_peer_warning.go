package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// warnWireguardPeerNotInKernel warns when a refresh read the kernel and found
// no peer under the stored public key. Only that state warns: uapi answers an
// empty runtime object when it did not read the kernel at all (the interface is
// not up, or wg failed), which is not "absent"; and wireguard.sh does not apply
// a disabled peer, so its absence from the kernel is by design (#53).
func warnWireguardPeerNotInKernel(obj map[string]any, diags *diag.Diagnostics) {
	rt, ok := obj["runtime"].(map[string]any)
	if !ok || len(rt) == 0 {
		return
	}
	ik, ok := rt["in_kernel"].(bool)
	if !ok || ik {
		return
	}
	if disabled := boolVal(obj, "disabled"); !disabled.IsNull() && disabled.ValueBool() {
		return
	}
	diags.AddWarning("Kernel holds no peer under this public key",
		"uapi read the kernel (the interface is up) and found no peer for this section's public key. "+
			"A manual wg command removed it, or an apply did not land, so the peer passes no traffic. "+
			"A write to the peer or its interface restores it; a no-change apply writes nothing and does not. "+
			"Check wg show on the router to see what the kernel holds.")
}
