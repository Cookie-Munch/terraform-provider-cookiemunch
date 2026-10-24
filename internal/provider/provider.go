// Package provider is the Cookie Munch Terraform provider.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// DefaultBaseURL is the hosted API.
const DefaultBaseURL = "https://api.cookiemunch.net"

type cookieMunchProvider struct{ version string }

type providerModel struct {
	APIKey  types.String `tfsdk:"api_key"`
	BaseURL types.String `tfsdk:"base_url"`
}

// New returns the provider factory.
func New(version string) func() provider.Provider {
	return func() provider.Provider { return &cookieMunchProvider{version: version} }
}

func (p *cookieMunchProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "cookiemunch"
	resp.Version = p.version
}

func (p *cookieMunchProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Cookie Munch: sites, their consent configuration, cookie declarations, webhooks, partner links and connected systems, as code.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Optional: true, Sensitive: true,
				Description: "A Cookie Munch API key. Defaults to COOKIEMUNCH_API_KEY. Prefer a scoped key (sites:write, webhooks:write, dsar:write) or one from `cookiemunch token exchange`.",
			},
			"base_url": schema.StringAttribute{
				Optional:    true,
				Description: "The API origin. Defaults to COOKIEMUNCH_BASE_URL, then https://api.cookiemunch.net.",
			},
		},
	}
}

func (p *cookieMunchProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var m providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	key := os.Getenv("COOKIEMUNCH_API_KEY")
	if !m.APIKey.IsNull() && !m.APIKey.IsUnknown() {
		key = m.APIKey.ValueString()
	}
	base := os.Getenv("COOKIEMUNCH_BASE_URL")
	if !m.BaseURL.IsNull() && !m.BaseURL.IsUnknown() {
		base = m.BaseURL.ValueString()
	}
	if base == "" {
		base = DefaultBaseURL
	}
	if key == "" {
		resp.Diagnostics.AddError("No API key", "Set api_key in the provider block or COOKIEMUNCH_API_KEY in the environment.")
		return
	}
	c := newClient(base, key, p.version)
	resp.ResourceData = c
	resp.DataSourceData = c
}

func (p *cookieMunchProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewSiteResource,
		NewSiteConfigResource,
		NewCookieDeclarationResource,
		NewWebhookResource,
		NewPartnerLinkResource,
		NewDsarConnectionResource,
	}
}

func (p *cookieMunchProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{NewMeDataSource, NewSiteDataSource}
}

// clientFrom is the Configure step every resource and data source shares.
func clientFrom(data any, add func(summary, detail string)) *Client {
	if data == nil {
		return nil // the provider is not configured yet (validation)
	}
	c, ok := data.(*Client)
	if !ok {
		add("Unexpected provider data", "This is a bug in the provider.")
		return nil
	}
	return c
}
