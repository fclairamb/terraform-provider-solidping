# A webhook. SolidPing signs every delivery; custom headers go in `settings.headers`.
resource "solidping_channel" "ops_webhook" {
  type = "webhook"
  name = "Ops webhook"

  settings = jsonencode({
    url     = "https://hooks.acme.com/solidping"
    headers = { "X-Source" = "solidping" }
  })
}

# An ntfy topic, with its access token kept out of the readable settings.
resource "solidping_channel" "ntfy" {
  type       = "ntfy"
  name       = "ntfy alerts"
  is_default = true

  settings = jsonencode({
    server_url = "https://ntfy.acme.com"
    topic      = "solidping-alerts"
  })

  settings_sensitive = jsonencode({
    auth_token = var.ntfy_token
  })
}

variable "ntfy_token" {
  type      = string
  sensitive = true
}
