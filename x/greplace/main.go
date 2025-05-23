package main

import (
	"fmt"
)

// Special character constants for pattern matching
const (
	SpaceChar   = 256 // \b - word boundary
	StartOfLine = 257 // \^ - start of line
	EndOfLine   = 258 // \$ - end of line
	LastChar    = 259 // sentinel
)

// Pattern represents a from->to replacement pair with metadata
type Pattern struct {
	From   string
	To     string
	Length int // effective length after processing escapes
}

// State represents a DFA state
type State struct {
	// Transition table: next[char] -> next state index
	Next [256]int

	// For final states: which pattern matched and replacement info
	IsFound       bool
	PatternIndex  int // which pattern this matches
	ReplaceString string
	ToOffset      int // how far back to start replacement
	FromOffset    int // how far forward to continue after replacement
}

// PatternProcessor handles the parsing and preprocessing of patterns
type PatternProcessor struct {
	patterns []Pattern
}

// NewPatternProcessor creates a new pattern processor
func NewPatternProcessor() *PatternProcessor {
	return &PatternProcessor{
		patterns: make([]Pattern, 0),
	}
}

// AddPattern adds a from->to replacement pattern
func (pp *PatternProcessor) AddPattern(from, to string) {
	pattern := Pattern{
		From:   from,
		To:     to,
		Length: pp.calculateEffectiveLength(from),
	}
	pp.patterns = append(pp.patterns, pattern)
}

// calculateEffectiveLength calculates the effective length after processing escape sequences
func (pp *PatternProcessor) calculateEffectiveLength(str string) int {
	length := 0
	for i := 0; i < len(str); i++ {
		if str[i] == '\\' && i+1 < len(str) {
			i++ // skip escaped character
		}
		length++
	}
	return length
}

// ProcessSpecialChar converts escape sequences to internal character codes
func (pp *PatternProcessor) ProcessSpecialChar(char byte, nextChar byte) int {
	if char != '\\' {
		return int(char)
	}

	switch nextChar {
	case 'b':
		return SpaceChar
	case '^':
		return StartOfLine
	case '$':
		return EndOfLine
	case 'r':
		return int('\r')
	case 't':
		return int('\t')
	case 'v':
		return int('\v')
	default:
		return int(nextChar)
	}
}

// GetPatterns returns the current patterns
func (pp *PatternProcessor) GetPatterns() []Pattern {
	return pp.patterns
}

// Example usage and testing
func main() {
	// Test BitSet
	fmt.Println("Testing BitSet:")
	bs := NewBitSet(100)
	bs.Set(5)
	bs.Set(10)
	bs.Set(95)

	fmt.Printf("Bit 5 set: %v\n", bs.IsSet(5))
	fmt.Printf("Bit 7 set: %v\n", bs.IsSet(7))

	// Test iteration
	fmt.Print("Set bits: ")
	pos := -1
	for {
		pos = bs.NextBit(pos)
		if pos == -1 {
			break
		}
		fmt.Printf("%d ", pos)
	}
	fmt.Println()

	// Test PatternProcessor
	fmt.Println("\nTesting PatternProcessor:")
	pp := NewPatternProcessor()
	pp.AddPattern("hello", "hi")
	pp.AddPattern("\\bworld\\b", "universe")
	pp.AddPattern("\\^start", "beginning")

	for i, pattern := range pp.GetPatterns() {
		fmt.Printf("Pattern %d: '%s' -> '%s' (length: %d)\n",
			i, pattern.From, pattern.To, pattern.Length)
	}
}
