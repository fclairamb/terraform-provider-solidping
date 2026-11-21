# Slack channels are created by installing the Slack app: look them up by name.
data "solidping_channel" "slack_ops" {
  name = "#ops-alerts"
}
