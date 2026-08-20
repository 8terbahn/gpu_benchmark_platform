# Build stage: compile the operator binary from source
FROM golang:1.22-alpine AS builder

WORKDIR /workspace

# Cache dependencies first for faster rebuilds
COPY go.mod go.sum ./
RUN go mod download

COPY api/ api/
COPY controllers/ controllers/
COPY cmd/ cmd/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -a -ldflags="-s -w" -o manager ./cmd/main.go

# Runtime stage: minimal distroless image, runs as non-root
FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
