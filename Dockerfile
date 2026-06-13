# ---- Builder stage ----
# Note: the Go module targets 1.26, so the builder must be >= 1.26.
# (The assignment suggested golang:1.22-alpine, but 1.22 cannot compile a
# go 1.26 module; we use 1.26-alpine which is the correct, compatible choice.)
FROM golang:1.26-alpine AS builder

# git is occasionally needed for module fetches; ca-certs for TLS.
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Cache deps first for faster rebuilds.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build a static binary.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server ./cmd/server

# ---- Final stage ----
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Run as a non-root user.
RUN addgroup -S app && adduser -S app -G app
USER app

COPY --from=builder /app/server /app/server

EXPOSE 3000

ENTRYPOINT ["/app/server"]
