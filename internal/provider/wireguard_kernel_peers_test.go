package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/openwrt-iac/terraform-provider-uapi/internal/client"
)

// An empty peer list next to an unread interface is not a kernel without
// drift, so the read warns (#380). The acceptance suite cannot see warnings.
func TestWireguardKernelPeersWarnsOnUnread(t *testing.T) {
	for _, tc := range []struct {
		header   string
		warnings int
	}{
		{"", 0},
		{"wg1", 1},
	} {
		ctx := context.Background()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if tc.header != "" {
				w.Header().Set("X-Kernel-Unread", tc.header)
			}
			_, _ = w.Write([]byte(`[]`))
		}))

		d := &wireguardKernelPeersDataSource{client: client.New(srv.URL, "tok", false, "test")}
		schemaResp := &datasource.SchemaResponse{}
		d.Schema(ctx, datasource.SchemaRequest{}, schemaResp)
		typ := schemaResp.Schema.Type().TerraformType(ctx)
		resp := &datasource.ReadResponse{
			State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(typ, nil)},
		}
		d.Read(ctx, datasource.ReadRequest{}, resp)
		srv.Close()

		if resp.Diagnostics.HasError() {
			t.Fatalf("header %q: read failed: %v", tc.header, resp.Diagnostics.Errors())
		}
		warns := resp.Diagnostics.Warnings()
		if len(warns) != tc.warnings {
			t.Fatalf("header %q: %d warnings, want %d: %v", tc.header, len(warns), tc.warnings, warns)
		}
		if tc.warnings > 0 && !strings.Contains(warns[0].Summary(), tc.header) {
			t.Errorf("header %q: the warning does not name the interface: %q", tc.header, warns[0].Summary())
		}
		var m wireguardKernelPeersModel
		resp.Diagnostics.Append(resp.State.Get(ctx, &m)...)
		if m.UnreadInterfaces.IsNull() {
			t.Errorf("header %q: unread_interfaces is null, want a list", tc.header)
		}
	}
}
