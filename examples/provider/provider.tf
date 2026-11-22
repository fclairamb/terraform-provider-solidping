terraform {
  required_providers {
    solidping = {
      source = "fclairamb/solidping"
    }
  }
}

# The three settings can also come from the environment:
# SOLIDPING_ENDPOINT, SOLIDPING_ORG and SOLIDPING_TOKEN.
provider "solidping" {
  endpoint = "https://solidping.acme.com"
  org      = "acme"
  token    = var.solidping_token
}

variable "solidping_token" {
  type      = string
  sensitive = true
}
