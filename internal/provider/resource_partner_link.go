package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type apiPartnerLink struct {
	ID         string   `json:"id"`
	Partner    string   `json:"partner"`
	Resource   string   `json:"resource"`
	Scopes     []string `json:"scopes"`
	ExpiresAt  *string  `json:"expires_at"`
	Status     string   `json:"status"`
	ExternalID string   `json:"externalId,omitempty"`
	Label      string   `json:"label,omitempty"`
}

type partnerLinkModel struct {
	ID         types.String `tfsdk:"id"`
	Partner    types.String `tfsdk:"partner"`
	Resource   types.String `tfsdk:"resource"`
	Scopes     types.List   `tfsdk:"scopes"`
	ExpiresAt  types.String `tfsdk:"expires_at"`
	ExternalID types.String `tfsdk:"external_id"`
	Label      types.String `tfsdk:"label"`
	Status     types.String `tfsdk:"status"`
}

type partnerLinkResource struct{ c *Client }

// NewPartnerLinkResource is cookiemunch_partner_link.
func NewPartnerLinkResource() resource.Resource { return &partnerLinkResource{} }

func (r *partnerLinkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_partner_link"
}

func (r *partnerLinkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "A grant to a partner platform (any partner the deployment knows): what it may do for one site or the organisation. No secret is created. A grant cannot be edited: any change revokes it and grants a new one.",
		Attributes: map[string]schema.Attribute{
			"id":       schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"partner":  schema.StringAttribute{Required: true, PlanModifiers: replace, Description: "e.g. forumulate, atlas, chatly, referalli."},
			"resource": schema.StringAttribute{Required: true, PlanModifiers: replace, Description: "`site:<cbid>` or `org:<org id>`."},
			"scopes": schema.ListAttribute{
				Required: true, ElementType: types.StringType, PlanModifiers: []planmodifier.List{listplanmodifier.RequiresReplace()},
				Description: "consent:read, receipt:read, dsar:read, dsar:write, dsar:rights-links, sites:read. Never more than the provider's key holds.",
			},
			"expires_at":  schema.StringAttribute{Optional: true, PlanModifiers: replace, Description: "ISO 8601. Unset: until revoked."},
			"external_id": schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()}, Description: "The partner's id for the account."},
			"label":       schema.StringAttribute{Optional: true, PlanModifiers: replace},
			"status":      schema.StringAttribute{Computed: true, Description: "active or waiting_on_partner."},
		},
	}
}

func (r *partnerLinkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.c = clientFrom(req.ProviderData, resp.Diagnostics.AddError)
}

func (r *partnerLinkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (m *partnerLinkModel) fill(l apiPartnerLink) {
	m.ID = types.StringValue(l.ID)
	m.Partner = types.StringValue(l.Partner)
	m.Resource = types.StringValue(l.Resource)
	m.Scopes = listOf(l.Scopes)
	if l.ExpiresAt != nil && *l.ExpiresAt != "" && m.ExpiresAt.IsNull() {
		m.ExpiresAt = types.StringValue(*l.ExpiresAt)
	}
	m.ExternalID = strOrNull(l.ExternalID)
	m.Label = strOrNull(l.Label)
	m.Status = types.StringValue(l.Status)
}

func (r *partnerLinkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan partnerLinkModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	scopes, _ := stringList(ctx, plan.Scopes)
	body := map[string]any{"partner": plan.Partner.ValueString(), "resource": plan.Resource.ValueString(), "scopes": scopes}
	for k, v := range map[string]types.String{"expires_at": plan.ExpiresAt, "externalId": plan.ExternalID, "label": plan.Label} {
		if s := optStr(v); s != "" {
			body[k] = s
		}
	}
	var l apiPartnerLink
	if err := r.c.Do(ctx, "POST", "/partner_links", body, &l); err != nil {
		resp.Diagnostics.AddError("Granting the partner link failed", err.Error())
		return
	}
	expires := plan.ExpiresAt
	plan.fill(l)
	plan.ExpiresAt = expires // the configuration's own spelling of the time
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *partnerLinkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state partnerLinkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var all []apiPartnerLink
	if err := r.c.Do(ctx, "GET", "/partner_links", nil, &all); err != nil {
		resp.Diagnostics.AddError("Reading the partner links failed", err.Error())
		return
	}
	for _, l := range all {
		if l.ID != state.ID.ValueString() {
			continue
		}
		// A revoked or expired grant is gone: the next apply grants it again.
		if l.Status == "revoked" || l.Status == "expired" {
			break
		}
		state.fill(l)
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
		return
	}
	resp.State.RemoveResource(ctx)
}

// Update is never called: every attribute replaces the grant.
func (r *partnerLinkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("A partner link cannot be edited", "Change it by replacing it.")
}

func (r *partnerLinkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state partnerLinkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.Do(ctx, "DELETE", "/partner_links/"+esc(state.ID.ValueString()), nil, nil); err != nil && !IsNotFound(err) {
		resp.Diagnostics.AddError("Revoking the partner link failed", err.Error())
	}
}
