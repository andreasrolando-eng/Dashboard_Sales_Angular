# Wajib: `make check` lulus sebelum setiap push (dijalankan otomatis oleh hook pre-push).
# `make setup` sekali setelah clone untuk memasang git hook.
SHELL := /bin/bash
TEST_DATABASE_URL ?= postgres://app:app@localhost:5432/app_test?sslmode=disable

.PHONY: setup check check-ci api-check migrate-check web-app-check flutter-app-check docker-check test-db

setup:
	git config core.hooksPath .githooks
	chmod +x .githooks/*
	@echo "Git hook terpasang."

## Lokal: semua cek + build image docker (Dockerfile rusak juga ketahuan sebelum push)
check: check-ci docker-check
	@echo "✅ make check lulus — aman untuk push"

## CI: dipakai GitHub Actions (build image dilakukan di job berikutnya)
check-ci: api-check migrate-check web-app-check flutter-app-check

api-check:
	@echo "==> api: format, vet, test, build"
	cd api && test -z "$$(gofmt -l .)" || { echo "gofmt belum dijalankan:"; gofmt -l .; exit 1; }
	cd api && go vet ./...
	cd api && go test ./...
	cd api && go build -o /dev/null ./cmd/server

## up → down-to 0 → up di database TEST terpisah (bukan database development)
migrate-check: test-db
	@echo "==> migration: up / down / up"
	goose -dir api/migrations postgres "$(TEST_DATABASE_URL)" up
	goose -dir api/migrations postgres "$(TEST_DATABASE_URL)" down-to 0
	goose -dir api/migrations postgres "$(TEST_DATABASE_URL)" up

test-db:
	@if [ -z "$$CI" ]; then \
	  docker compose up -d db >/dev/null && sleep 2 && \
	  docker compose exec -T db psql -U app -d postgres -tc "SELECT 1 FROM pg_database WHERE datname='app_test'" | grep -q 1 || \
	  docker compose exec -T db createdb -U app app_test; \
	fi

web-app-check:
	@echo "==> web-app: test, build"
	cd web-app && npm ci --no-audit --no-fund
	cd web-app && npm test -- --watch=false
	cd web-app && npm run build

flutter-app-check:
	@if [ -d flutter-app ]; then \
	  echo "==> flutter-app: analyze, test"; \
	  cd flutter-app && flutter pub get && flutter analyze && flutter test; \
	else echo "==> flutter-app: tidak ada, dilewati"; fi

docker-check:
	@echo "==> docker: build image api & app"
	docker compose build api web-app
