FROM golang:1.21.6-alpine AS builder

ARG BUILD_PATH

WORKDIR /go/src/github.com/canopy-network/canopy
COPY . /go/src/github.com/canopy-network/canopy

RUN make build-wallet
RUN make build-explorer
RUN go build -a -o bin ./${BUILD_PATH}/*.go

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /go/src/github.com/canopy-network/canopy/bin ./
ENTRYPOINT ["/app/bin"]
