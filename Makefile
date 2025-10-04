.PHONY: build push run stop clean help

IMAGE_NAME = hieubui1307/ai-agent
TAG ?= latest
REGISTRY ?= docker.io
FULL_IMAGE_NAME = $(REGISTRY)/$(IMAGE_NAME):$(TAG)

help:
	@echo "Available commands:"
	@echo "  make build       - Build Docker image"
	@echo "  make push        - Push Docker image to registry"
	@echo "  make run         - Run Docker container"
	@echo "  make stop        - Stop and remove Docker container"
	@echo "  make clean       - Remove Docker image"
	@echo "  make all         - Build and push Docker image"
	@echo ""
	@echo "Variables:"
	@echo "  TAG=latest       - Docker image tag (default: latest)"
	@echo "  REGISTRY=docker.io - Docker registry (default: docker.io)"

build:
	@echo "Building Docker image: $(FULL_IMAGE_NAME)"
	docker build -t $(IMAGE_NAME):$(TAG) .
	docker tag $(IMAGE_NAME):$(TAG) $(FULL_IMAGE_NAME)
	@echo "Build complete!"

push:
	@echo "Pushing Docker image: $(FULL_IMAGE_NAME)"
	docker push $(FULL_IMAGE_NAME)
	@echo "Push complete!"

run:
	@echo "Running Docker container..."
	docker run -d \
		-p 8080:8080 \
		-e OPENROUTER_API_KEY=${OPENROUTER_API_KEY} \
		-v $(PWD)/exports:/app/exports \
		-v $(PWD)/prompts:/app/prompts \
		--name $(IMAGE_NAME) \
		$(IMAGE_NAME):$(TAG)
	@echo "Container started! Access at http://localhost:8080"

stop:
	@echo "Stopping and removing container..."
	docker stop $(IMAGE_NAME) || true
	docker rm $(IMAGE_NAME) || true
	@echo "Container stopped and removed!"

clean:
	@echo "Removing Docker image..."
	docker rmi $(IMAGE_NAME):$(TAG) || true
	docker rmi $(FULL_IMAGE_NAME) || true
	docker image prune -f
	@echo "Image removed!"

all: clean build push

