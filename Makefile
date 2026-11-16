.PHONY: check test test-integration verify-public-mvp ci

check:
	@unformatted="$$(find . -type f -name '*.go' -not -path './vendor/*' -exec gofmt -l {} +)"; \
	if [ -n "$$unformatted" ]; then \
		echo "Go files require formatting:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	@packages="$$(go list ./...)"; \
	if [ -n "$$packages" ]; then go vet $$packages; fi

test:
	@packages="$$(go list ./...)"; \
	if [ -n "$$packages" ]; then go test -race -count=1 $$packages; fi

test-integration:
	@set -e; \
	compose_file=tests/integration/compose.yaml; \
	docker compose -f "$$compose_file" up -d --wait; \
	trap 'docker compose -f "$$compose_file" down --volumes --remove-orphans' EXIT; \
	DATAWARK_TEST_POSTGRES_DSN='postgres://datawark_test:datawark_test@127.0.0.1:54329/datawark_test?sslmode=disable' \
		go test -tags=integration -race -count=1 ./tests/integration/...

verify-public-mvp:
	@set -eu; \
	work_dir="$$(mktemp -d)"; \
	project="datawark_mvp_$$(date +%s)_$$$$"; \
	cleanup() { docker compose --env-file "$$work_dir/.env" --project-name "$$project" down --volumes --remove-orphans >/dev/null 2>&1 || true; rm -rf "$$work_dir"; }; \
	trap cleanup EXIT INT TERM; \
	secret() { openssl rand -hex 24; }; \
	{ \
		echo DATAWARK_GATEWAY_PORT=18080; \
		echo DATAWARK_POSTGRES_SUPERUSER=datawark_admin; \
		echo DATAWARK_POSTGRES_SUPERUSER_PASSWORD=$$(secret); \
		echo DATAWARK_POSTGRES_DB=datawark_demo; \
		echo DATAWARK_DEMO_OWNER=datawark_demo_owner; \
		echo DATAWARK_GATEWAY_DB_USER=datawark_gateway_reader; \
		echo DATAWARK_GATEWAY_DB_PASSWORD=$$(secret); \
		echo DATAWARK_LOCAL_API_KEY=$$(secret); \
	} >"$$work_dir/.env"; \
	chmod 600 "$$work_dir/.env"; \
	docker compose --env-file "$$work_dir/.env" --project-name "$$project" up --build --wait; \
	set -a; . "$$work_dir/.env"; set +a; \
	if ! env \
		DATAWARK_PUBLIC_ENDPOINT="http://127.0.0.1:$$DATAWARK_GATEWAY_PORT" \
	DATAWARK_PUBLIC_API_KEY="$$DATAWARK_LOCAL_API_KEY" \
	DATAWARK_PUBLIC_COMPOSE_PROJECT="$$project" \
	DATAWARK_PUBLIC_DB_READER="$$DATAWARK_GATEWAY_DB_USER" \
	DATAWARK_PUBLIC_DB_PASSWORD="$$DATAWARK_GATEWAY_DB_PASSWORD" \
	DATAWARK_PUBLIC_PROTECTED_VALUE="demo-protected-note" \
		go test -count=1 ./tests/security/... >"$$work_dir/release-test.log" 2>&1; then cat "$$work_dir/release-test.log"; exit 1; fi; \
	cat "$$work_dir/release-test.log"; \
	docker compose --env-file "$$work_dir/.env" --project-name "$$project" logs --no-color >"$$work_dir/compose.log" 2>&1; \
	! grep -F -e "$$DATAWARK_LOCAL_API_KEY" -e "$$DATAWARK_GATEWAY_DB_PASSWORD" -e "demo-protected-note" "$$work_dir/compose.log"

ci: check test test-integration
