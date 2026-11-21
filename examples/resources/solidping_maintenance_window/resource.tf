# Every Sunday night, until the end of the year.
resource "solidping_maintenance_window" "weekly_patching" {
  title          = "Weekly patching"
  description    = "Operating system updates"
  start_at       = "2027-01-03T02:00:00Z"
  end_at         = "2027-01-03T04:00:00Z"
  recurrence     = "weekly"
  recurrence_end = "2027-12-31T00:00:00Z"

  check_uids = [
    solidping_check.website.id,
    solidping_check.database.id,
  ]
}
