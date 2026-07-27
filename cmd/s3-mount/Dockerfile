# SEAWEEDFS_COMMIT should match the commit in go.mod for reproducibility
# Update via: docker build --build-arg SEAWEEDFS_COMMIT=<hash> ...
ARG SEAWEEDFS_COMMIT=19ce7c0b6fdb

FROM golang:1.25-alpine AS builder

RUN apk add git g++

# Build weed binary from a specific commit for reproducibility
ARG SEAWEEDFS_COMMIT
RUN git clone https://github.com/s3/s3 /go/src/github.com/s3/s3 \
    && cd /go/src/github.com/s3/s3 \
    && git checkout ${SEAWEEDFS_COMMIT} \
    && cd weed \
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
