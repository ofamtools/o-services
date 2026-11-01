GOENV = GOWORK=off GOFLAGS='-trimpath -p=2' GOMAXPROCS=3
GO = env $(GOENV) nice -n 15 go
TEST_TMP ?= /tmp/o-services-lane/tests

.PHONY: build test check
build:
	$(GO) build -o o-services .
test:
	@mkdir -p $(TEST_TMP)
	env TMPDIR=$(TEST_TMP) $(GO) test $(TEST_ARGS) ./...
check:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }
	$(GO) vet ./...
	@mkdir -p $(TEST_TMP)
	env TMPDIR=$(TEST_TMP) $(GO) test ./...
