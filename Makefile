PACKAGE  = github.com/tumblr/k8s-sidecar-injector
BINARY   = k8s-sidecar-injector
DATE    ?= $(shell date +%FT%T%z)
BRANCH  ?= $(shell git rev-parse --abbrev-ref HEAD)
COMMIT  ?= $(shell git rev-parse HEAD)
VERSION ?= $(shell git describe --tags --always --dirty --match=v* 2> /dev/null || \
			cat $(CURDIR)/.version 2> /dev/null || echo v0)
BIN      = $(GOPATH)/bin
PKGS     = $(or $(PKG),$(shell $(GO) list ./... | grep -v "^$(PACKAGE)/vendor/"))
TESTPKGS = $(shell env $(GO) list -f '{{ if or .TestGoFiles .XTestGoFiles }}{{ .ImportPath }}{{ end }}' $(PKGS))
DOCKER_IMAGE_BASE = tumblr/k8s-sidecar-injector
DOCKER_TAG = $(VERSION)

export CGO_ENABLED := 0

GO      = go
GODOC   = godoc
TIMEOUT = 15
V = 0
Q = $(if $(filter 1,$V),,@)
M = $(shell printf "\033[34;1m▶\033[0m")

.PHONY: all
all: fmt lint vendor | ; $(info $(M) building executable…) @ ## Build program binary
	$Q $(GO) build \
		-tags release \
		-ldflags '-X $(PACKAGE)/internal/pkg/version.Version=$(VERSION) -X $(PACKAGE)/internal/pkg/version.BuildDate=$(DATE) -X $(PACKAGE)/internal/pkg/version.Package=$(PACKAGE) -X $(PACKAGE)/internal/pkg/version.Commit=$(COMMIT) -X $(PACKAGE)/internal/pkg/version.Branch=$(BRANCH)' \
		-o bin/$(BINARY) $(PACKAGE)/cmd \
    && echo "Built bin/$(BINARY): $(VERSION) $(DATE)"

# Tools
#

GOLINT = $(BIN)/golint
$(BIN)/golint: | ; $(info $(M) building golint…)
	$Q go get golang.org/x/lint/golint

GOCOVMERGE = $(BIN)/gocovmerge
$(BIN)/gocovmerge: | ; $(info $(M) building gocovmerge…)
	$Q go get github.com/wadey/gocovmerge

GOCOV = $(BIN)/gocov
$(BIN)/gocov: | ; $(info $(M) building gocov…)
	$Q go get github.com/axw/gocov/...

GOCOVXML = $(BIN)/gocov-xml
$(BIN)/gocov-xml: | ; $(info $(M) building gocov-xml…)
	$Q go get github.com/AlekSi/gocov-xml

GO2XUNIT = $(BIN)/go2xunit
$(BIN)/go2xunit: | ; $(info $(M) building go2xunit…)
	$Q go get github.com/tebeka/go2xunit

# Tests
#

TEST_TARGETS := test-default test-bench test-short test-verbose test-race
.PHONY: $(TEST_TARGETS) test-xml check test tests
test-bench:   ARGS=-run=__absolutelynothing__ -bench=. ## Run benchmarks
test-short:   ARGS=-short        ## Run only short tests
test-verbose: ARGS=-v            ## Run tests in verbose mode with coverage reporting
test-race:    ARGS=-race         ## Run tests with race detector
$(TEST_TARGETS): NAME=$(MAKECMDGOALS:test-%=%)
$(TEST_TARGETS): test
check test tests: fmt lint vendor | ; $(info $(M) running $(NAME:%=% )tests…) @ ## Run tests
	$Q $(GO) test -count=1 -timeout=$(TIMEOUT)s $(ARGS) $(TESTPKGS)

test-xml: fmt lint vendor | $(GO2XUNIT) ; $(info $(M) running $(NAME:%=% )tests…) @ ## Run tests with xUnit output
	$Q 2>&1 $(GO) test -count=1 -timeout=$(TIMEOUT)s -v $(TESTPKGS) | tee test/tests.output
	$(GO2XUNIT) -fail -input test/tests.output -output test/tests.xml

COVERAGE_MODE = atomic
COVERAGE_PROFILE = $(COVERAGE_DIR)/profile.out
COVERAGE_XML = $(COVERAGE_DIR)/coverage.xml
COVERAGE_HTML = $(COVERAGE_DIR)/index.html
.PHONY: test-coverage test-coverage-tools
test-coverage-tools: | $(GOCOVMERGE) $(GOCOV) $(GOCOVXML)
test-coverage: COVERAGE_DIR := $(CURDIR)/test/coverage.$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
test-coverage: fmt lint vendor test-coverage-tools | ; $(info $(M) running coverage tests…) @ ## Run coverage tests
	$Q mkdir -p $(COVERAGE_DIR)/coverage
	$Q for pkg in $(TESTPKGS); do \
		$(GO) test \
			-coverpkg=$$($(GO) list -f '{{ join .Deps "\n" }}' $$pkg | \
					grep '^$(PACKAGE)/' | grep -v '^$(PACKAGE)/vendor/' | \
					tr '\n' ',')$$pkg \
			-covermode=$(COVERAGE_MODE) \
			-coverprofile="$(COVERAGE_DIR)/coverage/`echo $$pkg | tr "/" "-"`.cover" $$pkg ;\
	 done
	$Q $(GOCOVMERGE) $(COVERAGE_DIR)/coverage/*.cover > $(COVERAGE_PROFILE)
	$Q $(GO) tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML)
	$Q $(GOCOV) convert $(COVERAGE_PROFILE) | $(GOCOVXML) > $(COVERAGE_XML)

.PHONY: docker
docker: | ; $(info $(M) building docker container…)
	docker build -t $(DOCKER_IMAGE_BASE):$(DOCKER_TAG) .
	docker build -t $(DOCKER_IMAGE_BASE):latest .

.PHONY: lint
lint: vendor | $(GOLINT) ; $(info $(M) running golint…)
	$Q $(GOLINT) $(PKGS)

.PHONY: fmt
fmt: ; $(info $(M) running gofmt…) @
	$Q $(GO) fmt $(PKGS)

# Dependency management

vendor: go.mod go.sum | ; $(info $(M) retrieving dependencies…)
	$Q $(GO) mod download 2>&1
	$Q $(GO) mod vendor

.PHONY: clean
clean: ; $(info $(M) cleaning…)	@ ## Cleanup everything
	@rm -rf vendor/
	@rm -rf bin
	@rm -rf test/tests.* test/coverage.*

.PHONY: help
help:
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

.PHONY: version
version:
	@echo $(VERSION)
