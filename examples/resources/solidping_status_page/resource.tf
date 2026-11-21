resource "solidping_status_page" "public" {
  name           = "Acme status"
  slug           = "acme"
  description    = "Live status of the Acme services"
  history_period = "30d"
  custom_domain  = "status.acme.com"
}

# A page shared with a password.
resource "solidping_status_page" "partners" {
  name       = "Acme partner APIs"
  visibility = "password"
  password   = var.status_page_password
}

variable "status_page_password" {
  type      = string
  sensitive = true
}
