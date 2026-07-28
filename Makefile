# VERSION=latest make push
.PHONY: build container container-csi container-mount push push-csi push-mount clean deps

REGISTRY_NAME ?= chrislusf
DRIVER_IMAGE_NAME ?= s3-csi
MOUNT_IMAGE_NAME ?= s3-mount
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD)
LDFLAGS ?= -s -w -X github.com/hanzoai/s3-csi/pkg/driver.gitCommit=${COMMIT}

OUTPUT_DIR := _output
DRIVER_BINARY := $(OUTPUT_DIR)/s3-csi
MOUNT_BINARY := $(OUTPUT_DIR)/s3-mount

DRIVER_IMAGE_TAG := $(REGISTRY_NAME)/$(DRIVER_IMAGE_NAME):$(VERSION)
MOUNT_IMAGE_TAG := $(REGISTRY_NAME)/$(MOUNT_IMAGE_NAME):$(VERSION)

deps:
	go mod tidy

build: $(DRIVER_BINARY) $(MOUNT_BINARY)

$(OUTPUT_DIR):
	mkdir -p $@

$(DRIVER_BINARY): | $(OUTPUT_DIR)
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags '$(LDFLAGS)' -o $@ ./cmd/s3-csi-driver/main.go

$(MOUNT_BINARY): | $(OUTPUT_DIR)
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags '$(LDFLAGS)' -o $@ ./cmd/s3-mount/main.go

container: container-csi container-mount

container-csi: $(DRIVER_BINARY)
	docker build -t $(DRIVER_IMAGE_TAG) -f cmd/s3-csi-driver/Dockerfile.dev .

container-mount: $(MOUNT_BINARY)
	docker build -t $(MOUNT_IMAGE_TAG) -f cmd/s3-mount/Dockerfile.dev .

push: push-csi push-mount

push-csi: container-csi
	docker push $(DRIVER_IMAGE_TAG)

push-mount: container-mount
	docker push $(MOUNT_IMAGE_TAG)

clean:
	go clean -r -x
	-rm -rf $(OUTPUT_DIR)
