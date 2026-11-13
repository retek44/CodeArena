output "valkey_url" {
  description = "Leaderboard cache URL (TLS) for VALKEY_URL."
  value       = "rediss://${aws_elasticache_serverless_cache.leaderboard.endpoint[0].address}:${aws_elasticache_serverless_cache.leaderboard.endpoint[0].port}"
}

output "state_machine_arn" {
  description = "Contest lifecycle state machine."
  value       = aws_sfn_state_machine.lifecycle.arn
}

output "schedule_group" {
  description = "Scheduler group of the contest schedules."
  value       = aws_scheduler_schedule_group.contests.name
}

output "scheduler_role_arn" {
  description = "Role of the contest schedules."
  value       = aws_iam_role.scheduler.arn
}
