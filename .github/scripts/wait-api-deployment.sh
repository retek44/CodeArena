#!/usr/bin/env bash
# Waits for the newest deployment of the API service (ECS blue/green, ADR-0020) and fails
# unless it succeeded. After a rollback the old version keeps serving; the pipeline must
# not report that as deployed.
#   DEPLOY='<terraform output -json deploy>' wait-api-deployment.sh
set -euo pipefail

cluster=$(jq -r .cluster <<<"${DEPLOY}")
service=$(jq -r .api_service <<<"${DEPLOY}")
# Green start + smoke test (the hook waits up to 10 minutes for a new CloudFront
# distribution) + bake time; ECS gives up on a stage after hours, the pipeline earlier.
deadline=$((SECONDS + ${TIMEOUT_SECONDS:-2700}))

while :; do
  newest=$(aws ecs list-service-deployments --cluster "${cluster}" --service "${service}" \
    --output json | jq -c '.serviceDeployments | max_by(.createdAt) // empty')
  status=NONE
  if [ -n "${newest}" ]; then
    status=$(jq -r .status <<<"${newest}")
  fi
  case "${status}" in
    SUCCESSFUL)
      echo "API deployment succeeded: $(jq -r .serviceDeploymentArn <<<"${newest}")"
      exit 0
      ;;
    ROLLBACK_SUCCESSFUL | ROLLBACK_FAILED | STOPPED)
      echo "::error::API deployment ${status}: $(jq -r '.statusReason // "no reason given"' <<<"${newest}")." \
        "Hook logs: CloudWatch /aws/lambda/${cluster}-deploy-hook"
      exit 1
      ;;
  esac
  if [ "${SECONDS}" -ge "${deadline}" ]; then
    echo "::error::API deployment still ${status} after ${TIMEOUT_SECONDS:-2700} s"
    exit 1
  fi
  echo "API deployment: ${status}"
  sleep 20
done
