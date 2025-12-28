ARG GOVERSION=1.25.5

FROM --platform=$BUILDPLATFORM golang:${GOVERSION}-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /build/api-wrapper main.go


FROM ghcr.io/zhaarey/apple-music-downloader:3c30f35bc4ae99d5d8f5da8458a6c951811bac58

EXPOSE 8080

# Default user and group IDs
ENV PUID=10001 \
    PGID=10001 \
    USER_NAME=amdownloader

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get install -y --no-install-recommends gosu \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd -g ${PGID} ${USER_NAME} \
    && useradd -u ${PUID} -g ${PGID} -m ${USER_NAME}

COPY --from=builder /build/api-wrapper /usr/local/bin/api-wrapper

COPY entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
CMD ["/usr/local/bin/api-wrapper"]
