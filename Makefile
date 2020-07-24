.PHONY: test
test:
	go test -v -covermode=atomic -cover -race -coverprofile=coverage.txt .


.PHONY: checks
checks:
	echo " ! gofmt -d *.go 2>&1 | read " | bash
	go vet ./...

	
.PHONY: upload-coverage
upload-coverage:
	go get github.com/mattn/goveralls
	/go/bin/goveralls -v -coverprofile=coverage.txt -service=drone.io


.PHONY: build
build:
	go build .


.PHONE: drone-tests
drone-tests: test build checks upload-coverage
