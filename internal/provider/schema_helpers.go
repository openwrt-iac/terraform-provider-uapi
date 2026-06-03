package provider

import (
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
