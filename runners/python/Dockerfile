# syntax=docker/dockerfile:1
# Build context: runners/
# The AWS base image contains the Lambda Runtime Interface Emulator, so the same
# image runs locally (port 8080) and in Lambda.
FROM public.ecr.aws/lambda/python:3.13

COPY core/codearena_runner ${LAMBDA_TASK_ROOT}/codearena_runner
# Drops the AWS credentials before the runtime starts (core/entrypoint.sh).
COPY --chmod=0755 core/entrypoint.sh /codearena-entrypoint.sh
ENTRYPOINT ["/codearena-entrypoint.sh"]

# Lambda always runs functions as a non-root user. Do the same locally,
# otherwise RLIMIT_NPROC (fork bomb protection) is not enforced.
USER 993:990

CMD ["codearena_runner.handler.handler"]
