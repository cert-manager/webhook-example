OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)

GCP_PROJECT ?= pluralsh
IMAGE_NAME := "plural-certmanager-webhook"
IMAGE_TAG := "0.1.0"
DKR_HOST ?= dkr.plural.sh

OUT := $(shell pwd)/_out

KUBEBUILDER_VERSION=2.3.2

$(shell mkdir -p "$(OUT)")

test: _test/kubebuilder
	go test -v .

_test/kubebuilder:
	curl -fsSL https://github.com/kubernetes-sigs/kubebuilder/releases/download/v$(KUBEBUILDER_VERSION)/kubebuilder_$(KUBEBUILDER_VERSION)_$(OS)_$(ARCH).tar.gz -o kubebuilder-tools.tar.gz
	mkdir -p _test/kubebuilder
	tar -xvf kubebuilder-tools.tar.gz
	mv kubebuilder_$(KUBEBUILDER_VERSION)_$(OS)_$(ARCH)/bin _test/kubebuilder/
	rm kubebuilder-tools.tar.gz
	rm -R kubebuilder_$(KUBEBUILDER_VERSION)_$(OS)_$(ARCH)

clean: clean-kubebuilder

clean-kubebuilder:
	rm -Rf _test/kubebuilder

build:
	docker build -t gcr.io/$(GCP_PROJECT)/$(IMAGE_NAME):$(IMAGE_TAG) \
			         -t $(DKR_HOST)/bootstrap/$(IMAGE_NAME):$(IMAGE_TAG) .

push:
	docker push gcr.io/$(GCP_PROJECT)/$(IMAGE_NAME):$(IMAGE_TAG)
	docker push $(DKR_HOST)/bootstrap/$(IMAGE_NAME):$(IMAGE_TAG)

.PHONY: rendered-manifest.yaml
rendered-manifest.yaml:
	helm template \
	    --name example-webhook \
        --set image.repository=$(IMAGE_NAME) \
        --set image.tag=$(IMAGE_TAG) \
        deploy/example-webhook > "$(OUT)/rendered-manifest.yaml"
