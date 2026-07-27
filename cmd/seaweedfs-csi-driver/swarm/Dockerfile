ARG BASE_IMAGE=chrislusf/s3-csi:latest
ARG ARCH=linux/amd64
FROM --platform=${ARCH} ${BASE_IMAGE}

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
