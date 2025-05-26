main: main.go
	go build -o main main.go

digest.txt: $(GO_FILES) Makefile
	uvx gitingest -o digest.txt -e testdata/ -e attic/

.PHONY: clean
clean:
	rm -f main
	rm -f digest.txt

