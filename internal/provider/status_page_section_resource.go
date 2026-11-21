package provider

import (
	"context"
	"slices"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fclairamb/terraform-provider-solidping/internal/client"
)

var (
	_ resource.Resource                = (*statusPageSectionResource)(nil)
	_ resource.ResourceWithConfigure   = (*statusPageSectionResource)(nil)
	_ resource.ResourceWithImportState = (*statusPageSectionResource)(nil)
)

type statusPageSectionResource struct {
	api *client.Client
}

type sectionSelectorModel struct {
	All           types.Bool   `tfsdk:"all"`
	Labels        types.Map    `tfsdk:"labels"`
	CheckGroupUID types.String `tfsdk:"check_group_uid"`
}

type sectionResourceModel struct {
	CheckUID      types.String `tfsdk:"check_uid"`
	CheckGroupUID types.String `tfsdk:"check_group_uid"`
	PublicName    types.String `tfsdk:"public_name"`
	Explanation   types.String `tfsdk:"explanation"`
}

// key identifies the target of a section resource.
func (m *sectionResourceModel) key() string {
	if !m.CheckUID.IsNull() && m.CheckUID.ValueString() != "" {
		return "check:" + m.CheckUID.ValueString()
	}

	return "group:" + m.CheckGroupUID.ValueString()
}

type statusPageSectionModel struct {
	ID            types.String           `tfsdk:"id"`
	StatusPageUID types.String           `tfsdk:"status_page_uid"`
	Name          types.String           `tfsdk:"name"`
	Slug          types.String           `tfsdk:"slug"`
	Position      types.Int64            `tfsdk:"position"`
	Selector      *sectionSelectorModel  `tfsdk:"selector"`
	Resources     []sectionResourceModel `tfsdk:"resources"`
}

func newStatusPageSectionResource() resource.Resource { return &statusPageSectionResource{} }

func (r *statusPageSectionResource) Metadata(
	_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_status_page_section"
}

func (r *statusPageSectionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A section of a status page, with the components it shows.\n\n" +
			"A section is either hand-curated (`resources`, an ordered list of checks and check groups) or " +
			"dynamic (`selector`, a rule the server evaluates to pick the checks).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "UID of the section.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"status_page_uid": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UID of the status page. Changing it replaces the section.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Title of the section.",
			},
			"slug": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "URL-friendly identifier, unique in the page. Derived from `name` when omitted.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"position": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Position of the section in the page (0 is the top). " +
					"Appended after the existing sections when omitted.",
			},
			"selector": schema.SingleNestedAttribute{
				Optional: true,
				MarkdownDescription: "Dynamic membership rule. Exactly one of `all`, `labels` and `check_group_uid` " +
					"must be set. Conflicts with `resources`.",
				Attributes: map[string]schema.Attribute{
					"all": schema.BoolAttribute{
						Optional:            true,
						MarkdownDescription: "Select every check of the organization.",
					},
					"labels": schema.MapAttribute{
						ElementType:         types.StringType,
						Optional:            true,
						MarkdownDescription: "Select the checks carrying all of these labels.",
					},
					"check_group_uid": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Select every check of this check group (UID).",
					},
				},
				Validators: []validator.Object{
					objectvalidator.ConflictsWith(path.MatchRoot("resources")),
				},
			},
			"resources": schema.ListNestedAttribute{
				Optional: true,
				MarkdownDescription: "Components shown in the section, in display order. Each one targets a check " +
					"(`check_uid`) or a check group (`check_group_uid`), shown as one aggregated component.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"check_uid": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "UID of the check to show.",
							Validators: []validator.String{
								stringvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("check_group_uid")),
							},
						},
						"check_group_uid": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "UID of the check group to show.",
						},
						"public_name": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Name shown on the page instead of the check or group name.",
						},
						"explanation": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Short text shown with the component.",
						},
					},
				},
				Validators: []validator.List{
					listvalidator.UniqueValues(),
				},
			},
		},
	}
}

func (r *statusPageSectionResource) Configure(
	_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse,
) {
	r.api = configureClient(req.ProviderData, &resp.Diagnostics)
}

func selectorPayload(ctx context.Context, selector *sectionSelectorModel, diags *diag.Diagnostics) any {
	if selector == nil {
		return nil
	}

	out := client.Payload{}

	if selector.All.ValueBool() {
		out["all"] = true
	}

	if !selector.Labels.IsNull() && !selector.Labels.IsUnknown() {
		labels := map[string]string{}

		diags.Append(selector.Labels.ElementsAs(ctx, &labels, false)...)

		out["labels"] = labels
	}

	putString(out, "checkGroupUid", selector.CheckGroupUID)

	return out
}

func resourcePayload(res *sectionResourceModel, create bool) client.Payload {
	body := client.Payload{}

	if create {
		putString(body, "checkUid", res.CheckUID)
		putString(body, "checkGroupUid", res.CheckGroupUID)
		putString(body, "publicName", res.PublicName)
		putString(body, "explanation", res.Explanation)

		return body
	}

	body["publicName"] = stringOrEmpty(res.PublicName)
	body["explanation"] = stringOrEmpty(res.Explanation)

	return body
}

func serverResourceKey(res *client.StatusPageResource) string {
	if res.CheckUID != "" {
		return "check:" + res.CheckUID
	}

	return "group:" + res.CheckGroupUID
}

// curatedResources lists the hand-curated resources of a section (the ones a
// selector materialized are owned by the server), in display order.
func (r *statusPageSectionResource) curatedResources(
	ctx context.Context, pageUID, sectionUID string,
) ([]client.StatusPageResource, error) {
	all, err := r.api.ListStatusPageResources(ctx, pageUID, sectionUID)
	if err != nil {
		return nil, err
	}

	curated := make([]client.StatusPageResource, 0, len(all))

	for i := range all {
		if !all[i].ManagedBySelector {
			curated = append(curated, all[i])
		}
	}

	sort.SliceStable(curated, func(i, j int) bool { return curated[i].Position < curated[j].Position })

	return curated, nil
}

// syncResources makes the curated resources of the section match the plan:
// existing targets are updated in place (their UID is kept), new ones are
// added, the others removed, then the whole list is put in order.
func (r *statusPageSectionResource) syncResources(
	ctx context.Context, pageUID, sectionUID string, desired []sectionResourceModel,
) error {
	current, err := r.curatedResources(ctx, pageUID, sectionUID)
	if err != nil {
		return err
	}

	byKey := make(map[string]*client.StatusPageResource, len(current))
	for i := range current {
		byKey[serverResourceKey(&current[i])] = &current[i]
	}

	order := make([]string, 0, len(desired))

	for i := range desired {
		want := &desired[i]

		existing, found := byKey[want.key()]
		if !found {
			created, err := r.api.CreateStatusPageResource(ctx, pageUID, sectionUID, resourcePayload(want, true))
			if err != nil {
				return err
			}

			order = append(order, created.UID)

			continue
		}

		delete(byKey, want.key())

		order = append(order, existing.UID)

		if existing.PublicName != stringOrEmpty(want.PublicName) || existing.Explanation != stringOrEmpty(want.Explanation) {
			err := r.api.UpdateStatusPageResource(ctx, pageUID, sectionUID, existing.UID, resourcePayload(want, false))
			if err != nil {
				return err
			}
		}
	}

	for _, stale := range byKey {
		if err := r.api.DeleteStatusPageResource(ctx, pageUID, sectionUID, stale.UID); err != nil && !client.IsNotFound(err) {
			return err
		}
	}

	currentOrder := make([]string, 0, len(current))
	for i := range current {
		if _, stale := byKey[serverResourceKey(&current[i])]; !stale {
			currentOrder = append(currentOrder, current[i].UID)
		}
	}

	if len(order) > 1 && !slices.Equal(order, currentOrder) {
		return r.api.ReorderStatusPageResources(ctx, pageUID, sectionUID, order)
	}

	return nil
}

func (m *statusPageSectionModel) apply(ctx context.Context, section *client.StatusPageSection, diags *diag.Diagnostics) {
	m.ID = types.StringValue(section.UID)
	m.Name = types.StringValue(section.Name)
	m.Slug = types.StringValue(section.Slug)
	m.Position = types.Int64Value(section.Position)

	if section.Selector == nil {
		m.Selector = nil

		return
	}

	selector := &sectionSelectorModel{
		All:           types.BoolNull(),
		Labels:        types.MapNull(types.StringType),
		CheckGroupUID: types.StringNull(),
	}

	if section.Selector.All {
		selector.All = types.BoolValue(true)
	}

	if len(section.Selector.Labels) > 0 {
		labels, d := types.MapValueFrom(ctx, types.StringType, section.Selector.Labels)
		diags.Append(d...)

		selector.Labels = labels
	}

	if section.Selector.CheckGroupUID != "" {
		selector.CheckGroupUID = types.StringValue(section.Selector.CheckGroupUID)
	}

	m.Selector = selector
}

// applyResources copies the curated resources into the model. prior carries
// the values of the configuration, matched by target, so that an empty
// optional string keeps its null-or-empty spelling.
func (m *statusPageSectionModel) applyResources(resources []client.StatusPageResource) {
	prior := make(map[string]*sectionResourceModel, len(m.Resources))
	for i := range m.Resources {
		prior[m.Resources[i].key()] = &m.Resources[i]
	}

	if len(resources) == 0 && m.Resources == nil {
		return
	}

	out := make([]sectionResourceModel, 0, len(resources))

	for i := range resources {
		res := &resources[i]
		item := sectionResourceModel{
			CheckUID:      types.StringNull(),
			CheckGroupUID: types.StringNull(),
			PublicName:    types.StringNull(),
			Explanation:   types.StringNull(),
		}

		if before, ok := prior[serverResourceKey(res)]; ok {
			item.PublicName = before.PublicName
			item.Explanation = before.Explanation
		}

		if res.CheckUID != "" {
			item.CheckUID = types.StringValue(res.CheckUID)
		} else {
			item.CheckGroupUID = types.StringValue(res.CheckGroupUID)
		}

		item.PublicName = optionalString(res.PublicName, item.PublicName)
		item.Explanation = optionalString(res.Explanation, item.Explanation)

		out = append(out, item)
	}

	m.Resources = out
}

func (r *statusPageSectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan statusPageSectionModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	pageUID := plan.StatusPageUID.ValueString()
	body := client.Payload{"name": plan.Name.ValueString()}

	putString(body, "slug", plan.Slug)
	putInt(body, "position", plan.Position)

	if plan.Selector != nil {
		body["selector"] = selectorPayload(ctx, plan.Selector, &resp.Diagnostics)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	section, err := r.api.CreateStatusPageSection(ctx, pageUID, body)
	if err != nil {
		addAPIError(&resp.Diagnostics, "create the status page section", err)

		return
	}

	desired := plan.Resources

	plan.apply(ctx, section, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	if err := r.syncResources(ctx, pageUID, section.UID, desired); err != nil {
		addAPIError(&resp.Diagnostics, "set the resources of the status page section", err)
	}
}

func (r *statusPageSectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state statusPageSectionModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	pageUID := state.StatusPageUID.ValueString()

	section, err := r.api.GetStatusPageSection(ctx, pageUID, state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)

		return
	}

	if err != nil {
		addAPIError(&resp.Diagnostics, "read the status page section", err)

		return
	}

	resources, err := r.curatedResources(ctx, pageUID, section.UID)
	if err != nil {
		addAPIError(&resp.Diagnostics, "read the resources of the status page section", err)

		return
	}

	state.apply(ctx, section, &resp.Diagnostics)
	state.applyResources(resources)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *statusPageSectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state statusPageSectionModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	pageUID := state.StatusPageUID.ValueString()
	body := client.Payload{"name": plan.Name.ValueString()}

	putString(body, "slug", plan.Slug)
	putInt(body, "position", plan.Position)

	// Three-state on the API: an object replaces the rule, null clears it.
	if plan.Selector != nil || state.Selector != nil {
		body["selector"] = selectorPayload(ctx, plan.Selector, &resp.Diagnostics)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	section, err := r.api.UpdateStatusPageSection(ctx, pageUID, state.ID.ValueString(), body)
	if err != nil {
		addAPIError(&resp.Diagnostics, "update the status page section", err)

		return
	}

	desired := plan.Resources

	plan.apply(ctx, section, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	if err := r.syncResources(ctx, pageUID, section.UID, desired); err != nil {
		addAPIError(&resp.Diagnostics, "set the resources of the status page section", err)
	}
}

func (r *statusPageSectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state statusPageSectionModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.api.DeleteStatusPageSection(ctx, state.StatusPageUID.ValueString(), state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		addAPIError(&resp.Diagnostics, "delete the status page section", err)
	}
}

// ImportState accepts `org/statusPageUid/sectionUid` (the org is optional).
func (r *statusPageSectionResource) ImportState(
	ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse,
) {
	parts := strings.Split(req.ID, "/")

	if len(parts) < 2 || len(parts) > 3 || slices.Contains(parts, "") {
		resp.Diagnostics.AddError("Invalid import ID", "Expected `org/statusPageUid/sectionUid`, got `"+req.ID+"`.")

		return
	}

	if len(parts) == 3 {
		if !checkImportOrg(r.api, parts[0], &resp.Diagnostics) {
			return
		}

		parts = parts[1:]
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("status_page_uid"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}
