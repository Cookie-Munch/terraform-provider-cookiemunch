package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type apiExecutor struct {
	ID            string `json:"id"`
	System        string `json:"system"`
	Cbid          string `json:"cbid,omitempty"`
	BaseURL       string `json:"baseUrl"`
	KeyPrefix     string `json:"keyPrefix"`
	Auto          *bool  `json:"auto,omitempty"`
	Preset        string `json:"preset,omitempty"`
	PresetVersion int64  `json:"presetVersion,omitempty"`
	WebhookURL    string `json:"webhookUrl,omitempty"`
}

type connectionModel struct {
	ID            types.String `tfsdk:"id"`
	System        types.String `tfsdk:"system"`
	Preset        types.String `tfsdk:"preset"`
	PresetVersion types.Int64  `tfsdk:"preset_version"`
	BaseURL       types.String `tfsdk:"base_url"`
	SecretKey     types.String `tfsdk:"secret_key"`
	WebhookSecret types.String `tfsdk:"webhook_secret"`
	Cbid          types.String `tfsdk:"cbid"`
	Auto          types.Bool   `tfsdk:"auto"`
	SubjectNote   types.String `tfsdk:"subject_note"`
	Label         types.String `tfsdk:"label"`
	OrgID         types.String `tfsdk:"preset_org_id"`
	Profile       types.String `tfsdk:"profile_json"`
	KeyPrefix     types.String `tfsdk:"key_prefix"`
	WebhookURL    types.String `tfsdk:"webhook_url"`
}

type connectionResource struct{ c *Client }

// NewDsarConnectionResource is cookiemunch_dsar_connection.
func NewDsarConnectionResource() resource.Resource { return &connectionResource{} }

func (r *connectionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dsar_connection"
}

func (r *connectionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	rs := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		Description: "A connected system that runs part of a rights request, from a preset (`GET /v1/dsar/executors/presets`) or a profile. " +
			"Credentials are sent once and never read back. Any change makes a new connection (its task history stays with the old one).",
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Computed: true, PlanModifiers: keep},
			"system":         schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()}, Description: "The name it answers to in a plan, e.g. identity or crm. Defaults to the preset's."},
			"preset":         schema.StringAttribute{Optional: true, PlanModifiers: rs, Description: "Build the profile from this preset. Do not set profile_json with it."},
			"preset_version": schema.Int64Attribute{Optional: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}, Description: "Default: the latest."},
			"base_url":       schema.StringAttribute{Optional: true, PlanModifiers: rs, Description: "The system's https origin (unless the preset builds it)."},
			"secret_key":     schema.StringAttribute{Required: true, Sensitive: true, PlanModifiers: rs, Description: "Its API key. Stored encrypted; never returned."},
			"webhook_secret": schema.StringAttribute{Optional: true, Sensitive: true, PlanModifiers: rs, Description: "The signing secret of its webhook endpoint pointed at webhook_url."},
			"cbid":           schema.StringAttribute{Optional: true, PlanModifiers: rs, Description: "The site this system answers for. Unset: the whole organisation. This decides selection."},
			"auto":           schema.BoolAttribute{Optional: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()}, Description: "Open a sub-task automatically at fulfilment. Default true."},
			"subject_note":   schema.StringAttribute{Optional: true, PlanModifiers: rs, Description: "What the data subject reads on this system's line when it holds nothing about them."},
			"label":          schema.StringAttribute{Optional: true, PlanModifiers: rs, Description: "Your name for the credential."},
			"preset_org_id":  schema.StringAttribute{Optional: true, PlanModifiers: rs, Description: "The `orgId` param, for a preset that lists it."},
			"profile_json":   schema.StringAttribute{Optional: true, PlanModifiers: rs, Description: "An ExecutorProfile as JSON, instead of a preset."},
			"key_prefix":     schema.StringAttribute{Computed: true, PlanModifiers: keep, Description: "Names the stored key without revealing it."},
			"webhook_url":    schema.StringAttribute{Computed: true, PlanModifiers: keep, Description: "Point the system's webhook endpoint here."},
		},
	}
}

func (r *connectionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.c = clientFrom(req.ProviderData, resp.Diagnostics.AddError)
}

func (r *connectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *connectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan connectionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := map[string]any{"secretKey": plan.SecretKey.ValueString()}
	for k, v := range map[string]types.String{
		"system": plan.System, "preset": plan.Preset, "baseUrl": plan.BaseURL, "webhookSecret": plan.WebhookSecret,
		"cbid": plan.Cbid, "subjectNote": plan.SubjectNote, "label": plan.Label, "orgId": plan.OrgID,
	} {
		if s := optStr(v); s != "" {
			body[k] = s
		}
	}
	if !plan.PresetVersion.IsNull() {
		body["presetVersion"] = plan.PresetVersion.ValueInt64()
	}
	if !plan.Auto.IsNull() {
		body["auto"] = plan.Auto.ValueBool()
	}
	if s := optStr(plan.Profile); s != "" {
		var p any
		if err := json.Unmarshal([]byte(s), &p); err != nil {
			resp.Diagnostics.AddError("profile_json is not JSON", err.Error())
			return
		}
		body["profile"] = p
	}
	var e apiExecutor
	if err := r.c.Do(ctx, "POST", "/dsar/executors", body, &e); err != nil {
		resp.Diagnostics.AddError("Connecting the system failed", err.Error())
		return
	}
	plan.ID = types.StringValue(e.ID)
	plan.System = types.StringValue(e.System)
	plan.KeyPrefix = types.StringValue(e.KeyPrefix)
	plan.WebhookURL = types.StringValue(e.WebhookURL)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *connectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state connectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var e apiExecutor
	if err := r.c.Do(ctx, "GET", "/dsar/executors/"+esc(state.ID.ValueString()), nil, &e); err != nil {
		if IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Reading the connection failed", err.Error())
		return
	}
	state.System = types.StringValue(e.System)
	state.KeyPrefix = types.StringValue(e.KeyPrefix)
	if state.WebhookURL.IsNull() || state.WebhookURL.IsUnknown() {
		state.WebhookURL = strOrNull(e.WebhookURL)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update is never called: every argument replaces the connection.
func (r *connectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("A connection cannot be edited here", "Change it by replacing it.")
}

func (r *connectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state connectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.Do(ctx, "DELETE", "/dsar/executors/"+esc(state.ID.ValueString()), nil, nil); err != nil && !IsNotFound(err) {
		resp.Diagnostics.AddError("Removing the connection failed", err.Error())
	}
}
