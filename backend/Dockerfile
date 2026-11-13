# syntax=docker/dockerfile:1
# One image per command: docker build --build-arg CMD=worker backend

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src

COPY go.* ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
ARG CMD=api
ARG VERSION=dev
ARG COMMIT=none
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /out/app "./cmd/${CMD}"

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/app"]
