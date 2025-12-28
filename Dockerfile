ARG GOVERSION=1.25.5

FROM --platform=$BUILDPLATFORM golang:${GOVERSION}-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /build/api-wrapper main.go


FROM ghcr.io/zhaarey/apple-music-downloader:3c30f35bc4ae99d5d8f5da8458a6c951811bac58
ARG UID=10001
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    appuser
USER appuser
COPY --from=builder /build/api-wrapper /usr/local/bin/api-wrapper
EXPOSE 8080
# Change entrypoint to run the API wrapper instead
ENTRYPOINT []
CMD ["/usr/local/bin/api-wrapper"]
