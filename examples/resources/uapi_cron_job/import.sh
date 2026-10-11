# Import by id (the sha256 prefix of "<schedule> <command>" that uapi assigns).
# List the ids with GET /system/cron_jobs. Import recovers schedule and command.
terraform import uapi_cron_job.nightly_reboot 0a1b2c3d4e5f
