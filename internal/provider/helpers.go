package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/openwrt-iac/terraform-provider-uapi/internal/client"
)

func clientFromResourceConfigure(req resource.ConfigureRequest, resp *resource.ConfigureResponse) *client.Client {
	if req.ProviderData == nil {
		return nil
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data",
			fmt.Sprintf("Expected *client.Client, got %T. This is a provider bug.", req.ProviderData),
		)
		return nil
	}
	return c
}

func clientFromDataSourceConfigure(req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) *client.Client {
	if req.ProviderData == nil {
		return nil
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data",
			fmt.Sprintf("Expected *client.Client, got %T. This is a provider bug.", req.ProviderData),
		)
		return nil
	}
	return c
}

func strVal(m map[string]any, key string) types.String {
	v, ok := m[key]
	if !ok || v == nil {
		return types.StringNull()
	}
	switch s := v.(type) {
	case string:
		return types.StringValue(s)
	default:
		return types.StringValue(fmt.Sprintf("%v", v))
	}
}

func boolVal(m map[string]any, key string) types.Bool {
	v, ok := m[key]
	if !ok || v == nil {
		return types.BoolNull()
	}
	if b, ok := v.(bool); ok {
		return types.BoolValue(b)
	}
	return types.BoolNull()
}

func int64Val(m map[string]any, key string) types.Int64 {
	v, ok := m[key]
	if !ok || v == nil {
		return types.Int64Null()
	}
	switch n := v.(type) {
	case float64:
		return types.Int64Value(int64(n))
	case int64:
		return types.Int64Value(n)
	case int:
		return types.Int64Value(int64(n))
	}
	return types.Int64Null()
}

// listVal converts a JSON string array into a types.List. A missing or null
// value becomes an empty list, matching the API which always emits an array.
func listVal(ctx context.Context, m map[string]any, key string) (types.List, diag.Diagnostics) {
	raw, ok := m[key]
	items := []string{}
	if ok && raw != nil {
		if arr, ok := raw.([]any); ok {
			for _, e := range arr {
				if s, ok := e.(string); ok {
					items = append(items, s)
				} else if e != nil {
					items = append(items, fmt.Sprintf("%v", e))
				}
			}
		}
	}
	return types.ListValueFrom(ctx, types.StringType, items)
}

func putStr(m map[string]any, key string, v types.String) {
	if !v.IsNull() && !v.IsUnknown() {
		m[key] = v.ValueString()
	}
}

func putBool(m map[string]any, key string, v types.Bool) {
	if !v.IsNull() && !v.IsUnknown() {
		m[key] = v.ValueBool()
	}
}

func putInt64(m map[string]any, key string, v types.Int64) {
	if !v.IsNull() && !v.IsUnknown() {
		m[key] = v.ValueInt64()
	}
}

// boolValDefault maps a read-only/computed boolean, defaulting to false when the
// API omits it (used for has_* companions of write-only fields).
func boolValDefault(m map[string]any, key string) types.Bool {
	v := boolVal(m, key)
	if v.IsNull() {
		return types.BoolValue(false)
	}
	return v
}

func putList(ctx context.Context, m map[string]any, key string, v types.List, diags *diag.Diagnostics) {
	if v.IsNull() || v.IsUnknown() {
		return
	}
	var items []string
	diags.Append(v.ElementsAs(ctx, &items, false)...)
	m[key] = items
}

// importByID resolves an imported id, adopting the section first when it is not
// yet uapi-managed, and writes the resulting id into state. Adoption mutates the
// router (it renames the underlying uci section), so it emits a warning that
// names the old and new ids.
func importByID(ctx context.Context, c *client.Client, collection, label string, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, adopted, err := resolveImportID(ctx, c, collection, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing "+label, err.Error())
		return
	}
	if adopted {
		resp.Diagnostics.AddWarning(
			"Adopted an unmanaged section",
			fmt.Sprintf("%s %q was not uapi-managed, so it was adopted and renamed to %q. "+
				"This import mutated the router; the resource id is now %q.", label, req.ID, id, id),
		)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

// resolveImportID looks up an imported id and, when the section is not yet
// uapi-managed, adopts it (renaming it to a stable ULID). adopted reports
// whether an adoption (a mutating rename) took place.
func resolveImportID(ctx context.Context, c *client.Client, collection, importedID string) (id string, adopted bool, err error) {
	obj, _, found, err := c.GetObject(ctx, fmt.Sprintf("/%s/%s", collection, importedID))
	if err != nil {
		return "", false, err
	}
	if !found {
		return "", false, fmt.Errorf("no resource found at /%s/%s", collection, importedID)
	}
	if managed, ok := obj["managed"].(bool); ok && !managed {
		adoptedObj, _, err := c.Post(ctx, fmt.Sprintf("/%s/%s/adopt", collection, importedID), nil, "")
		if err != nil {
			return "", false, fmt.Errorf("adopting unmanaged section: %w", err)
		}
		if newID, ok := adoptedObj["id"].(string); ok && newID != "" {
			return newID, true, nil
		}
		return "", false, fmt.Errorf("adopt response missing id")
	}
	if existingID, ok := obj["id"].(string); ok && existingID != "" {
		return existingID, false, nil
	}
	return importedID, false, nil
}

// writeErr standardizes diagnostics for write failures, giving the 412 stale
// If-Match case a clear, actionable message instead of a raw error envelope.
func writeErr(diags *diag.Diagnostics, action, label string, err error) {
	if client.IsPreconditionFailed(err) {
		diags.AddError(
			label+" changed outside Terraform",
			"The "+label+" was modified on the router since Terraform last read it (If-Match / ETag mismatch). "+
				"Run a refresh (or re-plan) to pick up the current state, then retry.\n\n"+err.Error(),
		)
		return
	}
	diags.AddError("Error "+action+" "+label, err.Error())
}
