.PHONY: help ent interfaces migrate tests fmt check

help:
	@echo "make ent                 - Generate ent from ent/schema"
	@echo "make interfaces          - Generate the interfaces (ifacemaker) and mocks (mockery)"
	@echo "make migrate NAME=x      - Create a goose migration in ent/migrate/migrations"
	@echo "make tests               - Run the tests (needs the compose Postgres or DATABASE_URL)"
	@echo "make fmt                 - Format the Go code"
	@echo "make check               - gofmt, vet and tests, as CI runs them"

ent:
	go generate ./ent/...

interfaces:
	go generate ./internal/repositories/... ./internal/services/...
	go tool github.com/vektra/mockery/v3

migrate:
	@if [ -z "$(NAME)" ]; then echo "Usage: make migrate NAME=what_changed"; exit 1; fi
	go tool github.com/pressly/goose/v3/cmd/goose -dir ent/migrate/migrations create $(NAME) sql

tests:
	go test ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './ent/*' -not -path './mocks/*')

check:
	@out="$$(gofmt -l $$(find . -name '*.go' -not -path './ent/*' -not -path './mocks/*'))"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	go vet ./...
	go test ./...
