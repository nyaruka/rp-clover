# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.26 AS builder

WORKDIR /src

# pre-copy go.mod/go.sum so dependency downloads are cached across builds
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

# VERSION is injected into main.version so the binary serves its embedded assets
# (rather than reading ./static from disk) and reports a meaningful version. Pass
# --build-arg VERSION=<tag> from the release pipeline; the default suits local builds.
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /rp-clover ./cmd/rp-clover

# ---- runtime stage ----
# distroless static provides CA certificates (for outbound HTTPS forwarding and
# Postgres TLS) and tzdata, runs as a non-root UID, and ships no shell or package
# manager. The static binary needs nothing else, so the root filesystem can be
# mounted read-only.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /rp-clover /rp-clover

EXPOSE 8060

ENTRYPOINT ["/rp-clover"]
