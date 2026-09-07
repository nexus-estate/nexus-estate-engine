GO_VERSION := $(shell awk '/^go / {print $$2}' go.mod)
MODULE := $(shell awk '/^module / {print $$2}' go.mod)
RUNTIMES := search engine worker
PROTOC_VERSION := 36.0
PROTOC_GEN_GO_VERSION := v1.36.11
PROTOC_GEN_GO_GRPC_VERSION := v1.6.2

.PHONY: fmt lint vet test test-race tidy check proto build dev-search down $(addprefix run-,$(RUNTIMES)) $(addprefix build-,$(RUNTIMES)) $(addprefix docker-,$(RUNTIMES))
fmt:
	gofmt -w cmd internal gen
lint:
	golangci-lint run
vet:
	go vet ./...
test:
	go test ./...
test-race:
	go test -race ./...
tidy:
	go mod tidy
check:
	test -z "$$(gofmt -l .)"
	go vet ./...
	golangci-lint run
	go test -race ./...
	go mod tidy
	git diff --exit-code -- go.mod go.sum
proto:
	test "$$(protoc --version)" = "libprotoc $(PROTOC_VERSION)"
	test "$$(protoc-gen-go --version)" = "protoc-gen-go $(PROTOC_GEN_GO_VERSION)"
	test "$$(protoc-gen-go-grpc --version)" = "protoc-gen-go-grpc $(patsubst v%,%,$(PROTOC_GEN_GO_GRPC_VERSION))"
	protoc --go_out=. --go_opt=module=$(MODULE) --go-grpc_out=. --go-grpc_opt=module=$(MODULE) proto/search/v1/search.proto
$(addprefix run-,$(RUNTIMES)): run-%:
	go run ./cmd/$*
$(addprefix build-,$(RUNTIMES)): build-%:
	go build -trimpath -ldflags="-s -w" -o bin/nexus-$* ./cmd/$*
build: $(addprefix build-,$(RUNTIMES))
$(addprefix docker-,$(RUNTIMES)): docker-%:
	docker build --build-arg GO_VERSION=$(GO_VERSION) --target $* -t nexus-$*:local .
dev-search:
	GO_VERSION=$(GO_VERSION) docker compose up -d --build search elasticsearch redis
down:
	docker compose --profile platform down
