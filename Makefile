VGO=go
BINARY_NAME=fabcon
GOFILES := $(shell find cmd internal -name '*.go' -print)
# Expect that FireFly compiles with CGO disabled
CGO_ENABLED=0
GOGC=30
.DELETE_ON_ERROR:

all: build test go-mod-tidy
test: deps lint
		$(VGO) test ./internal/... ./cmd/... -cover -coverprofile=coverage.txt -covermode=atomic -timeout=10s
coverage.html:
		$(VGO) tool cover -html=coverage.txt
coverage: test coverage.html
lint:
		GOGC=20 $(shell go list -f '{{.Target}}' github.com/golangci/golangci-lint/cmd/golangci-lint) run -v --timeout 5m
firefly-nocgo: ${GOFILES}		
		CGO_ENABLED=0 $(VGO) build -o ${BINARY_NAME}-nocgo -ldflags "-X main.buildDate=`date -u +\"%Y-%m-%dT%H:%M:%SZ\"` -X main.buildVersion=$(BUILD_VERSION)" -tags=prod -tags=prod -v
firefly: ${GOFILES}
		$(VGO) build -o ${BINARY_NAME} -ldflags "-X main.buildDate=`date -u +\"%Y-%m-%dT%H:%M:%SZ\"` -X main.buildVersion=$(BUILD_VERSION)" -tags=prod -tags=prod -v
go-mod-tidy: .ALWAYS
		go mod tidy
build: firefly-nocgo firefly
.ALWAYS: ;
clean: 
		$(VGO) clean
		rm -f *.so ${BINARY_NAME}
builddeps:
		$(VGO) get github.com/golangci/golangci-lint/cmd/golangci-lint
deps: builddeps
		$(VGO) get
