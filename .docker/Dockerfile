FROM golang:1.21.6-alpine AS builder
RUN apk add --no-cache git

ARG BUILD_PATH

WORKDIR /go/src/github.com/canopy-network/canopy
COPY .. .

RUN CGO_ENABLED=0 GOOS=linux go build -a -o bin ./${BUILD_PATH}/*.go

FROM alpine:3.19
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /go/src/github.com/canopy-network/canopy/bin ./
USER appuser
ENTRYPOINT ["/app/bin"]
