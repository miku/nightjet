llmexp: cmd/llmexp/main.go
	go build -o llmexp cmd/llmexp/main.go

.PHONY: clean
clean:
	rm -f llmexp
