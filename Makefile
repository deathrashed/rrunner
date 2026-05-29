.PHONY: validate fmt test build shell-syntax dry-run diagnose actions actions-doc explain print-url install-dry-run

CORE_OUT ?= /tmp/rrunner-core
DRY_RUN_URL ?= rrunner://open?url=file:///Users/rd/Scripts/Riley/rrunner/README.md
ACTION ?= open
PATH_ARG ?= README.md
ACTIONS_DOC ?= docs/ACTIONS.generated.md

fmt:
	gofmt -w cmd/rrunner-core/main.go cmd/rrunner-core/main_test.go

test:
	go test ./...

build:
	go build -o $(CORE_OUT) ./cmd/rrunner-core

shell-syntax:
	bash -n bin/rrunner bin/rrunner.sh install.sh bin/md-restore.sh examples/plugins/riley-workflow/scripts/finder-convert.sh

dry-run: build
	$(CORE_OUT) --validate-install --json
	$(CORE_OUT) --dry-run '$(DRY_RUN_URL)'

diagnose: build
	$(CORE_OUT) --diagnose

actions: build
	$(CORE_OUT) --list-actions --markdown --agent-notes

actions-doc: build
	$(CORE_OUT) --export-actions '$(ACTIONS_DOC)' --agent-notes

explain: build
	$(CORE_OUT) --explain-action '$(ACTION)' --markdown

print-url: build
	$(CORE_OUT) --print-url '$(ACTION)' '$(PATH_ARG)'

install-dry-run:
	./install.sh --dry-run

validate: fmt test build shell-syntax dry-run
