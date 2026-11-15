#!/usr/bin/env bash
# Runs the sandbox hardening suite against the runner images (ADR-0003).
#   tests/sandbox-security/run.sh <tag>      images: codearena-runner-<language>:<tag>
#
# The runners run like Lambda (non-root, read-only root file system, writable /tmp only)
# on an internal Docker network: no route out and no DNS, like the isolated subnets on
# AWS. The test container joins that network after it installed pytest, and calls the
# Runtime Interface Emulator in each runner container.
set -euo pipefail

tag="${1:?image tag (codearena-runner-<language>:<tag>)}"
languages=(python cpp java javascript go)
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Git Bash on Windows: docker needs the Windows form of the path (pwd -W).
host_here="$(cd "${here}" && { pwd -W 2>/dev/null || pwd; })"
net=codearena-sandbox
suite=codearena-sandbox-suite
pytest_version=$(grep -E '^pytest==' "${here}/../../requirements-dev.txt" | cut -d= -f3)

cleanup() {
  for lang in "${languages[@]}"; do
    docker rm -f "codearena-sandbox-${lang}" >/dev/null 2>&1 || true
  done
  docker rm -f "${suite}" >/dev/null 2>&1 || true
  docker network rm "${net}" >/dev/null 2>&1 || true
}
trap cleanup EXIT
cleanup

docker network create --internal "${net}" >/dev/null
# RLIMIT_NPROC counts the tasks of a uid across the whole host, not per container. The
# images run as uid 993, like the local stack (docker compose) does, so each runner here
# gets a uid of its own; otherwise other runners would use up its process budget. On AWS
# every execution environment is its own microVM.
# Compiled programs (C++, Go) run from /tmp (Lambda allows it); Docker tmpfs is noexec by
# default, so every runner here gets exec, as in Lambda.
# Fake credentials in the environment, as Lambda sets them: the image entrypoint must
# drop them (programs/credentials.py).
fake_credentials=(-e AWS_ACCESS_KEY_ID=AKIAFAKESANDBOXSUITE -e AWS_SECRET_ACCESS_KEY=fake-secret -e AWS_SESSION_TOKEN=fake-token)
env_args=()
uid=20001
for lang in "${languages[@]}"; do
  docker run -d --name "codearena-sandbox-${lang}" --network "${net}" --read-only \
    --tmpfs /tmp:rw,exec,size=256m --memory 1g --user "${uid}:990" "${fake_credentials[@]}" \
    "codearena-runner-${lang}:${tag}" >/dev/null
  suffix=""
  if [ "${lang}" != python ]; then suffix="_${lang^^}"; fi
  env_args+=(-e "SANDBOX_RUNNER_URL${suffix}=http://codearena-sandbox-${lang}:8080/2015-03-31/functions/function/invocations")
  uid=$((uid + 1))
done

docker run -d --name "${suite}" python:3.13-slim sleep infinity >/dev/null
docker exec "${suite}" pip install --quiet --disable-pip-version-check --root-user-action=ignore "pytest==${pytest_version}"
docker network connect "${net}" "${suite}"
docker cp "${host_here}/." "${suite}:/suite"

status=0
docker exec "${env_args[@]}" "${suite}" python -m pytest -p no:cacheprovider -q -rA /suite || status=$?
if [ "${status}" -ne 0 ]; then
  for lang in "${languages[@]}"; do
    echo "--- ${lang} runner logs"
    docker logs --tail 30 "codearena-sandbox-${lang}" 2>&1 || true
  done
fi
exit "${status}"
