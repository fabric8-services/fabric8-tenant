PROJECT_NAME=fabric8-tenant
PACKAGE_NAME := github.com/fabric8-services/fabric8-tenant
CUR_DIR=$(shell pwd)
TMP_PATH=$(CUR_DIR)/tmp
INSTALL_PREFIX=$(CUR_DIR)/bin
VENDOR_DIR=vendor
ifeq ($(OS),Windows_NT)
include ./.make/Makefile.win
else
include ./.make/Makefile.lnx
endif
SOURCE_DIR ?= .
SOURCES := $(shell find $(SOURCE_DIR) -path $(SOURCE_DIR)/vendor -prune -o -name '*.go' -print)
DESIGN_DIR=design
DESIGNS := $(shell find $(SOURCE_DIR)/$(DESIGN_DIR) -path $(SOURCE_DIR)/vendor -prune -o -name '*.go' -print)
TEMPLATES := $(shell find $(SOURCE_DIR)/environment/templates -type f)

# Find all required tools:
GIT_BIN := $(shell command -v $(GIT_BIN_NAME) 2> /dev/null)
DEP_BIN := $(GOPATH)/bin/$(DEP_BIN_NAME)
GO_BIN := $(shell command -v $(GO_BIN_NAME) 2> /dev/null)
DOCKER_COMPOSE_BIN := $(shell command -v $(DOCKER_COMPOSE_BIN_NAME) 2> /dev/null)
DOCKER_BIN := $(shell command -v $(DOCKER_BIN_NAME) 2> /dev/null)

# This is a fix for a non-existing user in passwd file when running in a docker
# container and trying to clone repos of dependencies
GIT_COMMITTER_NAME ?= "user"
GIT_COMMITTER_EMAIL ?= "user@example.com"
export GIT_COMMITTER_NAME
export GIT_COMMITTER_EMAIL

COMMIT=$(shell git rev-parse HEAD)
GITUNTRACKEDCHANGES := $(shell git status --porcelain --untracked-files=no)
ifneq ($(GITUNTRACKEDCHANGES),)
COMMIT := $(COMMIT)
endif
BUILD_TIME=`date -u '+%Y-%m-%dT%H:%M:%SZ'`

# For the global "clean" target all targets in this variable will be executed
CLEAN_TARGETS =

# Pass in build time variables to main
LDFLAGS_FOR_TEMPLATES=$(foreach template-path, $(TEMPLATES), $(call set-latest-commit-sha,$(template-path)))
LDFLAGS=-ldflags "-X ${PACKAGE_NAME}/controller.Commit=${COMMIT} -X ${PACKAGE_NAME}/controller.BuildTime=${BUILD_TIME} $(LDFLAGS_FOR_TEMPLATES)"

define set-latest-commit-sha
-X ${PACKAGE_NAME}/environment.$(call get-variable-name, $(1))=$(shell git log -n 1 --pretty=format:%h -- $(1))
endef

define get-variable-name
Version$(shell echo $(notdir $(basename $(1))) | sed -r 's/(^|-)([a-z])/\U\2/g')File
endef

# Call this function with $(call log-info,"Your message")
define log-info =
@echo "INFO: $(1)"
endef

# If nothing was specified, run all targets as if in a fresh clone
.PHONY: all
## Default target - fetch dependencies, generate code and build.
all: prebuild-check deps generate build

.PHONY: help
# Based on https://gist.github.com/rcmachado/af3db315e31383502660
## Display this help text.
help:/
	$(info Available targets)
	$(info -----------------)
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
		helpMessage = match(lastLine, /^## (.*)/); \
		helpCommand = substr($$1, 0, index($$1, ":")-1); \
		if (helpMessage) { \
			helpMessage = substr(lastLine, RSTART + 3, RLENGTH); \
			gsub(/##/, "\n                                     ", helpMessage); \
		} else { \
			helpMessage = "(No documentation)"; \
		} \
		printf "%-35s - %s\n", helpCommand, helpMessage; \
		lastLine = "" \
	} \
	{ hasComment = match(lastLine, /^## (.*)/); \
          if(hasComment) { \
            lastLine=lastLine$$0; \
	  } \
          else { \
	    lastLine = $$0 \
          } \
        }' $(MAKEFILE_LIST)

.PHONY: check-go-format
## Exists with an error if there are files whose formatting differs from gofmt's
check-go-format: prebuild-check
	@gofmt -s -l ${SOURCES} 2>&1 \
		| tee /tmp/gofmt-errors \
		| read \
	&& echo "ERROR: These files differ from gofmt's style (run 'make format-go-code' to fix this):" \
	&& cat /tmp/gofmt-errors \
	&& exit 1 \
	|| true

.PHONY: analyze-go-code
## Run a complete static code analysis using the following tools: golint, gocyclo and go-vet.
analyze-go-code: golint gocyclo govet

## Run gocyclo analysis over the code.
golint: $(GOLINT_BIN)
	$(info >>--- RESULTS: GOLINT CODE ANALYSIS ---<<)
	@$(foreach d,$(GOANALYSIS_DIRS),$(GOLINT_BIN) $d 2>&1 | grep -vEf .golint_exclude;)

## Run gocyclo analysis over the code.
gocyclo: $(GOCYCLO_BIN)
	$(info >>--- RESULTS: GOCYCLO CODE ANALYSIS ---<<)
	@$(foreach d,$(GOANALYSIS_DIRS),$(GOCYCLO_BIN) -over 10 $d | grep -vEf .golint_exclude;)

## Run go vet analysis over the code.
govet:
	$(info >>--- RESULTS: GO VET CODE ANALYSIS ---<<)
	@$(foreach d,$(GOANALYSIS_DIRS),go tool vet --all $d/*.go 2>&1;)

.PHONY: format-go-code
## Formats any go file that differs from gofmt's style
format-go-code: prebuild-check
	@gofmt -s -l -w ${SOURCES}

.PHONY: build
## Build server and client.
build: prebuild-check deps generate $(BINARY_SERVER_BIN) # do the build

$(BINARY_SERVER_BIN): $(SOURCES)
ifeq ($(OS),Windows_NT)
	go build -v ${LDFLAGS} -o "$(shell cygpath --windows '$(BINARY_SERVER_BIN)')"
else
	go build -v ${LDFLAGS} -o ${BINARY_SERVER_BIN}
endif

# Build go tool to analysis the code
$(GOLINT_BIN):
	cd $(VENDOR_DIR)/github.com/golang/lint/golint && go build -v
$(GOCYCLO_BIN):
	cd $(VENDOR_DIR)/github.com/fzipp/gocyclo && go build -v

# Pack all migration SQL files into a compilable Go file
migration/sqlbindata.go: $(GO_BINDATA_BIN) $(wildcard migration/sql-files/*.sql)
	$(GO_BINDATA_BIN) \
		-o migration/sqlbindata.go \
		-pkg migration \
		-prefix migration/sql-files \
		-nocompress \
		migration/sql-files

environment/generated/templates.go: $(GO_BINDATA_BIN) $(wildcard environment/templates/*.yml)
	$(GO_BINDATA_BIN) \
		-o environment/generated/templates.go \
		-pkg templates \
		-prefix 'environment/templates' \
		-nocompress \
		environment/templates

# install dep (see https://golang.github.io/dep/docs/installation.html)
$(DEP_BIN):
	@echo "Installing 'dep' in $(GOPATH)/bin"
	@mkdir -p $(GOPATH)/bin
	curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

# These are binary tools from our vendored packages
$(GOAGEN_BIN): $(VENDOR_DIR)
	cd $(VENDOR_DIR)/github.com/goadesign/goa/goagen && go build -v
$(GO_BINDATA_BIN): $(VENDOR_DIR)
	cd $(VENDOR_DIR)/github.com/jteeuwen/go-bindata/go-bindata && go build -v
$(GO_BINDATA_ASSETFS_BIN): $(VENDOR_DIR)
	cd $(VENDOR_DIR)/github.com/elazarl/go-bindata-assetfs/go-bindata-assetfs && go build -v
$(FRESH_BIN): $(VENDOR_DIR)
	cd $(VENDOR_DIR)/github.com/pilu/fresh && go build -v
$(GO_JUNIT_BIN): $(VENDOR_DIR)
	cd $(VENDOR_DIR)/github.com/jstemmer/go-junit-report && go build -v

CLEAN_TARGETS += clean-artifacts
.PHONY: clean-artifacts


     
## Removes the ./bin directory.
clean-artifacts:
	-rm -rf $(INSTALL_PREFIX)

CLEAN_TARGETS += clean-object-files
.PHONY: clean-object-files
## Runs go clean to remove any executables or other object files.
clean-object-files:
	go clean ./...

CLEAN_TARGETS += clean-generated
.PHONY: clean-generated
## Removes all generated code.
clean-generated:
	-rm -rf ./app
	-rm -rf ./client
	-rm -rf ./swagger/
	-rm -f ./migration/sqlbindata.go
	-rm -f ./environment/generated/templates.go
	-rm -rf ./auth/client

CLEAN_TARGETS += clean-vendor
.PHONY: clean-vendor
## Removes the ./vendor directory.
clean-vendor:
	-rm -rf $(VENDOR_DIR)

$(VENDOR_DIR): Gopkg.lock Gopkg.toml
	$(DEP_BIN) ensure
	touch $(VENDOR_DIR)

.PHONY: deps
## Download build dependencies.
deps: $(VENDOR_DIR)

app/controllers.go: $(DESIGNS) $(GOAGEN_BIN) $(VENDOR_DIR)
	$(GOAGEN_BIN) app -d ${PACKAGE_NAME}/${DESIGN_DIR}
	$(GOAGEN_BIN) controller -d ${PACKAGE_NAME}/${DESIGN_DIR} -o controller/ --pkg controller --app-pkg app
	$(GOAGEN_BIN) client -d ${PACKAGE_NAME}/${DESIGN_DIR}
	$(GOAGEN_BIN) swagger -d ${PACKAGE_NAME}/${DESIGN_DIR}
	$(GOAGEN_BIN) client -d github.com/fabric8-services/fabric8-auth/design --notool --out auth --pkg client 
	
.PHONY: migrate-database
## Compiles the server and runs the database migration with it
migrate-database: $(BINARY_SERVER_BIN)
	 F8_POSTGRES_DATABASE=postgres $(BINARY_SERVER_BIN) -migrateDatabase

.PHONY: generate
## Generate GOA sources. Only necessary after clean of if changed `design` folder.
generate: app/controllers.go migration/sqlbindata.go environment/generated/templates.go

.PHONY: dev
dev: prebuild-check deps generate $(FRESH_BIN)
	docker-compose up -d db
	F8_DEVELOPER_MODE_ENABLED=true $(FRESH_BIN)

include ./.make/test.mk

ifneq ($(OS),Windows_NT)
ifdef DOCKER_BIN
include ./.make/docker.mk
endif
endif

$(INSTALL_PREFIX):
# Build artifacts dir
	mkdir -p $(INSTALL_PREFIX)

$(TMP_PATH):
	mkdir -p $(TMP_PATH)

.PHONY: prebuild-check
prebuild-check: $(TMP_PATH) $(INSTALL_PREFIX) $(CHECK_GOPATH_BIN) $(DEP_BIN)
# Check that all tools where found
ifndef GIT_BIN
	$(error The "$(GIT_BIN_NAME)" executable could not be found in your PATH)
endif
	@$(CHECK_GOPATH_BIN) -packageName=$(PACKAGE_NAME) || (echo "Project lives in wrong location"; exit 1)

$(CHECK_GOPATH_BIN): .make/check_gopath.go
ifndef GO_BIN
	$(error The "$(GO_BIN_NAME)" executable could not be found in your PATH)
endif
ifeq ($(OS),Windows_NT)
	@go build -o "$(shell cygpath --windows '$(CHECK_GOPATH_BIN)')" .make/check_gopath.go
else
	@go build -o $(CHECK_GOPATH_BIN) .make/check_gopath.go
endif

.PHONY: release
release: all

# Keep this "clean" target here at the bottom
.PHONY: clean
## Runs all clean-* targets.
clean: $(CLEAN_TARGETS)


bin/docker: Dockerfile.dev
	mkdir -p bin/docker
	cp Dockerfile.dev bin/docker/Dockerfile

bin/docker/fabric8-tenant-linux: bin/docker $(SOURCES)
	GO15VENDOREXPERIMENT=1 GOARCH=amd64 GOOS=linux go build -o bin/docker/fabric8-tenant-linux

fast-docker: bin/docker/fabric8-tenant-linux
	docker build -t fabric8/fabric8-tenant:dev bin/docker

kube-redeploy: fast-docker
	kubectl delete pod -l service=init-tenant
