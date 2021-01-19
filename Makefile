.PHONY: deps lint format check-format test test-cover add-license \
	check-license shorten-lines salus validate watch-blocks \
	watch-transactions watch-balances watch-reconciliations \
	view-block-benchmarks view-account-benchmarks mocks

# To run the the following packages as commands,
# it is necessary to use `go run <pkg>`. Running `go get` does
# not install any binaries that could be used to run
# the commands directly.
ADDLICENSE_CMD=go run github.com/google/addlicense
ADDLICENCE_SCRIPT=${ADDLICENSE_CMD} -c "Coinbase, Inc." -l "apache" -v
GOLINES_CMD=go run github.com/segmentio/golines
GOVERALLS_CMD=go run github.com/mattn/goveralls
COVERAGE_TEST_DIRECTORIES=./configuration/... ./pkg/constructor/... \
	./pkg/logger/... ./pkg/scenario/...
TEST_SCRIPT=go test -v ./pkg/... ./configuration/...
COVERAGE_TEST_SCRIPT=go test -v ${COVERAGE_TEST_DIRECTORIES}

deps:
	go get ./...

lint:
	golangci-lint run --timeout 2m0s -v \
		-E golint,misspell,gocyclo,whitespace,goconst,gocritic,gocognit,bodyclose,unconvert,lll,unparam,gomnd;

format:
	gofmt -s -w -l .;

check-format:
	! gofmt -s -l . | read;

validate-configuration-files:
	go run main.go configuration:validate examples/configuration/simple.json;
	go run main.go configuration:create examples/configuration/default.json;
	go run main.go configuration:validate examples/configuration/default.json;
	git diff --exit-code;

test: | validate-configuration-files
	${TEST_SCRIPT}

test-cover:	
	if [ "${COVERALLS_TOKEN}" ]; then ${COVERAGE_TEST_SCRIPT} -coverprofile=c.out -covermode=count; ${GOVERALLS_CMD} -coverprofile=c.out -repotoken ${COVERALLS_TOKEN}; fi

add-license:
	${ADDLICENCE_SCRIPT} .

check-license:
	${ADDLICENCE_SCRIPT} -check .

shorten-lines:
	${GOLINES_CMD} -w --shorten-comments pkg cmd configuration

salus:
	docker run --rm -t -v ${PWD}:/home/repo coinbase/salus

release: add-license shorten-lines format test lint salus

compile:
	./scripts/compile.sh $(version)

build:
	go build ./...

install:
	go install ./...

mocks:
	rm -rf mocks;
	mockery --dir pkg/constructor --all --case underscore --outpkg constructor --output mocks/constructor;
	${ADDLICENCE_SCRIPT} .;
