package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	"github.com/openwrt-iac/terraform-provider-uapi/internal/client"
)

var (
	_ datasource.DataSource              = &packageFeedDataSource{}
	_ datasource.DataSourceWithConfigure = &packageFeedDataSource{}
)

type packageFeedDataSource struct{ client *client.Client }

func NewPackageFeedDataSource() datasource.DataSource { return &packageFeedDataSource{} }

func (d *packageFeedDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_package_feed"
}

func (d *packageFeedDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *packageFeedDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Look up a package feed by id.",
		Attributes: map[string]dsschema.Attribute{
			"id":            dsIDAttribute(),
			"managed":       dsManagedAttribute(),
			"name":          dsComputedString("Feed name."),
			"url":           dsComputedString("HTTP(S) URL of the package repository."),
			"filename":      dsComputedString("Feed list filename on the router."),
			"enabled":       dsComputedBool("Whether the feed is enabled."),
			"update_status": dsComputedString("Status of the last feed update."),
		},
	}
}

func (d *packageFeedDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m packageFeedModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, _, found, err := d.client.GetObject(ctx, "/"+packageFeedCollection+"/"+m.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading package feed", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Package feed not found", "No package feed with id "+m.ID.ValueString())
		return
	}
	(&packageFeedResource{}).read(ctx, obj, &m)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
