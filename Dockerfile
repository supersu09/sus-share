# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build

ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/sus-share ./cmd/sus-share

FROM alpine:3.22

RUN addgroup -S susshare && adduser -S -G susshare susshare \
    && mkdir -p /config /data \
    && chown -R susshare:susshare /config /data

COPY --from=build /out/sus-share /usr/local/bin/sus-share

USER susshare

VOLUME ["/config", "/data"]
EXPOSE 19876

ENTRYPOINT ["/usr/local/bin/sus-share"]
CMD ["-config", "/config/config.yaml"]
