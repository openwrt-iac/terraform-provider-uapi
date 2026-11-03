package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/openwrt-iac/terraform-provider-uapi/internal/client"
)

// A server older than uapi 3.3.0 answers no state_dirs, and the read must
// answer a null map, not a provider panic: a zero types.Map carries a nil
// element type, and ToTerraformValue reads m.ElementType before its null
// branch, so an unassigned field kills the state encode. The literal in Read
// initializes it; this pins that.
func TestDiagnosticsStateDirsMissingReadsNull(t *testing.T) {
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"version": "3.2.0"}, "")
	}))
	defer srv.Close()

	d := &diagnosticsDataSource{client: client.New(srv.URL, "tok", false, "test")}
	schemaResp := &datasource.SchemaResponse{}
	d.Schema(ctx, datasource.SchemaRequest{}, schemaResp)

	typ := schemaResp.Schema.Type().TerraformType(ctx)
	attrTypes := typ.(tftypes.Object).AttributeTypes
	nullVals := map[string]tftypes.Value{}
	for name, at := range attrTypes {
		nullVals[name] = tftypes.NewValue(at, nil)
	}
	nullObj := tftypes.NewValue(typ, nullVals)
	req := datasource.ReadRequest{Config: tfsdk.Config{Raw: nullObj, Schema: schemaResp.Schema}}
	resp := &datasource.ReadResponse{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(typ, nil)},
	}
	d.Read(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("a read against a pre-3.3.0 answer must not fail: %v", resp.Diagnostics.Errors())
	}
	var m diagnosticsModel
	resp.Diagnostics.Append(resp.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		t.Fatalf("decoding the read state: %v", resp.Diagnostics.Errors())
	}
	if !m.StateDirs.IsNull() {
		t.Errorf("state_dirs = %v, want null", m.StateDirs)
	}
}
