SHELL := /bin/bash
GO    := go
MODULE := github.com/fbsobreira/gotron-examples

EXAMPLES := address justlend-energy sunswap pq-capabilities pq-keys pq-transfer pq-migrate

GOIMPORTS_VERSION := v0.28.0

# FN-DSA-512 (Falcon) signing needs CGO and liboqs: brew install liboqs pkgconf
GO_FALCON := CGO_ENABLED=1 $(GO)

.PHONY: all build build-falcon tidy tidy-check fmt fmt-check lint check clean $(addprefix run/,$(EXAMPLES)) run/pq-keys-falcon

all: check build

## Build

build:
	$(GO) build ./...

## Dependencies

tidy:
	$(GO) mod tidy

tidy-check:
	$(GO) mod tidy
	git diff --exit-code go.mod go.sum

## Formatting

fmt:
	@which goimports > /dev/null || $(GO) install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)
	goimports -w -local $(MODULE) .

fmt-check:
	@which goimports > /dev/null || $(GO) install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)
	@diff=$$(goimports -l -local $(MODULE) .); \
	if [ -n "$$diff" ]; then \
		echo "goimports: files need formatting:"; \
		echo "$$diff"; \
		exit 1; \
	fi

## Linting
# Install golangci-lint via: brew install golangci-lint
# or: https://golangci-lint.run/welcome/install/

lint:
	golangci-lint run ./...

## Combined checks (matches CI)

check: tidy-check fmt-check lint build

## Run examples

run/address:
	$(GO) run ./examples/address

run/sunswap:
	$(GO) run ./examples/sunswap $(ARGS)

run/justlend-energy:
	$(GO) run ./examples/justlend-energy $(ARGS)

run/pq-capabilities:
	$(GO) run ./examples/pq-capabilities $(ARGS)

run/pq-keys:
	$(GO) run ./examples/pq-keys $(ARGS)

run/pq-keys-falcon:
	$(GO_FALCON) run -tags falcon ./examples/pq-keys -scheme fn-dsa-512 $(ARGS)

run/pq-transfer:
	$(GO_FALCON) run -tags falcon ./examples/pq-transfer $(ARGS)

run/pq-migrate:
	$(GO) run ./examples/pq-migrate $(ARGS)

build-falcon:
	$(GO_FALCON) build -tags falcon ./...

## Cleanup

clean:
	$(GO) clean ./...
