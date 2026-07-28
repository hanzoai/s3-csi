#!/bin/sh

if [ -z "$FILER" ]; then
	echo "FILER is not set!"
	exit 1
fi

NODE_ID=$(cat /node_hostname)
C_WRITER=${C_WRITER:-32}
CMD="/s3-csi-driver --filer=$FILER --nodeid=${NODE_ID} --endpoint=unix://run/docker/plugins/s3.sock --concurrentWriters=${C_WRITER} --dataCenter=${DATACENTER} --dataLocality=none --logtostderr --map.uid=${UID_MAP} --map.gid=${GID_MAP} --cacheCapacityMB=${CACHE_SIZE} --cacheDir=/tmp/s3/docker-csi" 

exec $CMD
