FROM golang:1.12-alpine AS builder
RUN apk add --no-cache ca-certificates git

WORKDIR /build/src
RUN go mod init product_pusher_runner
COPY . .
RUN go build -o /product_pusher_runner .

FROM alpine AS release
WORKDIR /bin
COPY --from=builder /product_pusher_runner ./product_pusher_runner
WORKDIR /etc
COPY products.json .
ENTRYPOINT ["/bin/product_pusher_runner"]

