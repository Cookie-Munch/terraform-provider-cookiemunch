package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type apiWebhook struct {
	ID     string   `json:"id"`
	URL    string   `json:"url"`
	Events []string `json:"events"`
	Cbid   *string  `json:"cbid"`
	Active bool     `json:"active"`
	State  string   `json:"state"`
	Secret string   `json:"secret,omitempty"`
}

type webhookModel struct {
	ID     types.String `tfsdk:"id"`
	URL    types.String `tfsdk:"url"`
	Events types.List   `tfsdk:"events"`
	Cbid   types.String `tfsdk:"cbid"`
	Active types.Bool   `tfsdk:"active"`
	State  types.String `tfsdk:"state"`
	Secret types.String `tfsdk:"secret"`
}

type webhookResource struct{ c *Client }

// NewWebhookResource is cookiemunch_webhook.
func NewWebhookResource() resource.Resource { return &webhookResource{} }

func (r *webhookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}

func (r *webhookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A signed webhook subscription. The signing secret is known only from creation: an imported webhook has none in state.",
		Attributes: map[string]schema.Attribute{
			"id":     schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"url":    schema.StringAttribute{Required: true, Description: "An https URL."},
			"events": schema.ListAttribute{Required: true, ElementType: types.StringType, Description: "e.g. consent.recorded, dsar.created, scan.completed."},
			"cbid":   schema.StringAttribute{Optional: true, Description: "Limit it to one site; unset for every site in the organisation."},
			"active": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), Description: "false pauses delivery."},
			"state":  schema.StringAttribute{Computed: true, Description: "healthy, failing, disabled or paused."},
			"secret": schema.StringAttribute{
				Computed: true, Sensitive: true, Description: "The signing secret (shown once, at creation).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *webhookResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.c = clientFrom(req.ProviderData, resp.Diagnostics.AddError)
}

func (r *webhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (m *webhookModel) fill(w apiWebhook) {
	m.ID = types.StringValue(w.ID)
	m.URL = types.StringValue(w.URL)
	m.Events = listOf(w.Events)
	if w.Cbid != nil && *w.Cbid != "" {
		m.Cbid = types.StringValue(*w.Cbid)
	} else {
		m.Cbid = types.StringNull()
	}
	m.Active = types.BoolValue(w.Active)
	m.State = types.StringValue(w.State)
	if w.Secret != "" {
		m.Secret = types.StringValue(w.Secret)
	}
}

func (m webhookModel) body(ctx context.Context) map[string]any {
	events, _ := stringList(ctx, m.Events)
	b := map[string]any{"url": m.URL.ValueString(), "events": events}
	if m.Cbid.IsNull() {
		b["cbid"] = nil
	} else {
		b["cbid"] = m.Cbid.ValueString()
	}
	return b
}

func (r *webhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan webhookModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var w apiWebhook
	if err := r.c.Do(ctx, "POST", "/webhooks", plan.body(ctx), &w); err != nil {
		resp.Diagnostics.AddError("Creating the webhook failed", err.Error())
		return
	}
	if !plan.Active.ValueBool() {
		var paused apiWebhook
		if err := r.c.Do(ctx, "PATCH", "/webhooks/"+esc(w.ID), map[string]any{"active": false}, &paused); err != nil {
			resp.Diagnostics.AddError("Pausing the new webhook failed", err.Error())
			return
		}
		paused.Secret = w.Secret
		w = paused
	}
	plan.fill(w)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *webhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state webhookModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var all []apiWebhook
	if err := r.c.Do(ctx, "GET", "/webhooks", nil, &all); err != nil {
		resp.Diagnostics.AddError("Reading the webhooks failed", err.Error())
		return
	}
	for _, w := range all {
		if w.ID == state.ID.ValueString() {
			state.fill(w)
			if state.Secret.IsUnknown() {
				state.Secret = types.StringNull()
			}
			resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *webhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state webhookModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := plan.body(ctx)
	body["active"] = plan.Active.ValueBool()
	var w apiWebhook
	if err := r.c.Do(ctx, "PATCH", "/webhooks/"+esc(state.ID.ValueString()), body, &w); err != nil {
		resp.Diagnostics.AddError("Updating the webhook failed", err.Error())
		return
	}
	plan.Secret = state.Secret
	plan.fill(w)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *webhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state webhookModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.Do(ctx, "DELETE", "/webhooks/"+esc(state.ID.ValueString()), nil, nil); err != nil && !IsNotFound(err) {
		resp.Diagnostics.AddError("Deleting the webhook failed", err.Error())
	}
}
