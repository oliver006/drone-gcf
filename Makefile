.PHONY: test
test:
	go test -v -covermode=atomic -cover -race -coverprofile=coverage.txt .


.PHONY: vet
vet:
	echo " ! gofmt -d *.go 2>&1 | read " | bash
	go vet ./...

	
.PHONY: upload-coverage
upload-coverage:
	go install github.com/mattn/goveralls@v0.0.11
	/go/bin/goveralls -coverprofile=coverage.txt -service=drone.io


.PHONY: build
build:
	go build .


.PHONY: lint
lint:
        #
        # this will run the default linters on non-test files
        # and then all but the "errcheck" linters on the tests
	golangci-lint run --tests=false --exclude-use-default
	golangci-lint run -D=errcheck   --exclude-use-default


.PHONY: drone-tests
drone-tests: test build vet upload-coverage
