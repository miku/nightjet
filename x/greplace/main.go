package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Simple replacement structure - no complex DFA needed for now
type SimpleReplacer struct {
	fromStrings []string
	toStrings   []string
}

func NewSimpleReplacer(from, to []string) *SimpleReplacer {
	if len(from) != len(to) {
		panic("from and to arrays must have same length")
	}
	return &SimpleReplacer{
		fromStrings: from,
		toStrings:   to,
	}
}

// Replace all occurrences in the input string
func (r *SimpleReplacer) Replace(input string) string {
	result := input
	for i := 0; i < len(r.fromStrings); i++ {
		result = strings.ReplaceAll(result, r.fromStrings[i], r.toStrings[i])
	}
	return result
}

// Process line by line from reader to writer
func (r *SimpleReplacer) ProcessStream(reader io.Reader, writer io.Writer) error {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		replaced := r.Replace(line)
		if _, err := io.WriteString(writer, replaced+"\n"); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func main() {
	// Simple argument parsing: pairs of from/to strings
	args := os.Args[1:]

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: %s from1 to1 [from2 to2 ...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Reads from stdin and writes to stdout\n")
		os.Exit(1)
	}

	if len(args)%2 != 0 {
		fmt.Fprintf(os.Stderr, "Error: Arguments must be pairs of from/to strings\n")
		os.Exit(1)
	}

	// Build from and to arrays
	var fromStrings, toStrings []string
	for i := 0; i < len(args); i += 2 {
		fromStrings = append(fromStrings, args[i])
		toStrings = append(toStrings, args[i+1])
	}

	// Create replacer and process stdin to stdout
	replacer := NewSimpleReplacer(fromStrings, toStrings)
	if err := replacer.ProcessStream(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error processing input: %v\n", err)
		os.Exit(1)
	}
}
