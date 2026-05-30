package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	"github.com/raspbeguy/terraform-provider-uapi/internal/client"
)

var (
	_ datasource.DataSource              = &systemDataSource{}
	_ datasource.DataSourceWithConfigure = &systemDataSource{}
)

type systemDataSource struct{ client *client.Client }

func NewSystemDataSource() datasource.DataSource { return &systemDataSource{} }

func (d *systemDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_system"
}

func (d *systemDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *systemDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "The global system settings (uci system.system).",
		Attributes: map[string]dsschema.Attribute{
			"id":           dsschema.StringAttribute{Computed: true},
			"managed":      dsschema.BoolAttribute{Computed: true},
			"hostname":     dsschema.StringAttribute{Computed: true},
			"description":  dsschema.StringAttribute{Computed: true},
			"notes":        dsschema.StringAttribute{Computed: true},
			"timezone":     dsschema.StringAttribute{Computed: true},
			"zonename":     dsschema.StringAttribute{Computed: true},
			"log_size":     dsschema.StringAttribute{Computed: true},
			"log_ip":       dsschema.StringAttribute{Computed: true},
			"log_proto":    dsschema.StringAttribute{Computed: true},
			"log_remote":   dsschema.BoolAttribute{Computed: true},
			"urandom_seed": dsschema.BoolAttribute{Computed: true},
		},
	}
}

func (d *systemDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	obj, found, err := d.client.GetObject(ctx, systemPath)
	if err != nil {
		resp.Diagnostics.AddError("Error reading system settings", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("System settings not found", "The system singleton is missing on the router")
		return
	}
	var m systemModel
	(&systemResource{}).read(ctx, obj, &m)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
