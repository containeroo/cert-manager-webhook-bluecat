GO ?= $(shell which go)
OS ?= $(shell $(GO) env GOOS)
ARCH ?= $(shell $(GO) env GOARCH)

IMAGE_NAME := "ghcr.io/containeroo/cert-manager-webhook-bluecat"
IMAGE_TAG := "latest"

LOCAL_IMAGE_NAME ?= bluecat-webhook
LOCAL_IMAGE_TAG ?= dev
KIND_CLUSTER_NAME ?= kind
WEBHOOK_NAMESPACE ?= cert-manager
WEBHOOK_DEPLOYMENT ?= bluecat-webhook

OUT := $(shell pwd)/_out

KUBEBUILDER_VERSION=1.28.0

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
	docker build -t "$(IMAGE_NAME):$(IMAGE_TAG)" .

.PHONY: kind-redeploy
kind-redeploy:
	docker build -t "$(LOCAL_IMAGE_NAME):$(LOCAL_IMAGE_TAG)" .
	kind load docker-image "$(LOCAL_IMAGE_NAME):$(LOCAL_IMAGE_TAG)" --name "$(KIND_CLUSTER_NAME)"
	kubectl -n "$(WEBHOOK_NAMESPACE)" set image deployment/"$(WEBHOOK_DEPLOYMENT)" "*=$(LOCAL_IMAGE_NAME):$(LOCAL_IMAGE_TAG)"
	kubectl -n "$(WEBHOOK_NAMESPACE)" rollout restart deployment/"$(WEBHOOK_DEPLOYMENT)"
	kubectl -n "$(WEBHOOK_NAMESPACE)" rollout status deployment/"$(WEBHOOK_DEPLOYMENT)"

_test $(OUT) _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH):
	mkdir -p $@
