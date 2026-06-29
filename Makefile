.PHONY: default build run

default: build

build:
	mkdir -p .out
	go build -o .out/ucode .

run: build
	./.out/ucode
