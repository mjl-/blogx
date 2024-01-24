run: build
	./blogx serve local/blogx.conf

build:
	CGO_ENABLED=0 go build

clean:
	CGO_ENABLED=0 go clean

test:
	CGO_ENABLED=1 go test -race -cover

fmt:
	gofmt -w -s *.go
