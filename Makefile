REGISTRY_DIR := semconv/registry

.PHONY: semconv-generate
semconv-generate:
	weaver registry generate --v2 --registry $(REGISTRY_DIR) --templates ./semconv/templates/ go ./pkg/semconv/
	gofmt -w ./pkg/semconv/semconv_gen.go

.PHONY: semconv-check
semconv-check:
	weaver registry check --v2 -r $(REGISTRY_DIR)

.PHONY: semconv-stats
semconv-stats:
	weaver registry stats -r $(REGISTRY_DIR)

.PHONY: semconv-json
semconv-json:
	weaver registry json-schema -r $(REGISTRY_DIR)

.PHONY: semconv-diff
semconv-diff:
	weaver registry diff --v2 --registry $(REGISTRY_DIR) --baseline-registry $(REGISTRY_DIR)@$(BASE)

.PHONY: build
build:
	go build -o opensloctl .

.PHONY: lint
lint:
	golangci-lint run

.PHONY: test
test:
	go test ./...

.PHONY: generate
generate:
	go run . generate -f $(FILE) -o $(OUTPUT)

.PHONY: load
load:
	go run . load -f $(FILE)

.PHONY: tidy
tidy:
	go mod tidy
