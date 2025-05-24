SHELL = /bin/bash
GO_FILES = $(shell find . -name "*.go" -type f)

greplace: $(GO_FILES)
	go build -o greplace greplace.go bitset.go pattern.go dfa.go replace.go

.PHONY: test
test:
	go test -cover -v .

.PHONY: clean
clean:
	rm -f greplace

digest.txt: $(GO_FILES)
	uvx gitingest -o digest.txt


