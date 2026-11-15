# syntax=docker/dockerfile:1
# Go runner. Build context: runners/
# Same runner core as the other images (the core is Python), plus the Go toolchain.
FROM public.ecr.aws/lambda/python:3.13

# hadolint ignore=DL3041
RUN dnf install -y golang \
    && dnf clean all \
    && rm -rf /var/cache/dnf

# The standard library is compiled once, here. A submission is compiled and linked with
# the compile and link tools against these packages (go/build.sh): no build cache to
# write at run time, nothing a submission could leave behind for the next one.
COPY go/build.sh /opt/codearena-go/build.sh
# hadolint ignore=SC2016
RUN export CGO_ENABLED=0 GOCACHE=/opt/codearena-go/cache GOFLAGS=-trimpath \
    && go list -deps -export -f '{{if .Export}}packagefile {{.ImportPath}}={{.Export}}{{end}}' std \
       > /opt/codearena-go/importcfg \
    && ln -s "$(go env GOTOOLDIR)" /opt/codearena-go/tools \
    && echo "GOEXPERIMENT=$(go env GOEXPERIMENT)" > /opt/codearena-go/env \
    && chmod -R a+rX /opt/codearena-go \
    && chmod 0755 /opt/codearena-go/build.sh

COPY core/codearena_runner ${LAMBDA_TASK_ROOT}/codearena_runner
# The entrypoint drops the AWS credentials before the runtime starts (core/entrypoint.sh).
COPY --chmod=0755 core/entrypoint.sh /codearena-entrypoint.sh

ENV CODEARENA_LANGUAGES=go

# Lambda always runs functions as a non-root user. Do the same locally,
# otherwise RLIMIT_NPROC (fork bomb protection) is not enforced.
USER 993:990

ENTRYPOINT ["/codearena-entrypoint.sh"]
CMD ["codearena_runner.handler.handler"]
