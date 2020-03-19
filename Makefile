export GOFLAGS=-mod=vendor
export GOPROXY=off
export CGO_ENABLED=0

run: build
	./blogx serve local/blogx.conf

build:
	go build

clean:
	go clean

test:
	CGO_ENABLED=1 go test -race -cover

fmt:
	go fmt ./...
