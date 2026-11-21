# Production image of the Go API. Built in GitHub Actions (also used by the local docker-compose.yml).
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.20
ARG VERSION=dev
LABEL org.opencontainers.image.source="https://github.com/almukhanbetov/ortalyq-new" \
      org.opencontainers.image.title="ortalyq-new-api" \
      org.opencontainers.image.revision="$VERSION"
# CA bundle: outgoing HTTPS (GIPHY) and SMTP over TLS. Migrations are embedded in the binary.
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 app && mkdir -p /data/uploads && chown app /data/uploads
USER app
COPY --from=build /out/api /usr/local/bin/api
EXPOSE 8448
ENTRYPOINT ["api"]
CMD ["serve"]
