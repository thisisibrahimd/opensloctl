DASHBOARDS_DIR := dashboards
LEGACY_DASHBOARDS_DIR := ../../deploy/dashboards
RULES_OUT_DIR := ../../deploy/rules
RULES_SRC_DIR := rules
PROMTOOL := $(shell mise which promtool 2>/dev/null || command -v promtool 2>/dev/null || echo promtool)
# Dashboard/render annotation pulled from the current git tree (short
# SHA + dirty marker). The `version` extVar that Grafana dashboards
# consume stays alive so the jsonnet blocks below don't need editing -
# they bind whatever `-V version=…` provides. Git SHA gives one
# unique-per-commit identifier; rendering off `git diff` would noisily
# churn the dashboards on every local save so SHA is the practical pick.
GIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DIRTY    := $(shell git diff --quiet 2>/dev/null && echo clean || echo dirty)
VERSION  := $(GIT_HASH)-$(DIRTY)

.PHONY: generate
generate: $(DASHBOARDS_DIR)
	jsonnet -J ../tmp/grafonnet-vendor -V version=$(VERSION) \
	  openslo-list.jsonnet > $(DASHBOARDS_DIR)/openslo-list.json
	jsonnet -J ../tmp/grafonnet-vendor -V version=$(VERSION) \
	  openslo-detail.jsonnet > $(DASHBOARDS_DIR)/openslo-detail.json

.PHONY: list
list: $(DASHBOARDS_DIR)
	jsonnet -J ../tmp/grafonnet-vendor -V version=$(VERSION) \
	  openslo-list.jsonnet > $(DASHBOARDS_DIR)/openslo-list.json

.PHONY: detail
detail: $(DASHBOARDS_DIR)
	jsonnet -J ../tmp/grafonnet-vendor -V version=$(VERSION) \
	  openslo-detail.jsonnet > $(DASHBOARDS_DIR)/openslo-detail.json

.PHONY: clean
clean:
	rm -rf $(DASHBOARDS_DIR)

.PHONY: sync-legacy
sync-legacy: generate
	@echo "==> Replacing legacy $(LEGACY_DASHBOARDS_DIR)/*.json with the new render..."
	@cp $(DASHBOARDS_DIR)/openslo-list.json  $(LEGACY_DASHBOARDS_DIR)/openslo-list.json
	@cp $(DASHBOARDS_DIR)/openslo-detail.json $(LEGACY_DASHBOARDS_DIR)/openslo-detail.json
	@echo "synced."

# Default workflow: generate → sync-legacy → rules → lint-rules.
# Bump is gone - render is annotated by the current git SHA dirty status.
.PHONY: release
release: generate sync-legacy rules lint-rules

# Render `rules/*.jsonnet` to $(RULES_OUT_DIR) as yaml. The jsonnet source
# must top-level call std.manifestYamlDoc(...) (with `quote_keys=false` so
# keys don't all get quoted), and the make rule passes `-S` so jsonnet
# emits the manifest as plain text instead of JSON-encoding the string.
.PHONY: rules
rules: $(RULES_OUT_DIR)
	@mkdir -p $(RULES_OUT_DIR)
	@set -e; \
	for src in $(RULES_SRC_DIR)/*.jsonnet; do \
	  [ -f "$$src" ] || continue; \
	  base=$$(basename "$$src" .jsonnet); \
	  out="$(RULES_OUT_DIR)/$$base-rules.yaml"; \
	  echo "==> $$src -> $$out"; \
	  jsonnet -S -J ../tmp/grafonnet-vendor -V version=$(VERSION) "$$src" \
	    > "$$out"; \
	done

.PHONY: lint-rules
lint-rules:
	@for f in $(RULES_OUT_DIR)/*.yaml; do \
	  [ -f "$$f" ] || continue; \
	  echo "==> $$f"; \
	  $(PROMTOOL) check rules "$$f"; \
	done

$(DASHBOARDS_DIR):
	mkdir -p $@

$(RULES_OUT_DIR):
	mkdir -p $@
