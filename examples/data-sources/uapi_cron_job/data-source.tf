data "uapi_cron_job" "nightly_reboot" {
  id = "0a1b2c3d4e5f"
}

output "crond_running" {
  value = data.uapi_cron_job.nightly_reboot.runtime.crond_running
}
