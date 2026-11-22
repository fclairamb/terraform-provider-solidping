package provider

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fclairamb/terraform-provider-solidping/internal/client"
)

// importID builds the `org/uid` import address of a resource in the state.
func importID(name string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return "", fmt.Errorf("resource %s not in state", name)
		}

		return os.Getenv(envOrg) + "/" + rs.Primary.ID, nil
	}
}

// checkDestroyed fails when a resource of the given type still exists.
func checkDestroyed(typeName string, get func(ctx context.Context, api *client.Client, rs *terraform.ResourceState) error) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		api := testAccClient()

		for name, rs := range s.RootModule().Resources {
			if rs.Type != typeName {
				continue
			}

			err := get(context.Background(), api, rs)
			if err == nil {
				return fmt.Errorf("%s still exists", name)
			}

			if !client.IsNotFound(err) {
				return fmt.Errorf("%s: %w", name, err)
			}
		}

		return nil
	}
}

func TestAccCheckResource(t *testing.T) {
	suffix := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	name := "solidping_check.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: checkDestroyed("solidping_check", func(ctx context.Context, api *client.Client, rs *terraform.ResourceState) error {
			_, err := api.GetCheck(ctx, rs.Primary.ID)

			return err
		}),
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig + fmt.Sprintf(`
resource "solidping_check" "test" {
  name        = "TF acc %[1]s"
  slug        = "tf-acc-%[1]s"
  description = "Created by the acceptance tests"
  type        = "http"
  period      = "00:05:00"

  config = jsonencode({
    url     = "https://acme.com/%[1]s"
    headers = { "X-Probe" = "terraform" }
  })

  config_sensitive = jsonencode({
    basicAuth = "alice:s3cret-%[1]s"
  })

  labels = {
    env  = "test"
    team = "platform"
  }
}
`, suffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr(name, "id", regexp.MustCompile(`^[0-9a-f-]{36}$`)),
					resource.TestCheckResourceAttr(name, "name", "TF acc "+suffix),
					resource.TestCheckResourceAttr(name, "slug", "tf-acc-"+suffix),
					resource.TestCheckResourceAttr(name, "enabled", "true"),
					resource.TestCheckResourceAttr(name, "period", "00:05:00"),
					resource.TestCheckResourceAttr(name, "labels.env", "test"),
					resource.TestCheckResourceAttrSet(name, "placement"),
				),
			},
			{
				ResourceName:            name,
				ImportState:             true,
				ImportStateIdFunc:       importID(name),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"config_sensitive"},
			},
			{
				// Rename, disable, drop the description, the labels and the secrets.
				Config: testAccProviderConfig + fmt.Sprintf(`
resource "solidping_check" "test" {
  name    = "TF acc %[1]s renamed"
  slug    = "tf-acc-%[1]s"
  type    = "http"
  period  = "00:10:00"
  enabled = false

  config = jsonencode({
    url = "https://acme.com/%[1]s/v2"
  })
}
`, suffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "name", "TF acc "+suffix+" renamed"),
					resource.TestCheckResourceAttr(name, "enabled", "false"),
					resource.TestCheckResourceAttr(name, "period", "00:10:00"),
					resource.TestCheckNoResourceAttr(name, "description"),
					resource.TestCheckNoResourceAttr(name, "labels.%"),
					func(s *terraform.State) error {
						check, err := testAccClient().GetCheck(context.Background(), s.RootModule().Resources[name].Primary.ID)
						if err != nil {
							return err
						}

						if len(check.Labels) != 0 || check.Description != nil && *check.Description != "" {
							return fmt.Errorf("labels/description not cleared on the server: %+v", check)
						}

						if len(check.ConfigPrivateKeys) != 0 {
							return fmt.Errorf("secrets not cleared on the server: %v", check.ConfigPrivateKeys)
						}

						if _, ok := check.Config["headers"]; ok {
							return fmt.Errorf("headers not removed from the server config: %v", check.Config)
						}

						return nil
					},
				),
			},
			{
				// Minimal check: the server names it.
				Config: testAccProviderConfig + fmt.Sprintf(`
resource "solidping_check" "minimal" {
  type   = "tcp"
  config = jsonencode({ host = "%[1]s.acme.com", port = 443 })
}

data "solidping_check" "by_slug" {
  slug = solidping_check.minimal.slug
}

data "solidping_check" "by_uid" {
  uid = solidping_check.minimal.id
}
`, suffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("solidping_check.minimal", "name"),
					resource.TestCheckResourceAttrSet("solidping_check.minimal", "slug"),
					resource.TestCheckResourceAttrPair("data.solidping_check.by_slug", "id", "solidping_check.minimal", "id"),
					resource.TestCheckResourceAttrPair("data.solidping_check.by_uid", "slug", "solidping_check.minimal", "slug"),
					resource.TestCheckResourceAttr("data.solidping_check.by_uid", "type", "tcp"),
				),
			},
		},
	})
}

func TestAccCheckResource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig + `
resource "solidping_check" "test" {
  type   = "http"
  period = "60s"
  config = jsonencode({ url = "https://acme.com" })
}
`,
				ExpectError: regexp.MustCompile(`HH:MM:SS`),
			},
			{
				// The API error shape reaches the diagnostics.
				Config: testAccProviderConfig + `
resource "solidping_check" "test" {
  type   = "http"
  config = jsonencode({})
}
`,
				ExpectError: regexp.MustCompile(`(?s)VALIDATION_ERROR|url`),
			},
		},
	})
}

func TestAccChannelResource(t *testing.T) {
	suffix := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	name := "solidping_channel.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: checkDestroyed("solidping_channel", func(ctx context.Context, api *client.Client, rs *terraform.ResourceState) error {
			_, err := api.GetChannel(ctx, rs.Primary.ID)

			return err
		}),
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig + fmt.Sprintf(`
resource "solidping_channel" "test" {
  type = "webhook"
  name = "TF acc %[1]s"

  settings = jsonencode({
    url     = "https://acme.com/hooks/%[1]s"
    headers = { "X-Source" = "solidping" }
  })
}

data "solidping_channel" "by_name" {
  name = solidping_channel.test.name
}
`, suffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr(name, "id", regexp.MustCompile(`^[0-9a-f-]{36}$`)),
					resource.TestCheckResourceAttr(name, "enabled", "true"),
					resource.TestCheckResourceAttr(name, "is_default", "false"),
					resource.TestCheckResourceAttrPair("data.solidping_channel.by_name", "id", name, "id"),
					resource.TestCheckResourceAttr("data.solidping_channel.by_name", "type", "webhook"),
				),
			},
			{
				ResourceName:      name,
				ImportState:       true,
				ImportStateIdFunc: importID(name),
				ImportStateVerify: true,
			},
			{
				Config: testAccProviderConfig + fmt.Sprintf(`
resource "solidping_channel" "test" {
  type    = "webhook"
  name    = "TF acc %[1]s renamed"
  enabled = false

  settings = jsonencode({
    url = "https://acme.com/hooks/%[1]s/v2"
  })
}
`, suffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "name", "TF acc "+suffix+" renamed"),
					resource.TestCheckResourceAttr(name, "enabled", "false"),
					func(s *terraform.State) error {
						channel, err := testAccClient().GetChannel(context.Background(), s.RootModule().Resources[name].Primary.ID)
						if err != nil {
							return err
						}

						if _, ok := channel.Settings["headers"]; ok {
							return fmt.Errorf("headers not removed from the server settings: %v", channel.Settings)
						}

						return nil
					},
				),
			},
		},
	})
}

func TestAccChannelResource_slackRefused(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig + `
resource "solidping_channel" "slack" {
  type = "slack"
  name = "nope"
}
`,
				ExpectError: regexp.MustCompile(`slack`),
			},
		},
	})
}

func TestAccStatusPageResource(t *testing.T) {
	suffix := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	page := "solidping_status_page.test"
	section := "solidping_status_page_section.api"

	checks := fmt.Sprintf(`
resource "solidping_check" "a" {
  type   = "http"
  config = jsonencode({ url = "https://acme.com/a-%[1]s" })
}

resource "solidping_check" "b" {
  type   = "http"
  labels = { tier = "tf-%[1]s" }
  config = jsonencode({ url = "https://acme.com/b-%[1]s" })
}
`, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: checkDestroyed("solidping_status_page", func(ctx context.Context, api *client.Client, rs *terraform.ResourceState) error {
			_, err := api.GetStatusPage(ctx, rs.Primary.ID)

			return err
		}),
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig + checks + fmt.Sprintf(`
resource "solidping_status_page" "test" {
  name        = "TF acc %[1]s"
  slug        = "tf-acc-%[1]s"
  description = "Acme services"
  visibility  = "password"
  password    = "open-sesame"
}

resource "solidping_status_page_section" "api" {
  status_page_uid = solidping_status_page.test.id
  name            = "API"

  resources = [
    { check_uid = solidping_check.a.id, public_name = "Public API" },
    { check_uid = solidping_check.b.id, explanation = "Background jobs" },
  ]
}

data "solidping_status_page" "by_slug" {
  slug = solidping_status_page.test.slug
}
`, suffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(page, "enabled", "true"),
					resource.TestCheckResourceAttr(page, "visibility", "password"),
					resource.TestCheckResourceAttr(page, "history_period", "90d"),
					resource.TestCheckResourceAttr(section, "resources.#", "2"),
					resource.TestCheckResourceAttr(section, "resources.0.public_name", "Public API"),
					resource.TestCheckResourceAttrPair("data.solidping_status_page.by_slug", "id", page, "id"),
					func(s *terraform.State) error {
						// Only the declared section is left: the seeded one is gone.
						sections, err := testAccClient().ListStatusPageSections(
							context.Background(), s.RootModule().Resources[page].Primary.ID)
						if err != nil {
							return err
						}

						if len(sections) != 1 || sections[0].Name != "API" {
							return fmt.Errorf("unexpected sections: %+v", sections)
						}

						return nil
					},
				),
			},
			{
				ResourceName:            page,
				ImportState:             true,
				ImportStateIdFunc:       importID(page),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password"},
			},
			{
				ResourceName: section,
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources[section]

					return os.Getenv(envOrg) + "/" + rs.Primary.Attributes["status_page_uid"] + "/" + rs.Primary.ID, nil
				},
				ImportStateVerify: true,
			},
			{
				// Reorder the resources, rename one, drop the other's text; update the page.
				Config: testAccProviderConfig + checks + fmt.Sprintf(`
resource "solidping_status_page" "test" {
  name                       = "TF acc %[1]s renamed"
  slug                       = "tf-acc-%[1]s"
  visibility                 = "public"
  enabled                    = false
  show_response_time         = false
  history_period             = "30d"
  auto_publish_delay_seconds = 120
  custom_css                 = ":root { --brand: #0055ff; }"
}

resource "solidping_status_page_section" "api" {
  status_page_uid = solidping_status_page.test.id
  name            = "API and jobs"

  resources = [
    { check_uid = solidping_check.b.id },
    { check_uid = solidping_check.a.id, public_name = "Public API v2" },
  ]
}
`, suffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(page, "name", "TF acc "+suffix+" renamed"),
					resource.TestCheckResourceAttr(page, "enabled", "false"),
					resource.TestCheckResourceAttr(page, "visibility", "public"),
					resource.TestCheckResourceAttr(page, "show_response_time", "false"),
					resource.TestCheckResourceAttr(page, "history_period", "30d"),
					resource.TestCheckNoResourceAttr(page, "description"),
					resource.TestCheckResourceAttr(section, "name", "API and jobs"),
					resource.TestCheckResourceAttrPair(section, "resources.0.check_uid", "solidping_check.b", "id"),
					resource.TestCheckResourceAttr(section, "resources.1.public_name", "Public API v2"),
				),
			},
			{
				// Switch the section to a selector, drop the stylesheet, add a second section.
				Config: testAccProviderConfig + checks + fmt.Sprintf(`
resource "solidping_status_page" "test" {
  name = "TF acc %[1]s renamed"
  slug = "tf-acc-%[1]s"
}

resource "solidping_status_page_section" "api" {
  status_page_uid = solidping_status_page.test.id
  name            = "API and jobs"

  selector = {
    labels = { tier = "tf-%[1]s" }
  }
}

resource "solidping_status_page_section" "other" {
  status_page_uid = solidping_status_page.test.id
  name            = "Other"
  resources       = [{ check_uid = solidping_check.a.id }]
}
`, suffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(page, "enabled", "true"),
					resource.TestCheckNoResourceAttr(page, "custom_css"),
					resource.TestCheckResourceAttr(section, "selector.labels.tier", "tf-"+suffix),
					resource.TestCheckNoResourceAttr(section, "resources.#"),
					resource.TestCheckResourceAttr("solidping_status_page_section.other", "resources.#", "1"),
				),
			},
			{
				// Back to a hand-curated, empty section.
				Config: testAccProviderConfig + checks + fmt.Sprintf(`
resource "solidping_status_page" "test" {
  name = "TF acc %[1]s renamed"
  slug = "tf-acc-%[1]s"
}

resource "solidping_status_page_section" "api" {
  status_page_uid = solidping_status_page.test.id
  name            = "API and jobs"
}
`, suffix),
				Check: resource.TestCheckNoResourceAttr(section, "selector.%"),
			},
		},
	})
}

func TestAccMaintenanceWindowResource(t *testing.T) {
	suffix := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	name := "solidping_maintenance_window.test"

	checks := fmt.Sprintf(`
resource "solidping_check" "a" {
  type   = "http"
  config = jsonencode({ url = "https://acme.com/mw-a-%[1]s" })
}

resource "solidping_check" "b" {
  type   = "http"
  config = jsonencode({ url = "https://acme.com/mw-b-%[1]s" })
}
`, suffix)

	steps := []resource.TestStep{
		{
			// A timestamp with an offset: the server answers in UTC.
			Config: testAccProviderConfig + checks + fmt.Sprintf(`
resource "solidping_maintenance_window" "test" {
  title          = "TF acc %[1]s"
  description    = "Database upgrade"
  start_at       = "2031-03-01T22:00:00+02:00"
  end_at         = "2031-03-01T23:30:00+02:00"
  recurrence     = "weekly"
  recurrence_end = "2031-09-01T00:00:00Z"
  check_uids     = [solidping_check.a.id, solidping_check.b.id]
}
`, suffix),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestMatchResourceAttr(name, "id", regexp.MustCompile(`^[0-9a-f-]{36}$`)),
				resource.TestCheckResourceAttr(name, "start_at", "2031-03-01T22:00:00+02:00"),
				resource.TestCheckResourceAttr(name, "recurrence", "weekly"),
				resource.TestCheckResourceAttr(name, "check_uids.#", "2"),
			),
		},
		{
			ResourceName:      name,
			ImportState:       true,
			ImportStateIdFunc: importID(name),
			ImportStateVerify: true,
			// Same instants, written in UTC by the server.
			ImportStateVerifyIgnore: []string{"start_at", "end_at"},
		},
		{
			Config: testAccProviderConfig + checks + fmt.Sprintf(`
resource "solidping_maintenance_window" "test" {
  title          = "TF acc %[1]s moved"
  description    = "Database upgrade, second attempt"
  start_at       = "2031-03-02T22:00:00Z"
  end_at         = "2031-03-02T23:00:00Z"
  recurrence     = "monthly"
  recurrence_end = "2031-12-01T00:00:00Z"
  check_uids     = [solidping_check.b.id]
}
`, suffix),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(name, "title", "TF acc "+suffix+" moved"),
				resource.TestCheckResourceAttr(name, "recurrence", "monthly"),
				resource.TestCheckResourceAttr(name, "check_uids.#", "1"),
				resource.TestCheckTypeSetElemAttrPair(name, "check_uids.*", "solidping_check.b", "id"),
			),
		},
	}

	steps = append(steps, resource.TestStep{
		// Clearing description and recurrence_end needs a server that reads an explicit null.
		SkipFunc: func() (bool, error) { return !serverAtLeast(0, 53), nil },
		Config: testAccProviderConfig + checks + fmt.Sprintf(`
resource "solidping_maintenance_window" "test" {
  title    = "TF acc %[1]s moved"
  start_at = "2031-03-02T22:00:00Z"
  end_at   = "2031-03-02T23:00:00Z"
}
`, suffix),
		Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr(name, "recurrence", "none"),
			resource.TestCheckNoResourceAttr(name, "description"),
			resource.TestCheckNoResourceAttr(name, "recurrence_end"),
			resource.TestCheckNoResourceAttr(name, "check_uids.#"),
			func(s *terraform.State) error {
				uid := s.RootModule().Resources[name].Primary.ID

				window, err := testAccClient().GetMaintenanceWindow(context.Background(), uid)
				if err != nil {
					return err
				}

				links, err := testAccClient().ListMaintenanceWindowChecks(context.Background(), uid)
				if err != nil {
					return err
				}

				if window.Description != "" || window.RecurrenceEnd != "" || len(links) != 0 {
					return fmt.Errorf("fields not cleared on the server: %+v, %d checks", window, len(links))
				}

				return nil
			},
		),
	})

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: checkDestroyed("solidping_maintenance_window", func(ctx context.Context, api *client.Client, rs *terraform.ResourceState) error {
			_, err := api.GetMaintenanceWindow(ctx, rs.Primary.ID)

			return err
		}),
		Steps: steps,
	})
}

func TestAccResourceGoneIsRecreated(t *testing.T) {
	suffix := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	config := testAccProviderConfig + fmt.Sprintf(`
resource "solidping_check" "test" {
  type   = "http"
  config = jsonencode({ url = "https://acme.com/gone-%[1]s" })
}
`, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: func(s *terraform.State) error {
					// Deleted behind Terraform's back: the next refresh drops it from the state.
					return testAccClient().DeleteCheck(context.Background(), s.RootModule().Resources["solidping_check.test"].Primary.ID)
				},
				ExpectNonEmptyPlan: true,
			},
			{
				Config: config,
				Check:  resource.TestCheckResourceAttrSet("solidping_check.test", "id"),
			},
		},
	})
}
