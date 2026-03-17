SHELL := /bin/bash
GO    := go
MODULE := github.com/fbsobreira/gotron-examples

EXAMPLES := address justlend-energy sunswap

GOIMPORTS_VERSION := v0.28.0

.PHONY: all build tidy tidy-check fmt fmt-check lint check clean $(addprefix run/,$(EXAMPLES))

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

## Cleanup

clean:
	$(GO) clean ./...
