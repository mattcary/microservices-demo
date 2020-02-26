FROM golang:1.12-alpine AS builder
RUN apk add --no-cache ca-certificates git

WORKDIR /build/src
RUN go mod init github.com/mattcary/microservices-demo/src
WORKDIR productcatalogservice
COPY . .
RUN go build -o /productcatalogservice .

FROM alpine AS release
RUN apk add --no-cache ca-certificates
RUN GRPC_HEALTH_PROBE_VERSION=v0.2.0 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe
WORKDIR /productcatalogservice
COPY --from=builder /productcatalogservice ./server
EXPOSE 3550
ENTRYPOINT ["/productcatalogservice/server"]

