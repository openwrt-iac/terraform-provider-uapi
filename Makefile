BINARY  := terraform-provider-uapi
VERSION ?= 0.1.0
# Dev override install location (see dev.tfrc).
HOSTNAME := registry.terraform.io
NAMESPACE := raspbeguy
NAME := uapi
OS_ARCH := $(shell go env GOOS)_$(shell go env GOARCH)
INSTALL_DIR := $(HOME)/.terraform.d/plugins/$(HOSTNAME)/$(NAMESPACE)/$(NAME)/$(VERSION)/$(OS_ARCH)

.PHONY: build install test testacc fmt vet tidy clean

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) .

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)_v$(VERSION)

test:
	go test ./...

# Acceptance tests need a live uapi instance; set UAPI_ENDPOINT/UAPI_TOKEN and TF_ACC=1.
testacc:
	TF_ACC=1 go test ./... -v -timeout 120m

fmt:
	gofmt -w .

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)
