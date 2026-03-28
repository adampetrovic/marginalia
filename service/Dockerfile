FROM golang:1.24.6-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /marginalia ./cmd/marginalia

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /marginalia /marginalia

EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/marginalia"]
