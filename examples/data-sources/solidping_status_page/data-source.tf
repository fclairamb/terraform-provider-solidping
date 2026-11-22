data "solidping_status_page" "public" {
  slug = "acme"
}

resource "solidping_status_page_section" "extra" {
  status_page_uid = data.solidping_status_page.public.id
  name            = "Extra"
}
