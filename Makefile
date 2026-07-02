TEST_POSTGRES_URL ?= postgres://uigraph:devpassword@localhost:5432/uigraph?sslmode=disable

.PHONY: test test-unit test-integration build lint openapi

## Run all tests (unit + integration).
test: test-unit test-integration

## Run unit tests only (no database required).
test-unit:
	go test $$(go list ./... | grep -v github.com/uigraph/app/tests) -count=1

## Run integration tests against a real Postgres instance.
test-integration:
	TEST_POSTGRES_URL=$(TEST_POSTGRES_URL) go test ./tests/... -count=1 -v

## Build the binary.
build:
	go build ./...

## Run go vet.
lint:
	go vet ./...

## Generate the OpenAPI spec from swag annotations.
openapi:
	swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal --outputTypes yaml,json
	mv docs/swagger.json docs/openapi.json
	mv docs/swagger.yaml docs/openapi.yaml
