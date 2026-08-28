# --- build stage ----------------------------------------------------------
FROM golang:1.24-alpine AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/tmpdrop ./cmd/tmpdrop

# --- runtime stage ---------------------------------------------------------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 tmpdrop \
    && adduser -S -D -H -u 10001 -G tmpdrop tmpdrop
WORKDIR /app
RUN mkdir -p /app/data && chown tmpdrop:tmpdrop /app/data
COPY --from=build /out/tmpdrop /usr/local/bin/tmpdrop
USER tmpdrop
ENV TMPDROP_ADDR=:8080 \
    TMPDROP_STORAGE_DIR=/app/data
VOLUME ["/app/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q -O- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["tmpdrop"]
