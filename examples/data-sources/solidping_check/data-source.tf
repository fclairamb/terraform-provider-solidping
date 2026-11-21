data "solidping_check" "website" {
  slug = "website"
}

output "website_status" {
  value = data.solidping_check.website.status
}
