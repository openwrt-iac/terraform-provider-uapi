BINARY  := terraform-provider-uapi
VERSION ?= 0.1.0
# Dev override install location (see dev.tfrc).
HOSTNAME := registry.terraform.io
NAMESPACE := raspbeguy
NAME := uapi
OS_ARCH := $(shell go env GOOS)_$(shell go env GOARCH)
INSTALL_DIR := $(HOME)/.terraform.d/plugins/$(HOSTNAME)/$(NAMESPACE)/$(NAME)/$(VERSION)/$(OS_ARCH)

.PHONY: build install test testacc fmt vet tidy gen docs clean

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) .

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)_v$(VERSION)

test:
	go test ./...

# Acceptance tests run the provider end to end (real terraform binary) against an
# in-process fake uapi server, so they need no router. Requires a terraform/tofu
# binary on PATH.
testacc:
	TF_ACC=1 go test ./internal/provider/ -run TestAcc -count=1 -v -timeout 30m

fmt:
	gofmt -w .

vet:
	go vet ./...

tidy:
	go mod tidy

# Regenerate the curated resources/data sources from internal/gen/openapi.json
# plus the descriptor overlay. Idempotent: re-run after bumping the spec.
gen:
	go run ./internal/gen

# Regenerate code then docs/ from schema descriptions and examples/.
# `go generate ./...` runs the code generator first, then tfplugindocs.
docs:
	go generate ./...

clean:
	rm -f $(BINARY)
