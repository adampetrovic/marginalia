FROM golang:1.24.6-alpine AS builder
WORKDIR /build
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /mock-readeck .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /mock-readeck /mock-readeck
EXPOSE 9090
ENTRYPOINT ["/mock-readeck"]
