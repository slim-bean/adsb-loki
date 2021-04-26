ARG BUILD_PLATFORM=linux/amd64
ARG TARGET_PLATFORM=linux/amd64

FROM --platform=${BUILD_PLATFORM} golang:1.16-alpine AS builder
ARG COMPILE_GOOS=linux
ARG COMPILE_GOARCH=amd64
ARG COMPILE_GOARM=""
COPY . /build
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=${COMPILE_GOOS} GOARCH=${COMPILE_GOARCH} GOARM=${COMPILE_GOARM} go build -ldflags '-extldflags "-static"' -o adsb-loki ./cmd/adsb-loki/main.go
RUN mv /build/adsb-loki /

FROM --platform=${TARGET_PLATFORM} alpine
WORKDIR /
COPY --from=builder /adsb-loki /adsb-loki
RUN mkdir -p /adsb-loki-data && mkdir -p /etc/adsb-loki
ENTRYPOINT [ "/adsb-loki" ]
CMD ["-config.file=/etc/adsb-loki/adsb-loki.yaml"]