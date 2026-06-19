# Multi-stage build for the Go services. The same Dockerfile builds either the
# WebSocket gateway or the REST api by passing --build-arg CMD=gateway|api.
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG CMD=gateway
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/${CMD}

# Distroless static, non-root: no shell, minimal attack surface.
FROM gcr.io/distroless/static-debian12:nonroot AS run
COPY --from=build /out/app /app
EXPOSE 8080 9000
ENTRYPOINT ["/app"]
