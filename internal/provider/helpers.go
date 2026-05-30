package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/raspbeguy/terraform-provider-uapi/internal/client"
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

func putList(ctx context.Context, m map[string]any, key string, v types.List, diags *diag.Diagnostics) {
	if v.IsNull() || v.IsUnknown() {
		return
	}
	var items []string
	diags.Append(v.ElementsAs(ctx, &items, false)...)
	m[key] = items
}

// resolveImportID looks up an imported id and, when the section is not yet
// uapi-managed, adopts it (renaming it to a stable ULID). It returns the id the
// resource should track. Note that importing an unmanaged section mutates the
// router: the underlying uci section is renamed.
func resolveImportID(ctx context.Context, c *client.Client, collection, importedID string) (string, error) {
	obj, found, err := c.GetObject(ctx, fmt.Sprintf("/%s/%s", collection, importedID))
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("no resource found at /%s/%s", collection, importedID)
	}
	if managed, ok := obj["managed"].(bool); ok && !managed {
		adopted, err := c.Post(ctx, fmt.Sprintf("/%s/%s/adopt", collection, importedID), nil)
		if err != nil {
			return "", fmt.Errorf("adopting unmanaged section: %w", err)
		}
		if id, ok := adopted["id"].(string); ok && id != "" {
			return id, nil
		}
		return "", fmt.Errorf("adopt response missing id")
	}
	if id, ok := obj["id"].(string); ok && id != "" {
		return id, nil
	}
	return importedID, nil
}
