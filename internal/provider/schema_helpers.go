package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func computedIDAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		Computed:      true,
		Description:   "Stable resource id assigned by uapi (a prefixed ULID).",
		PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
	}
}

// optionalComputedIDAttribute is the id of a collection resource: settable at
// create to pick the uci section name (uapi >= 2.2.0), otherwise server-assigned.
// RequiresReplace because the section name is create-only (a rename is a new
// section); UseStateForUnknown keeps a server-assigned id stable so an unset id
// never shows a perpetual diff. Setting id to an existing section's name lets
// `terraform import` then plan reconcile with no replace.
func optionalComputedIDAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		Optional:    true,
		Computed:    true,
		Description: "Resource id. Set it at create to choose the uci section name (e.g. `lan`); omit it to let uapi assign a prefixed ULID. Create-only: changing it forces replacement.",
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.RequiresReplace(),
			stringplanmodifier.UseStateForUnknown(),
		},
	}
}

// deprecatedAliasRequiresReplace is the plan modifier for a deprecated create-only
// alias of `id` (currently `name` on network interfaces). It forces replacement
// only when the alias is set to a *different* non-null value (a real section
// rename). Dropping the alias to migrate to `id` (config goes null) or leaving it
// unchanged is not a replace, so `name = "x"` -> `id = "x"` is a no-op rather than
// a destroy+recreate. `id` keeps its own RequiresReplace, so renaming the section
// still replaces.
func deprecatedAliasRequiresReplace() planmodifier.String {
	return stringplanmodifier.RequiresReplaceIf(
		func(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
			resp.RequiresReplace = !req.ConfigValue.IsNull() &&
				!req.ConfigValue.IsUnknown() &&
				!req.ConfigValue.Equal(req.StateValue)
		},
		"Replaced only when changed to a different non-null value (a section rename); removing it to use `id` is not a replace.",
		"Replaced only when changed to a different non-null value (a section rename); removing it to use `id` is not a replace.",
	)
}

func managedAttribute() schema.BoolAttribute {
	return schema.BoolAttribute{
		Computed:      true,
		Description:   "Whether the underlying uci section is uapi-managed.",
		PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
	}
}

// etagAttribute carries the uapi ETag of the last read/write so Update and
// Delete can send If-Match for optimistic concurrency. Plain Computed (no
// UseStateForUnknown): the value genuinely changes on every write.
func etagAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		Computed:    true,
		Description: "Opaque ETag of the resource's current state, used for If-Match optimistic concurrency.",
	}
}

// The optionalComputed* helpers cover fields uapi normalizes or defaults
// server-side. Optional+Computed plus UseStateForUnknown stops omitted fields
// from showing a perpetual diff against the value the server fills in.
func optionalComputedStringList(desc string) schema.ListAttribute {
	return schema.ListAttribute{
		ElementType:   types.StringType,
		Optional:      true,
		Computed:      true,
		Description:   desc,
		PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
	}
}

func optionalComputedString(desc string) schema.StringAttribute {
	return schema.StringAttribute{
		Optional:      true,
		Computed:      true,
		Description:   desc,
		PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
	}
}

func optionalComputedBool(desc string) schema.BoolAttribute {
	return schema.BoolAttribute{
		Optional:      true,
		Computed:      true,
		Description:   desc,
		PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
	}
}

func optionalComputedInt64(desc string) schema.Int64Attribute {
	return schema.Int64Attribute{
		Optional:      true,
		Computed:      true,
		Description:   desc,
		PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
	}
}

// diagsink lets resource code convert lists inline without threading diag slices through every call.
type diagsink struct {
	d *diag.Diagnostics
}

func newDiagsink(d *diag.Diagnostics) *diagsink {
	return &diagsink{d: d}
}

func (s *diagsink) list(v types.List, dd diag.Diagnostics) types.List {
	s.d.Append(dd...)
	return v
}
