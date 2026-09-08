SPECS_DIR := specs
RULES_DIR := rules
KIND_DIR  := kind
REPO_ROOT := $(shell git rev-parse --show-toplevel 2>/dev/null || echo ../..)
PROMTOOL := $(shell mise which promtool 2>/dev/null || command -v promtool 2>/dev/null || echo promtool)

.PHONY: verify
verify:
	cd $(REPO_ROOT) && go run . load -f examples/oteldemo/$(SPECS_DIR)/ -r

.PHONY: generate
generate:
	mkdir -p $(RULES_DIR)
	cd $(REPO_ROOT) && go run . generate -f examples/oteldemo/$(SPECS_DIR)/ -r -o examples/oteldemo/$(RULES_DIR)
	$(MAKE) lint-rules

.PHONY: lint-rules
lint-rules:
	@for f in $(RULES_DIR)/*.yaml; do \
		echo "==> $$f"; \
		$(PROMTOOL) check rules "$$f"; \
	done

.PHONY: clean
clean:
	rm -rf $(RULES_DIR)

.PHONY: start-demo
start-demo:
	$(KIND_DIR)/setup.sh

.PHONY: stop-demo
stop-demo:
	$(KIND_DIR)/teardown.sh

.PHONY: sync
sync:
	$(KIND_DIR)/sync.sh
