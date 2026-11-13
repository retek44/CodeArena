# syntax=docker/dockerfile:1
# C++ runner. Build context: runners/
# Same runner core as the Python image (the core is Python), plus GCC. Programs are
# linked statically, so they need no shared libraries at run time.
FROM public.ecr.aws/lambda/python:3.13

# hadolint ignore=DL3041
RUN dnf install -y gcc-c++ glibc-static libstdc++-static \
    && dnf clean all \
    && rm -rf /var/cache/dnf

COPY core/codearena_runner ${LAMBDA_TASK_ROOT}/codearena_runner

ENV CODEARENA_LANGUAGES=cpp

# Lambda always runs functions as a non-root user. Do the same locally,
# otherwise RLIMIT_NPROC (fork bomb protection) is not enforced.
USER 993:990

CMD ["codearena_runner.handler.handler"]
