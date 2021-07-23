VGO=go
BINARY_NAME=fabconnect
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
mockery: .ALWAYS
	go get github.com/vektra/mockery/cmd/mockery
mocks: mockery ${GOFILES}
	$(eval MOCKERY := $(shell go list -f '{{.Target}}' github.com/vektra/mockery/cmd/mockery))
	${MOCKERY} --case underscore --dir internal/fabric --name RPCClient --output mocks/fabric --outpkg mockfabric
	${MOCKERY} --case underscore --dir internal/rest/async --name AsyncDispatcher --output mocks/rest/async --outpkg mockasync
	${MOCKERY} --case underscore --dir internal/rest/receipt --name ReceiptStore --output mocks/rest/receipt --outpkg mockreceipt
	${MOCKERY} --case underscore --dir internal/rest/sync --name SyncDispatcher --output mocks/rest/sync --outpkg mocksync
	${MOCKERY} --case underscore --dir internal/tx --name TxProcessor --output mocks/tx --outpkg mocktx
	${MOCKERY} --case underscore --dir internal/ws --name WebSocketServer --output mocks/ws --outpkg mockws
