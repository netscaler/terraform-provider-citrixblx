GOFMT_FILES?=$$(find . -name '*.go' |grep -v vendor | grep -v tools)

default: build

build: fmtcheck
	go install
fmt:
	@echo "==> Fixing source code with gofmt..."
	gofmt -s -w ./$(GOFMT_FILES)

fmtcheck:
	@sh -c "'$(CURDIR)/scripts/gofmtcheck.sh'"

.PHONY: build fmt fmtcheck
