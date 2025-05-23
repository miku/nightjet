GO_FILES = $(shell find . -name "*.go" -type f)
SHELL = /bin/bash

main: main.go
	go build -o main main.go bitset.go pattern.go dfa.go

.PHONY: test
test:
	go test -cover -v .

.PHONY: clean
clean:
	rm -f main

digest.txt: $(GO_FILES)
	uvx gitingest -o digest.txt


