FROM --platform=$BUILDPLATFORM golang:1.24.6-alpine AS builder

# xx provides cross-compilation helpers so we can build arm64 binaries
# natively on amd64 instead of emulating via QEMU (~15min → ~1min).
COPY --from=tonistiigi/xx / /

# Platform-independent: install build tools and download Go modules
RUN apk add --no-cache clang lld
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Platform-dependent: install cross-compilation libs and build
ARG TARGETPLATFORM
RUN xx-apk add --no-cache gcc musl-dev
ENV CGO_ENABLED=1
RUN xx-go build -ldflags="-s -w -linkmode external -extldflags '-static'" -o /marginalia ./cmd/marginalia && \
    xx-verify --static /marginalia

# Create empty /data dir owned by nonroot (uid 65532)
RUN mkdir -p /data && chown 65532:65532 /data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /marginalia /marginalia
COPY --from=builder --chown=65532:65532 /data /data

EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/marginalia"]
