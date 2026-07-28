# S3_REF pins the hanzoai/s3 source this image builds its `s3` binary from, and
# it MUST track the github.com/hanzoai/s3 version in go.mod — the mount daemon
# and the driver have to speak the same filer wire protocol as the server.
#
# The DEFAULT is load-bearing: our builder invokes buildkit with no --build-arg,
# so whatever is written here is what actually ships. It was left at a bare
# commit from before v1.0.2/v1.0.3/v1.0.4, which would have shipped a mount
# binary carrying the promise-ID deadlock and the stream head-of-line stall
# those releases fixed. Pin the TAG so it is legible and diffable against go.mod.
ARG S3_REF=v1.0.4

FROM golang:1.26-alpine AS builder

RUN apk add git g++

# Build the s3 binary from a specific commit for reproducibility
ARG S3_REF
RUN git clone https://github.com/hanzoai/s3 /go/src/github.com/hanzoai/s3 \
    && cd /go/src/github.com/hanzoai/s3 \
    && git checkout ${S3_REF} \
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
COPY --from=builder /go/bin/s3 /usr/bin/
COPY --from=builder /s3-mount /

RUN chmod +x /s3-mount
ENTRYPOINT ["/s3-mount"]
