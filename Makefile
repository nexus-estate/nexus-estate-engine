APP_NAME=nexus-estate-search-service

ENV_FILE ?= .env.develop
PROFILE ?= develop

.PHONY: proto
proto:
	protoc \
		--go_out=. \
		--go_opt=module=github.com/NexusEstate/nexus-estate-search-service \
		--go-grpc_out=. \
		--go-grpc_opt=module=github.com/NexusEstate/nexus-estate-search-service \
		proto/search/v1/search.proto

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: run
run:
	go run ./cmd/server

.PHONY: build
build:
	go build -trimpath -ldflags="-s -w" -o bin/$(APP_NAME) ./cmd/server

.PHONY: test
test:
	go test ./...

.PHONY: docker-build-dev
docker-build-dev:
	docker build --target development -t $(APP_NAME):develop .

.PHONY: docker-build-release
docker-build-release:
	docker build --target release -t $(APP_NAME):release .

.PHONY: docker-build-prod
docker-build-prod:
	docker build --target production -t $(APP_NAME):production .

.PHONY: dev
dev:
	docker compose --env-file .env --profile develop up -d --build

.PHONY: dev-logs
dev-logs:
	docker compose --env-file .env --profile develop logs -f search-service

.PHONY: release
release:
	docker compose --env-file .env --profile release up -d --build

.PHONY: release-logs
release-logs:
	docker compose --env-file .env --profile release logs -f search-service-release

.PHONY: prod
prod:
	docker compose --env-file .env --profile production up -d --build

.PHONY: prod-logs
prod-logs:
	docker compose --env-file .env --profile production logs -f search-service-production

.PHONY: down
down:
	docker compose --profile develop --profile release --profile production down

.PHONY: down-volume
down-volume:
	docker compose --profile develop --profile release --profile production down -v

.PHONY: ps
ps:
	docker compose --profile develop --profile release --profile production ps

.PHONY: grpc-list
grpc-list:
	grpcurl -plaintext localhost:50052 list

.PHONY: grpc-describe
grpc-describe:
	grpcurl -plaintext localhost:50052 describe nexusestate.search.v1.SearchService