# A hand-curated section: the components are listed in display order.
resource "solidping_status_page_section" "core" {
  status_page_uid = solidping_status_page.public.id
  name            = "Core services"

  resources = [
    {
      check_uid   = solidping_check.website.id
      public_name = "Website"
    },
    {
      check_uid   = solidping_check.database.id
      public_name = "Database"
      explanation = "Primary PostgreSQL cluster"
    },
  ]
}

# A dynamic section: every check labelled team=web, kept up to date by SolidPing.
resource "solidping_status_page_section" "web" {
  status_page_uid = solidping_status_page.public.id
  name            = "Web"

  selector = {
    labels = { team = "web" }
  }
}
