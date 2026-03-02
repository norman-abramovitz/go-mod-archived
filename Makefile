.PHONY: build test vet fmt clean

build:
	go build -o modrot .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

clean:
	rm -f modrot coverage.out
