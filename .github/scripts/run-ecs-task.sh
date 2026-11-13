#!/usr/bin/env bash
# Runs a one-off ECS task (migrate, seed) and fails unless its container exits with 0.
#   DEPLOY='<terraform output -json jobs>' run-ecs-task.sh migrate
set -euo pipefail

job="$1"
cluster=$(jq -r .cluster <<<"${DEPLOY}")
taskdef=$(jq -r --arg job "${job}" '.job_task_definitions[$job]' <<<"${DEPLOY}")
subnets=$(jq -r '.task_subnet_ids | join(",")' <<<"${DEPLOY}")
sg=$(jq -r .job_security_group_id <<<"${DEPLOY}")

# Public subnets without NAT (ADR-0005): the task needs a public IP to pull from ECR.
task=$(aws ecs run-task --cluster "${cluster}" --task-definition "${taskdef}" --launch-type FARGATE \
  --network-configuration "awsvpcConfiguration={subnets=[${subnets}],securityGroups=[${sg}],assignPublicIp=ENABLED}" \
  --query 'tasks[0].taskArn' --output text)
if [ -z "${task}" ] || [ "${task}" = "None" ]; then
  echo "::error::ECS did not start the ${job} task"
  exit 1
fi
echo "${job} task: ${task}"

aws ecs wait tasks-stopped --cluster "${cluster}" --tasks "${task}"
read -r code reason < <(aws ecs describe-tasks --cluster "${cluster}" --tasks "${task}" \
  --query 'tasks[0].[containers[0].exitCode, stoppedReason]' --output text)
if [ "${code}" != "0" ]; then
  echo "::error::${job} failed (exit code ${code}: ${reason}). Logs: CloudWatch /ecs/${cluster}/${job}"
  exit 1
fi
echo "${job} finished"
