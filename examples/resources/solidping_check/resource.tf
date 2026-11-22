# An HTTP check, every minute, with basic authentication.
resource "solidping_check" "website" {
  name   = "Website"
  type   = "http"
  period = "00:01:00"

  config = jsonencode({
    url     = "https://www.acme.com/health"
    headers = { "Accept" = "application/json" }
  })

  # Secrets are stored encrypted by SolidPing and never read back.
  config_sensitive = jsonencode({
    basicAuth = "monitor:${var.monitor_password}"
  })

  labels = {
    env  = "prod"
    team = "web"
  }
}

# A TCP check pinned to two regions (the region slugs are those of your SolidPing server).
resource "solidping_check" "database" {
  name    = "Database port"
  type    = "tcp"
  regions = ["eu-west", "us-east"]

  config = jsonencode({
    host = "db.acme.com"
    port = 5432
  })
}

variable "monitor_password" {
  type      = string
  sensitive = true
}
