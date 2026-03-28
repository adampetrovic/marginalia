FROM golang:1.24.6-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w -linkmode external -extldflags '-static'" -o /marginalia ./cmd/marginalia

# Create empty /data dir owned by nonroot (uid 65532)
RUN mkdir -p /data && chown 65532:65532 /data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /marginalia /marginalia
COPY --from=builder --chown=65532:65532 /data /data

EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/marginalia"]
