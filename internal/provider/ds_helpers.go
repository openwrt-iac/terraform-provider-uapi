package provider

import (
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Shared data-source attribute builders, used by generated and hand-written data sources.

func dsIDAttribute() dsschema.StringAttribute {
	return dsschema.StringAttribute{Required: true, Description: "Resource id to look up."}
}

func dsManagedAttribute() dsschema.BoolAttribute {
	return dsComputedBool("Whether the underlying uci section is uapi-managed.")
}

func dsComputedString(desc string) dsschema.StringAttribute {
	return dsschema.StringAttribute{Computed: true, Description: desc}
}

func dsComputedBool(desc string) dsschema.BoolAttribute {
	return dsschema.BoolAttribute{Computed: true, Description: desc}
}

func dsComputedInt64(desc string) dsschema.Int64Attribute {
	return dsschema.Int64Attribute{Computed: true, Description: desc}
}

func dsComputedStringList(desc string) dsschema.ListAttribute {
	return dsschema.ListAttribute{ElementType: types.StringType, Computed: true, Description: desc}
}
