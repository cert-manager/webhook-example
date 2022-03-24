OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)

IMAGE_NAME := "ghcr.io/g-core/cert-manager-webhook-gcore"
IMAGE_TAG := "latest"

OUT := $(shell pwd)/_out

KUBE_VERSION=1.21.2

$(shell mkdir -p "$(OUT)")

clean:
	rm -Rf $(OUT)/kubebuilder

install-tools:
	sh ./scripts/fetch-test-binaries.sh

test: clean install-tools _test/kubebuilder
	TEST_ASSET_ETCD=_test/kubebuilder/bin/etcd \
    	TEST_ASSET_KUBE_APISERVER=_test/kubebuilder/bin/kube-apiserver \
    	TEST_ASSET_KUBECTL=_test/kubebuilder/bin/kubectl \
    	go test -v .

_test/kubebuilder:
	curl -fsSL https://go.kubebuilder.io/test-tools/$(KUBE_VERSION)/$(OS)/$(ARCH) -o kubebuilder-tools.tar.gz
	mkdir -p _test/kubebuilder
	tar -xvf kubebuilder-tools.tar.gz
	mv kubebuilder/bin _test/kubebuilder/
	rm kubebuilder-tools.tar.gz
	rm -R kubebuilder

clean: clean-kubebuilder

clean-kubebuilder:
	rm -Rf _test/kubebuilder

build:
	docker build -t "$(IMAGE_NAME):$(IMAGE_TAG)" .

push:
	docker push "$(IMAGE_NAME):$(IMAGE_TAG)"

.PHONY: rendered-manifest.yaml
rendered-manifest.yaml:
	helm template \
	    --name example-webhook \
        --set image.repository=$(IMAGE_NAME) \
        --set image.tag=$(IMAGE_TAG) \
        deploy/example-webhook > "$(OUT)/rendered-manifest.yaml"
