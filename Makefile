IMAGE_NAME ?= prom-operator
IMAGE_TAG ?= local
REPO_NAME ?= neoseele

.PHONY: build
build:
	gcloud builds submit --config=cloudbuild.yaml

IMAGE := $(shell docker image inspect $(IMAGE_NAME):$(IMAGE_TAG) &>/dev/null || echo missing)

.PHONY: build-local
build-local:

ifeq ($(IMAGE),missing)
	@echo "building image [$(IMAGE_NAME):$(IMAGE_TAG)] ..."
	@docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .
else
	@echo "image [$(IMAGE_NAME):$(IMAGE_TAG)] already exists."
endif

.PHONY: build-dockerhub
build-dockerhub: build-local # depend on build-local
	@image_id=$$(docker images $(IMAGE_NAME):$(IMAGE_TAG) --format '{{.ID}}') && \
	if [ -n "$$image_id" ]; then \
		echo "$$image_id"; \
		docker tag $$image_id $(REPO_NAME)/$(IMAGE_NAME):$(IMAGE_TAG); \
		docker push $(REPO_NAME)/$(IMAGE_NAME); \
	fi

.PHONY: clean
clean:
	@echo "removing images ..."
	-@docker rmi $(IMAGE_NAME):$(IMAGE_TAG)
	-@docker rmi $(REPO_NAME)/$(IMAGE_NAME):$(IMAGE_TAG)