#!/usr/bin/env bash
# Runs the sandbox hardening suite against the runner images (ADR-0003).
#   tests/sandbox-security/run.sh <python runner image> <cpp runner image>
#
# The runners run like Lambda (non-root, read-only root file system, writable /tmp only)
# on an internal Docker network: no route out and no DNS, like the isolated subnets on
# AWS. The test container joins that network after it installed pytest, and calls the
# Runtime Interface Emulator in each runner container.
set -euo pipefail

python_image="${1:?python runner image}"
cpp_image="${2:?cpp runner image}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Git Bash on Windows: docker needs the Windows form of the path (pwd -W).
host_here="$(cd "${here}" && { pwd -W 2>/dev/null || pwd; })"
net=codearena-sandbox
py=codearena-sandbox-python
cpp=codearena-sandbox-cpp
suite=codearena-sandbox-suite
pytest_version=$(grep -E '^pytest==' "${here}/../../requirements-dev.txt" | cut -d= -f3)

cleanup() {
  docker rm -f "${py}" "${cpp}" "${suite}" >/dev/null 2>&1 || true
  docker network rm "${net}" >/dev/null 2>&1 || true
}
trap cleanup EXIT
cleanup

docker network create --internal "${net}" >/dev/null
# RLIMIT_NPROC counts the tasks of a uid across the whole host, not per container. The
# images run as uid 993, like the local stack (docker compose) does, so each runner here
# gets a uid of its own; otherwise other runners would use up its process budget. On AWS
# every execution environment is its own microVM.
docker run -d --name "${py}" --network "${net}" --read-only --tmpfs /tmp:rw,size=256m --memory 1g \
  --user 20001:990 "${python_image}" >/dev/null
# The compiled program runs from /tmp (Lambda allows it); Docker tmpfs is noexec by default.
docker run -d --name "${cpp}" --network "${net}" --read-only --tmpfs /tmp:rw,exec,size=256m --memory 1g \
  --user 20002:990 "${cpp_image}" >/dev/null

docker run -d --name "${suite}" python:3.13-slim sleep infinity >/dev/null
docker exec "${suite}" pip install --quiet --disable-pip-version-check --root-user-action=ignore "pytest==${pytest_version}"
docker network connect "${net}" "${suite}"
docker cp "${host_here}/." "${suite}:/suite"

invocations=2015-03-31/functions/function/invocations
status=0
docker exec \
  -e "SANDBOX_RUNNER_URL=http://${py}:8080/${invocations}" \
  -e "SANDBOX_RUNNER_URL_CPP=http://${cpp}:8080/${invocations}" \
  "${suite}" python -m pytest -p no:cacheprovider -q -rA /suite || status=$?
if [ "${status}" -ne 0 ]; then
  echo "--- python runner logs"
  docker logs --tail 50 "${py}" 2>&1 || true
  echo "--- cpp runner logs"
  docker logs --tail 50 "${cpp}" 2>&1 || true
fi
exit "${status}"
