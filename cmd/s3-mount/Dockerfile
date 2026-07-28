# S3_COMMIT should match the commit in go.mod for reproducibility
# Update via: docker build --build-arg S3_COMMIT=<hash> ...
ARG S3_COMMIT=6dda4e4d6bca

FROM golang:1.26-alpine AS builder

RUN apk add git g++

# Build the s3 binary from a specific commit for reproducibility
ARG S3_COMMIT
RUN git clone https://github.com/hanzoai/s3 /go/src/github.com/hanzoai/s3 \
    && cd /go/src/github.com/hanzoai/s3 \
    && git checkout ${S3_COMMIT} \
    && cd s3 \
    && go install \
    && go clean -cache -modcache

# Build s3-mount from current context
WORKDIR /go/src/github.com/hanzoai/s3-csi
COPY . .

RUN go build -ldflags="-s -w" -o /s3-mount ./cmd/s3-mount/main.go && go clean -cache -modcache

FROM alpine AS final
RUN apk add fuse
LABEL org.opencontainers.image.source="https://github.com/hanzoai/s3-csi"
COPY --from=builder /go/bin/weed /usr/bin/
COPY --from=builder /s3-mount /

RUN chmod +x /s3-mount
ENTRYPOINT ["/s3-mount"]
