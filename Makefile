GO ?= $(shell which go)
OS ?= $(shell $(GO) env GOOS)
ARCH ?= $(shell $(GO) env GOARCH)

IMAGE_NAME := "icr.io/skills-network/cert-manager-webhook-ibm-cis"
IMAGE_TAG := "latest"

OUT := $(shell pwd)/_out

KUBEBUILDER_VERSION=1.28.0

HELM_FILES := $(shell find deploy/cert-manager-ibm-cis-webhook)

test: _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/etcd _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/kube-apiserver _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/kubectl
	TEST_ASSET_ETCD=_test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/etcd \
	TEST_ASSET_KUBE_APISERVER=_test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/kube-apiserver \
	TEST_ASSET_KUBECTL=_test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/kubectl \
	$(GO) test -v .

_test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH).tar.gz: | _test
	curl -fsSL https://go.kubebuilder.io/test-tools/$(KUBEBUILDER_VERSION)/$(OS)/$(ARCH) -o $@

_test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/etcd _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/kube-apiserver _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/kubectl: _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH).tar.gz | _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)
	tar xfO $< kubebuilder/bin/$(notdir $@) > $@ && chmod +x $@

.PHONY: clean
clean:
	rm -r _test $(OUT)

.PHONY: build
build:
	docker buildx build -t "$(IMAGE_NAME):$(IMAGE_TAG)" . --platform linux/amd64 

.PHONY: rendered-manifest.yaml
rendered-manifest.yaml: $(OUT)/rendered-manifest.yaml

$(OUT)/rendered-manifest.yaml: $(HELM_FILES) | $(OUT)
	helm template \
	    --name cert-manager-ibm-cis-webhook \
            --set image.repository=$(IMAGE_NAME) \
            --set image.tag=$(IMAGE_TAG) \
            deploy/cert-manager-ibm-cis-webhook > $@

_test $(OUT) _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH):
	mkdir -p $@
