.PHONY: check test test-integration ci

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

ci: check test test-integration
