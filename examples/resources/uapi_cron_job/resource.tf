resource "uapi_cron_job" "nightly_reboot" {
  schedule = "30 4 * * sun"
  command  = "/sbin/reboot"
}
