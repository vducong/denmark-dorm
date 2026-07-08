# Housing waitlist scraper — command shortcuts
# Config: internal/config/config.yaml (see internal/config/config.example.yaml)

BINARY     := waitlist
CMD        := ./cmd/waitlist
CONFIG     := internal/config/config.yaml
CONFIG_EXAMPLE := internal/config/config.example.yaml
DEBUG_HTML := debug.html

.PHONY: help build test clean run run-no-email score dump install auth-sheets list-sources

help: ## Show targets
	@grep -E '^[a-zA-Z0-9_-]+:.*##' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

build: ## Build binary to ./$(BINARY)
	go build -o $(BINARY) $(CMD)

test: ## Run Go tests
	go test ./...

clean: ## Remove binary and debug HTML (keeps data/ history)
	rm -f $(BINARY) $(DEBUG_HTML)

install: build ## Alias for build

list-sources: build ## List registered sources
	./$(BINARY) --list-sources

run: build ## Run all enabled sources: scrape + CSV + sheet + email (requires config.yaml)
	@test -f $(CONFIG) || (echo "error: cp $(CONFIG_EXAMPLE) $(CONFIG) and edit" >&2 && exit 1)
	./$(BINARY)

run-no-email: build ## Run all enabled sources, skip email
	@test -f $(CONFIG) || (echo "error: cp $(CONFIG_EXAMPLE) $(CONFIG) and edit" >&2 && exit 1)
	./$(BINARY) --no-email

score: build ## Crawl fresh and write the scored candidates CSV only (no ranking CSV/sheet/email)
	@test -f $(CONFIG) || (echo "error: cp $(CONFIG_EXAMPLE) $(CONFIG) and edit" >&2 && exit 1)
	./$(BINARY) --score-only

dump: build ## Scrape one source, save HTML to debug.html, skip email + sheet (SRC=kkik)
	@test -f $(CONFIG) || (echo "error: cp $(CONFIG_EXAMPLE) $(CONFIG) and edit" >&2 && exit 1)
	./$(BINARY) --source $(or $(SRC),kkik) --dump-html $(DEBUG_HTML) --no-email --no-sheet

auth-sheets: build ## One-time Google OAuth for Sheets (requires config.yaml)
	@test -f $(CONFIG) || (echo "error: cp $(CONFIG_EXAMPLE) $(CONFIG) and edit" >&2 && exit 1)
	./$(BINARY) --auth-sheets
