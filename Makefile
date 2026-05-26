# KKIK waitlist scraper — command shortcuts
# Config: config.yaml (see config.example.yaml)

BINARY     := kkik-waitlist
CMD        := ./cmd/kkik-waitlist
CONFIG     := config.yaml
TEST_HTML  := testdata/list.html
DEBUG_HTML := debug.html

.PHONY: help build test clean parse run run-no-email dump install

help: ## Show targets
	@grep -E '^[a-zA-Z0-9_-]+:.*##' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

build: ## Build binary to ./$(BINARY)
	go build -o $(BINARY) $(CMD)

test: ## Run Go tests
	go test ./...

clean: ## Remove binary and generated CSV/debug files
	rm -f $(BINARY) *_kkik_waitlist.csv $(DEBUG_HTML)

install: build ## Alias for build

parse: build ## Parse testdata/list.html → CSV, no login or email
	./$(BINARY) --parse-only $(TEST_HTML) --no-email

run: build ## Live scrape + CSV + email (requires config.yaml)
	@test -f $(CONFIG) || (echo "error: cp config.example.yaml $(CONFIG) and edit" >&2 && exit 1)
	./$(BINARY)

run-no-email: build ## Live scrape + CSV, skip email (requires config.yaml)
	@test -f $(CONFIG) || (echo "error: cp config.example.yaml $(CONFIG) and edit" >&2 && exit 1)
	./$(BINARY) --no-email

dump: build ## Live scrape, save HTML to debug.html, no email (requires config.yaml)
	@test -f $(CONFIG) || (echo "error: cp config.example.yaml $(CONFIG) and edit" >&2 && exit 1)
	./$(BINARY) --dump-html $(DEBUG_HTML) --no-email
