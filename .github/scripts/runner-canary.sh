#!/usr/bin/env bash
# Rolls the runner Lambdas out as a CodeDeploy canary (ADR-0020). Per language: the
# BeforeAllowTraffic hook judges an accepted solution with the new version, then the
# live alias sends it 10 % of the invocations for 5 minutes, then all of them. A failed
# hook, or a function error behind the alias (alarm), rolls the alias back.
#   DEPLOY='<terraform output -json deploy>' runner-canary.sh
set -euo pipefail

canary=$(jq -c .runner_canary <<<"${DEPLOY}")
if [ "${canary}" = "null" ]; then
  echo "No runner canary in this environment: Terraform moved the live aliases."
  exit 0
fi
app=$(jq -r .application <<<"${canary}")
hook=$(jq -r .hook <<<"${canary}")

ids=()
for lang in $(jq -r '.runners | keys[]' <<<"${canary}"); do
  fn=$(jq -r --arg l "${lang}" '.runners[$l].function' <<<"${canary}")
  group=$(jq -r --arg l "${lang}" '.runners[$l].deployment_group' <<<"${canary}")
  target=$(jq -r --arg l "${lang}" '.runners[$l].version' <<<"${canary}")
  current=$(aws lambda get-alias --function-name "${fn}" --name live --query FunctionVersion --output text)
  if [ "${current}" = "${target}" ]; then
    echo "${fn}: live already points to version ${target}"
    continue
  fi
  # Written by hand: jq would print the AppSpec version 0.0 as 0.
  appspec=$(printf '{"version":0.0,"Resources":[{"runner":{"Type":"AWS::Lambda::Function","Properties":{"Name":"%s","Alias":"live","CurrentVersion":"%s","TargetVersion":"%s"}}}],"Hooks":[{"BeforeAllowTraffic":"%s"}]}' \
    "${fn}" "${current}" "${target}" "${hook}")
  revision=$(jq -cn --arg content "${appspec}" '{revisionType: "AppSpecContent", appSpecContent: {content: $content}}')
  id=$(aws deploy create-deployment --application-name "${app}" --deployment-group-name "${group}" \
    --revision "${revision}" --description "${GITHUB_SHA:-manual}: ${fn} ${current} -> ${target}" \
    --query deploymentId --output text)
  echo "${fn}: canary ${current} -> ${target} (${id})"
  ids+=("${id}")
done

failed=0
for id in "${ids[@]}"; do
  if aws deploy wait deployment-successful --deployment-id "${id}"; then
    echo "${id}: succeeded"
  else
    reason=$(aws deploy get-deployment --deployment-id "${id}" \
      --query 'deploymentInfo.[status, errorInformation.message]' --output text)
    echo "::error::Runner canary ${id} did not succeed (${reason}); the live alias stays on the old version"
    failed=1
  fi
done
exit "${failed}"
