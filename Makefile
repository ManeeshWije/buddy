all: test vet fmt build
.PHONY: build-lambda build-cli run-cli clean

build-lambda:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bootstrap ./cmd/buddy/main.go
	zip lambda.zip bootstrap

test:
	go test ./...

vet:
	go vet ./...

fmt:
	go list -f '{{.Dir}}' ./... | grep -v /vendor/ | xargs -L1 gofmt -l
	test -z $$(go list -f '{{.Dir}}' ./... | grep -v /vendor/ | xargs -L1 gofmt -l)

build:
	go build -o bin/buddy ./cmd/buddy
	chmod +x bin/buddy
	go build -o bin/cli ./cmd/cli
	chmod +x bin/cli

clean:
	rm -f bin/buddy bin/cli main lambda.zip bootstrap

# Helper targets
cli: build-cli
	@./bin/cli -api $(API_URL) -key $(API_KEY)

deploy: build-lambda
	terraform apply
