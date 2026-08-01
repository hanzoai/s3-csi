FROM golang:1.26-alpine as builder
ENV GOTOOLCHAIN=auto

RUN apk add git g++

WORKDIR /go/src/github.com/hanzoai/s3-csi
COPY . .

RUN go build -ldflags="-s -w" -o /s3-csi ./cmd/s3-csi-driver/main.go && go clean -cache -modcache

FROM alpine AS final
RUN apk add fuse
LABEL org.opencontainers.image.source="https://github.com/hanzoai/s3-csi"
COPY --from=builder /s3-csi /

RUN chmod +x /s3-csi
ENTRYPOINT ["/s3-csi"]
