# syntax=docker/dockerfile:1
# JavaScript runner (Node.js 22). Build context: runners/
# Same runner core as the other images (the core is Python), plus Node.js.
FROM public.ecr.aws/lambda/python:3.13

# hadolint ignore=DL3041
RUN dnf install -y nodejs22 \
    && dnf clean all \
    && rm -rf /var/cache/dnf \
    && ln -sf "$(command -v node-22 || command -v node)" /usr/local/bin/codearena-node

COPY core/codearena_runner ${LAMBDA_TASK_ROOT}/codearena_runner
# The entrypoint drops the AWS credentials before the runtime starts (core/entrypoint.sh).
COPY --chmod=0755 core/entrypoint.sh /codearena-entrypoint.sh

ENV CODEARENA_LANGUAGES=javascript

# Lambda always runs functions as a non-root user. Do the same locally,
# otherwise RLIMIT_NPROC (fork bomb protection) is not enforced.
USER 993:990

ENTRYPOINT ["/codearena-entrypoint.sh"]
CMD ["codearena_runner.handler.handler"]
