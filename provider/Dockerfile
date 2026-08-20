# syntax=docker/dockerfile:1

# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /workspace

# Cache dependencies before copying source.
COPY go.mod go.mod
RUN go mod download

COPY api/     api/
COPY cmd/     cmd/
COPY internal/ internal/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /provider ./cmd/provider/

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
# Distroless nonroot image — minimal attack surface.
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /provider /provider

USER nonroot:nonroot

ENTRYPOINT ["/provider"]
