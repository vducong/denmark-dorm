# KKIK waitlist scraper — command shortcuts
# Config: internal/config/config.yaml (see internal/config/config.example.yaml)

BINARY     := kkik-waitlist
CMD        := ./cmd/kkik-waitlist
CONFIG     := internal/config/config.yaml
CONFIG_EXAMPLE := internal/config/config.example.yaml
DEBUG_HTML := debug.html

.PHONY: help build test clean run run-no-email dump install auth-sheets

help: ## Show targets
	@grep -E '^[a-zA-Z0-9_-]+:.*##' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

build: ## Build binary to ./$(BINARY)
	go build -o $(BINARY) $(CMD)

test: ## Run Go tests
	go test ./...

clean: ## Remove binary and generated CSV/debug files
	rm -f $(BINARY) *_kkik_waitlist.csv $(DEBUG_HTML)

install: build ## Alias for build

run: build ## Live scrape + CSV + email (requires config.yaml)
	@test -f $(CONFIG) || (echo "error: cp $(CONFIG_EXAMPLE) $(CONFIG) and edit" >&2 && exit 1)
	./$(BINARY)

run-no-email: build ## Live scrape + CSV, skip email (STEPS_EMAIL=false)
	@test -f $(CONFIG) || (echo "error: cp $(CONFIG_EXAMPLE) $(CONFIG) and edit" >&2 && exit 1)
	STEPS_EMAIL=false ./$(BINARY)

dump: build ## Live scrape, save HTML to debug.html, no email (requires config.yaml)
	@test -f $(CONFIG) || (echo "error: cp $(CONFIG_EXAMPLE) $(CONFIG) and edit" >&2 && exit 1)
	STEPS_EMAIL=false ./$(BINARY) --dump-html $(DEBUG_HTML)

auth-sheets: build ## One-time Google OAuth for Sheets (requires config.yaml)
	@test -f $(CONFIG) || (echo "error: cp $(CONFIG_EXAMPLE) $(CONFIG) and edit" >&2 && exit 1)
	./$(BINARY) --auth-sheets
