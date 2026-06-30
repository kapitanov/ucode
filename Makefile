.PHONY: default build run test test-coverage

default: build test

build:
	mkdir -p .out
	go build -o .out/ucode .

run: build
	./.out/ucode

test:
	go test ./...

