#!/bin/sh
# Entrypoint of every runner image (ADR-0003). Lambda puts the function role's
# credentials into the environment, and its seccomp filter refuses
# prctl(PR_SET_DUMPABLE): a submission of the same user could read them from
# /proc/<pid>/environ. The runner needs none (no network), so drop them here. exec
# replaces this shell, so no process of the execution environment keeps them.
unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN \
  AWS_CONTAINER_CREDENTIALS_FULL_URI AWS_CONTAINER_AUTHORIZATION_TOKEN
exec /lambda-entrypoint.sh "$@"
