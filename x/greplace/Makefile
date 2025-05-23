GO_FILES = $(shell find . -name "*.go" -type f)
SHELL = /bin/bash

main: main.go
	go build -o main main.go bitset.go

.PHONY: clean
clean:
	rm -f main

digest.txt: $(GO_FILES)
	uvx gitingest -o digest.txt


