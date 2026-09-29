ARG GOVERSION=1.27.1

FROM --platform=$BUILDPLATFORM golang:${GOVERSION}-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /build/api-wrapper main.go


FROM ghcr.io/tikhonp/apple-music-downloader:0.1

EXPOSE 8080

# Default user and group IDs
ENV PUID=10001 \
    PGID=10001 \
    USER_NAME=amdownloader

RUN apk add --no-cache shadow su-exec

RUN groupadd -g ${PGID} ${USER_NAME} \
    && useradd -u ${PUID} -g ${PGID} -m ${USER_NAME}

# amdl reads config.yaml from its working directory
WORKDIR /app

COPY --from=builder /build/api-wrapper /usr/local/bin/api-wrapper

COPY entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
CMD ["/usr/local/bin/api-wrapper"]
