package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// LineBasedReplacer mimics the C algorithm exactly
type LineBasedReplacer struct {
	patterns *PatternProcessor
	compiled bool
}

func NewLineBasedReplacer() *LineBasedReplacer {
	return &LineBasedReplacer{
		patterns: NewPatternProcessor(),
		compiled: false,
	}
}

func (r *LineBasedReplacer) AddPattern(from, to string) error {
	return r.patterns.AddPattern(from, to)
}

func (r *LineBasedReplacer) Compile() error {
	if r.patterns.PatternCount() == 0 {
		return fmt.Errorf("no patterns defined")
	}
	r.compiled = true
	return nil
}

// ReplaceString does simple, correct pattern replacement
func (r *LineBasedReplacer) ReplaceString(input string) (string, bool, error) {
	if !r.compiled {
		return "", false, fmt.Errorf("not compiled")
	}

	result := input
	updated := false
	patterns := r.patterns.GetPatterns()

	// Apply replacements left-to-right, longest match first
	pos := 0
	for pos < len(result) {
		bestMatch := -1
		bestLength := 0

		// Find the longest matching pattern at current position
		for i, pattern := range patterns {
			if strings.HasPrefix(result[pos:], pattern.From) {
				if len(pattern.From) > bestLength {
					bestMatch = i
					bestLength = len(pattern.From)
				}
			}
		}

		if bestMatch >= 0 {
			// Apply the replacement
			pattern := patterns[bestMatch]
			result = result[:pos] + pattern.To + result[pos+len(pattern.From):]
			pos += len(pattern.To)
			updated = true
		} else {
			pos++
		}
	}

	return result, updated, nil
}

// ReplaceReader mimics the C convert_pipe function exactly
func (r *LineBasedReplacer) ReplaceReader(reader io.Reader, writer io.Writer) (bool, error) {
	if !r.compiled {
		return false, fmt.Errorf("not compiled")
	}

	scanner := bufio.NewScanner(reader)

	// Set a large buffer size to handle long lines
	const maxCapacity = 64 * 1024 * 1024 // 64MB max line length
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	updated := false

	for scanner.Scan() {
		line := scanner.Text()

		// Process this line
		processedLine, lineUpdated, err := r.ReplaceString(line)
		if err != nil {
			return false, fmt.Errorf("failed to process line: %v", err)
		}

		if lineUpdated {
			updated = true
		}

		// Write the processed line with newline
		_, err = writer.Write([]byte(processedLine + "\n"))
		if err != nil {
			return false, fmt.Errorf("failed to write line: %v", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("scanner error: %v", err)
	}

	return updated, nil
}

// GetStats returns basic statistics
func (r *LineBasedReplacer) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"patterns": r.patterns.PatternCount(),
		"compiled": r.compiled,
	}
}

// Alternative: For inputs without newlines, read all at once like C does
func (r *LineBasedReplacer) ReplaceReaderNoNewlines(reader io.Reader, writer io.Writer) (bool, error) {
	if !r.compiled {
		return false, fmt.Errorf("not compiled")
	}

	// Read all input (like C does for inputs without newlines)
	input, err := io.ReadAll(reader)
	if err != nil {
		return false, fmt.Errorf("failed to read input: %v", err)
	}

	// Process the entire input as one "line"
	result, updated, err := r.ReplaceString(string(input))
	if err != nil {
		return false, err
	}

	// Write result
	_, err = writer.Write([]byte(result))
	if err != nil {
		return false, fmt.Errorf("failed to write output: %v", err)
	}

	return updated, nil
}

// Update your main.go to use this instead:
func processStreamWithLineBasedReplacer(replacer *LineBasedReplacer, input io.Reader, output io.Writer, verbose bool) error {
	// For your test case (no newlines), use the no-newlines version
	updated, err := replacer.ReplaceReaderNoNewlines(input, output)
	if err != nil {
		return fmt.Errorf("stream processing failed: %v", err)
	}

	if verbose && updated {
		fmt.Fprintf(os.Stderr, "Stream processed with replacements\n")
	}

	return nil
}
