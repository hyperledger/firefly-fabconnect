VGO=go
BINARY_NAME=fabconnect
GOFILES := $(shell find cmd internal -name '*.go' -print)
TESTED_INTERNALS := $(shell go list ./internal/... | grep -v test | grep -v kafka)
TESTED_CMD := $(shell go list ./cmd/...)
COVERPKG_INTERNALS = $(shell go list ./internal/... | grep -v test | grep -v kafka | tr "\n" ",")
COVERPKG_CMD = $(shell go list ./cmd/... | tr "\n" ",")
# Expect that FireFly compiles with CGO disabled
CGO_ENABLED=0
GOGC=30
.DELETE_ON_ERROR:

all: build test go-mod-tidy
test: deps lint
	$(VGO) test $(TESTED_INTERNALS) $(TESTED_CMD) -cover -coverpkg $(COVERPKG_INTERNALS)$(COVERPKG_CMD) -coverprofile=coverage.txt -covermode=atomic -timeout=10s
coverage.html:
	$(VGO) tool cover -html=coverage.txt
coverage: test coverage.html
lint: ${LINT}
		GOGC=20 $(LINT) run -v --timeout 5m
${LINT}:
		$(VGO) install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.47.3
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
	$(VGO) get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.53.3
deps: builddeps
	$(VGO) get
mockery: .ALWAYS
	go get github.com/vektra/mockery/cmd/mockery
mocks: mockery ${GOFILES}
	$(eval MOCKERY := $(shell go list -f '{{.Target}}' github.com/vektra/mockery/cmd/mockery))
	${MOCKERY} --case underscore --dir internal/events --name SubscriptionManager --output mocks/events --outpkg mockevents
	${MOCKERY} --case underscore --dir internal/fabric/client --name RPCClient --output mocks/fabric/client --outpkg mockfabric
	${MOCKERY} --case underscore --dir internal/fabric/client --name IdentityClient --output mocks/fabric/client --outpkg mockfabric
	${MOCKERY} --case underscore --dir internal/fabric/dep --name CAClient --output mocks/fabric/dep --outpkg mockfabricdep
	${MOCKERY} --case underscore --dir internal/kvstore --name KVStore --output mocks/kvstore --outpkg mockkvstore
	${MOCKERY} --case underscore --dir internal/kvstore --name KVIterator --output mocks/kvstore --outpkg mockkvstore
	${MOCKERY} --case underscore --dir internal/rest/async --name Dispatcher --output mocks/rest/async --outpkg mockasync
	${MOCKERY} --case underscore --dir internal/rest/identity --name Client --output mocks/rest/identity --outpkg mockidentity
	${MOCKERY} --case underscore --dir internal/rest/receipt --name Store --output mocks/rest/receipt --outpkg mockreceipt
	${MOCKERY} --case underscore --dir internal/rest/receipt/api --name ReceiptStorePersistence --output mocks/rest/receipt/api --outpkg mockreceiptapi
	${MOCKERY} --case underscore --dir internal/rest/sync --name Dispatcher --output mocks/rest/sync --outpkg mocksync
	${MOCKERY} --case underscore --dir internal/tx --name Processor --output mocks/tx --outpkg mocktx
	${MOCKERY} --case underscore --dir internal/ws --name WebSocketServer --output mocks/ws --outpkg mockws
	${MOCKERY} --case underscore --dir internal/ws --name WebSocketChannels --output mocks/ws --outpkg mockws
