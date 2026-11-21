package client

import (
	"context"
	"net/http"
)

// Payload is a JSON request body. Maps (rather than structs) let callers send
// an explicit `null`, which several PATCH endpoints read as "clear this field".
type Payload map[string]any

// Check is a monitoring check.
type Check struct {
	UID                 string            `json:"uid"`
	Name                string            `json:"name"`
	Slug                string            `json:"slug"`
	Description         *string           `json:"description"`
	Type                string            `json:"type"`
	Config              map[string]any    `json:"config"`
	ConfigPrivateKeys   []string          `json:"configPrivateKeys"`
	Enabled             bool              `json:"enabled"`
	Period              string            `json:"period"`
	Regions             []string          `json:"regions"`
	Placement           string            `json:"placement"`
	RegionCount         *int64            `json:"regionCount"`
	Labels              map[string]string `json:"labels"`
	CheckGroupUID       *string           `json:"checkGroupUid"`
	EscalationPolicyUID *string           `json:"escalationPolicyUid"`
	Status              string            `json:"status"`
}

// CreateCheck creates a check.
func (c *Client) CreateCheck(ctx context.Context, body Payload) (*Check, error) {
	var out Check

	return &out, c.do(ctx, http.MethodPost, c.orgPath("checks"), body, &out)
}

// GetCheck reads a check by uid or slug.
func (c *Client) GetCheck(ctx context.Context, uidOrSlug string) (*Check, error) {
	var out Check

	return &out, c.do(ctx, http.MethodGet, c.orgPath("checks", uidOrSlug), nil, &out)
}

// UpdateCheck patches a check.
func (c *Client) UpdateCheck(ctx context.Context, uid string, body Payload) (*Check, error) {
	var out Check

	return &out, c.do(ctx, http.MethodPatch, c.orgPath("checks", uid), body, &out)
}

// DeleteCheck deletes a check.
func (c *Client) DeleteCheck(ctx context.Context, uid string) error {
	return c.do(ctx, http.MethodDelete, c.orgPath("checks", uid), nil, nil)
}

// Channel is a notification channel.
type Channel struct {
	UID                 string         `json:"uid"`
	Type                string         `json:"type"`
	Name                string         `json:"name"`
	Slug                string         `json:"slug"`
	Enabled             bool           `json:"enabled"`
	IsDefault           bool           `json:"isDefault"`
	Settings            map[string]any `json:"settings"`
	SettingsPrivateKeys []string       `json:"settingsPrivateKeys"`
}

// CreateChannel creates a channel.
func (c *Client) CreateChannel(ctx context.Context, body Payload) (*Channel, error) {
	var out Channel

	return &out, c.do(ctx, http.MethodPost, c.orgPath("channels"), body, &out)
}

// GetChannel reads a channel by uid (settings included).
func (c *Client) GetChannel(ctx context.Context, uid string) (*Channel, error) {
	var out Channel

	return &out, c.do(ctx, http.MethodGet, c.orgPath("channels", uid), nil, &out)
}

// ListChannels lists the channels of the organization (settings omitted).
func (c *Client) ListChannels(ctx context.Context) ([]Channel, error) {
	var out listEnvelope[Channel]

	return out.Data, c.do(ctx, http.MethodGet, c.orgPath("channels"), nil, &out)
}

// UpdateChannel patches a channel.
func (c *Client) UpdateChannel(ctx context.Context, uid string, body Payload) (*Channel, error) {
	var out Channel

	return &out, c.do(ctx, http.MethodPatch, c.orgPath("channels", uid), body, &out)
}

// DeleteChannel deletes a channel.
func (c *Client) DeleteChannel(ctx context.Context, uid string) error {
	return c.do(ctx, http.MethodDelete, c.orgPath("channels", uid), nil, nil)
}

// StatusPage is the header of a status page (sections are separate).
type StatusPage struct {
	UID                     string  `json:"uid"`
	Name                    string  `json:"name"`
	Slug                    string  `json:"slug"`
	Description             string  `json:"description"`
	Visibility              string  `json:"visibility"`
	HasPassword             bool    `json:"hasPassword"`
	HideBranding            bool    `json:"hideBranding"`
	IsDefault               bool    `json:"isDefault"`
	Enabled                 bool    `json:"enabled"`
	ShowAvailability        bool    `json:"showAvailability"`
	ShowResponseTime        bool    `json:"showResponseTime"`
	HistoryPeriod           string  `json:"historyPeriod"`
	Language                string  `json:"language"`
	AutoPublish             bool    `json:"autoPublish"`
	AutoPublishDelaySeconds int64   `json:"autoPublishDelaySeconds"`
	PublishDegraded         bool    `json:"publishDegraded"`
	AutoResolve             string  `json:"autoResolve"`
	CustomCSS               *string `json:"customCss"`
	CustomDomain            *string `json:"customDomain"`
}

// CreateStatusPage creates a status page.
func (c *Client) CreateStatusPage(ctx context.Context, body Payload) (*StatusPage, error) {
	var out StatusPage

	return &out, c.do(ctx, http.MethodPost, c.orgPath("status-pages"), body, &out)
}

// GetStatusPage reads a status page by uid or slug.
func (c *Client) GetStatusPage(ctx context.Context, uidOrSlug string) (*StatusPage, error) {
	var out StatusPage

	return &out, c.do(ctx, http.MethodGet, c.orgPath("status-pages", uidOrSlug), nil, &out)
}

// UpdateStatusPage patches a status page.
func (c *Client) UpdateStatusPage(ctx context.Context, uid string, body Payload) (*StatusPage, error) {
	var out StatusPage

	return &out, c.do(ctx, http.MethodPatch, c.orgPath("status-pages", uid), body, &out)
}

// DeleteStatusPage deletes a status page.
func (c *Client) DeleteStatusPage(ctx context.Context, uid string) error {
	return c.do(ctx, http.MethodDelete, c.orgPath("status-pages", uid), nil, nil)
}

// SectionSelector is the dynamic-membership rule of a section.
type SectionSelector struct {
	All           bool              `json:"all,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	CheckGroupUID string            `json:"checkGroupUid,omitempty"`
}

// StatusPageSection is a section of a status page.
type StatusPageSection struct {
	UID      string           `json:"uid"`
	Name     string           `json:"name"`
	Slug     string           `json:"slug"`
	Position int64            `json:"position"`
	Selector *SectionSelector `json:"selector"`
}

// ListStatusPageSections lists the sections of a page.
func (c *Client) ListStatusPageSections(ctx context.Context, pageUID string) ([]StatusPageSection, error) {
	var out listEnvelope[StatusPageSection]

	return out.Data, c.do(ctx, http.MethodGet, c.orgPath("status-pages", pageUID, "sections"), nil, &out)
}

// CreateStatusPageSection creates a section.
func (c *Client) CreateStatusPageSection(ctx context.Context, pageUID string, body Payload) (*StatusPageSection, error) {
	var out StatusPageSection

	return &out, c.do(ctx, http.MethodPost, c.orgPath("status-pages", pageUID, "sections"), body, &out)
}

// GetStatusPageSection reads a section.
func (c *Client) GetStatusPageSection(ctx context.Context, pageUID, uid string) (*StatusPageSection, error) {
	var out StatusPageSection

	return &out, c.do(ctx, http.MethodGet, c.orgPath("status-pages", pageUID, "sections", uid), nil, &out)
}

// UpdateStatusPageSection patches a section.
func (c *Client) UpdateStatusPageSection(
	ctx context.Context, pageUID, uid string, body Payload,
) (*StatusPageSection, error) {
	var out StatusPageSection

	return &out, c.do(ctx, http.MethodPatch, c.orgPath("status-pages", pageUID, "sections", uid), body, &out)
}

// DeleteStatusPageSection deletes a section.
func (c *Client) DeleteStatusPageSection(ctx context.Context, pageUID, uid string) error {
	return c.do(ctx, http.MethodDelete, c.orgPath("status-pages", pageUID, "sections", uid), nil, nil)
}

// StatusPageResource is one component shown in a section.
type StatusPageResource struct {
	UID               string `json:"uid"`
	CheckUID          string `json:"checkUid"`
	CheckGroupUID     string `json:"checkGroupUid"`
	PublicName        string `json:"publicName"`
	Explanation       string `json:"explanation"`
	ManagedBySelector bool   `json:"managedBySelector"`
	Position          int64  `json:"position"`
}

// ListStatusPageResources lists the resources of a section.
func (c *Client) ListStatusPageResources(ctx context.Context, pageUID, sectionUID string) ([]StatusPageResource, error) {
	var out listEnvelope[StatusPageResource]

	path := c.orgPath("status-pages", pageUID, "sections", sectionUID, "resources")

	return out.Data, c.do(ctx, http.MethodGet, path, nil, &out)
}

// CreateStatusPageResource adds a resource to a section.
func (c *Client) CreateStatusPageResource(
	ctx context.Context, pageUID, sectionUID string, body Payload,
) (*StatusPageResource, error) {
	var out StatusPageResource

	path := c.orgPath("status-pages", pageUID, "sections", sectionUID, "resources")

	return &out, c.do(ctx, http.MethodPost, path, body, &out)
}

// UpdateStatusPageResource patches a resource of a section.
func (c *Client) UpdateStatusPageResource(ctx context.Context, pageUID, sectionUID, uid string, body Payload) error {
	path := c.orgPath("status-pages", pageUID, "sections", sectionUID, "resources", uid)

	return c.do(ctx, http.MethodPatch, path, body, nil)
}

// DeleteStatusPageResource removes a resource from a section.
func (c *Client) DeleteStatusPageResource(ctx context.Context, pageUID, sectionUID, uid string) error {
	path := c.orgPath("status-pages", pageUID, "sections", sectionUID, "resources", uid)

	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// ReorderStatusPageResources sets the order of the resources of a section.
func (c *Client) ReorderStatusPageResources(ctx context.Context, pageUID, sectionUID string, uids []string) error {
	path := c.orgPath("status-pages", pageUID, "sections", sectionUID, "resources", "reorder")

	return c.do(ctx, http.MethodPost, path, Payload{"uids": uids}, nil)
}

// MaintenanceWindow is a planned maintenance.
type MaintenanceWindow struct {
	UID           string `json:"uid"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	StartAt       string `json:"startAt"`
	EndAt         string `json:"endAt"`
	Recurrence    string `json:"recurrence"`
	RecurrenceEnd string `json:"recurrenceEnd"`
}

// MaintenanceWindowCheck links a window to a check or a check group.
type MaintenanceWindowCheck struct {
	UID           string `json:"uid"`
	CheckUID      string `json:"checkUid"`
	CheckGroupUID string `json:"checkGroupUid"`
}

// CreateMaintenanceWindow creates a maintenance window.
func (c *Client) CreateMaintenanceWindow(ctx context.Context, body Payload) (*MaintenanceWindow, error) {
	var out MaintenanceWindow

	return &out, c.do(ctx, http.MethodPost, c.orgPath("maintenance-windows"), body, &out)
}

// GetMaintenanceWindow reads a maintenance window.
func (c *Client) GetMaintenanceWindow(ctx context.Context, uid string) (*MaintenanceWindow, error) {
	var out MaintenanceWindow

	return &out, c.do(ctx, http.MethodGet, c.orgPath("maintenance-windows", uid), nil, &out)
}

// UpdateMaintenanceWindow patches a maintenance window.
func (c *Client) UpdateMaintenanceWindow(ctx context.Context, uid string, body Payload) (*MaintenanceWindow, error) {
	var out MaintenanceWindow

	return &out, c.do(ctx, http.MethodPatch, c.orgPath("maintenance-windows", uid), body, &out)
}

// DeleteMaintenanceWindow deletes a maintenance window.
func (c *Client) DeleteMaintenanceWindow(ctx context.Context, uid string) error {
	return c.do(ctx, http.MethodDelete, c.orgPath("maintenance-windows", uid), nil, nil)
}

// ListMaintenanceWindowChecks lists the checks and check groups of a window.
func (c *Client) ListMaintenanceWindowChecks(ctx context.Context, uid string) ([]MaintenanceWindowCheck, error) {
	var out listEnvelope[MaintenanceWindowCheck]

	return out.Data, c.do(ctx, http.MethodGet, c.orgPath("maintenance-windows", uid, "checks"), nil, &out)
}

// SetMaintenanceWindowChecks replaces the checks and check groups of a window.
func (c *Client) SetMaintenanceWindowChecks(ctx context.Context, uid string, checkUIDs, groupUIDs []string) error {
	body := Payload{"checkUids": checkUIDs, "checkGroupUids": groupUIDs}

	return c.do(ctx, http.MethodPut, c.orgPath("maintenance-windows", uid, "checks"), body, nil)
}
