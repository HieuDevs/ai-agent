.PHONY: build build-multi push push-multi run stop clean help all

IMAGE_NAME = hieubui1307/ai-agent
TAG ?= latest
REGISTRY ?= docker.io
PLATFORM ?= linux/amd64,linux/arm64,linux/arm/v7
SINGLE_PLATFORM ?= linux/amd64
FULL_IMAGE_NAME = $(REGISTRY)/$(IMAGE_NAME):$(TAG)

help:
	@echo "Available commands:"
	@echo "  make build       - Build Docker image for local platform"
	@echo "  make build-multi - Build multi-platform Docker image"
	@echo "  make push        - Push Docker image to registry"
	@echo "  make push-multi  - Build and push multi-platform images"
	@echo "  make clean       - Remove Docker image"
	@echo "  make all         - Build and push Docker image"
	@echo ""
	@echo "Variables:"
	@echo "  TAG=latest       - Docker image tag (default: latest)"
	@echo "  REGISTRY=docker.io - Docker registry (default: docker.io)"
	@echo "  PLATFORM=linux/amd64,linux/arm64,linux/arm/v7 - Target platforms"
	@echo "  SINGLE_PLATFORM=linux/amd64 - Single platform for local build"

build: clean
	@echo "Building Docker image: $(FULL_IMAGE_NAME) for platform: $(SINGLE_PLATFORM)"
	docker buildx build --platform $(SINGLE_PLATFORM) -t $(IMAGE_NAME):$(TAG) --load .
	docker tag $(IMAGE_NAME):$(TAG) $(FULL_IMAGE_NAME)
	@echo "Build complete!"

build-multi:
	@echo "Building multi-platform Docker image: $(FULL_IMAGE_NAME)"
	@echo "Platforms: $(PLATFORM)"
	docker buildx build --platform $(PLATFORM) -t $(FULL_IMAGE_NAME) .
	@echo "Multi-platform build complete!"

push:
	@echo "Pushing Docker image: $(FULL_IMAGE_NAME)"
	docker push $(FULL_IMAGE_NAME)
	@echo "Push complete!"

push-multi:
	@echo "Building and pushing multi-platform Docker image: $(FULL_IMAGE_NAME)"
	@echo "Platforms: $(PLATFORM)"
	docker buildx build --platform $(PLATFORM) -t $(FULL_IMAGE_NAME) --push .
	@echo "Multi-platform push complete!"

clean:
	@echo "Removing Docker image..."
	docker rmi $(IMAGE_NAME):$(TAG) || true
	docker rmi $(FULL_IMAGE_NAME) || true
	docker image prune -f
	@echo "Image removed!"

all: clean build push

