FROM golang:1.25-alpine as builder

RUN apk add git g++

WORKDIR /go/src/github.com/hanzoai/s3-csi
COPY . .

RUN go build -ldflags="-s -w" -o /s3-csi ./cmd/s3-csi/main.go && go clean -cache -modcache

FROM alpine AS final
RUN apk add fuse
LABEL author="Chris Lu"
COPY --from=builder /s3-csi /

RUN chmod +x /s3-csi
ENTRYPOINT ["/s3-csi"]
