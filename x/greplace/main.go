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

// BitSet represents a set of states during DFA construction
type BitSet struct {
	Bits   []uint64
	Length int
}

// NewBitSet creates a new bit set that can hold 'size' bits
func NewBitSet(size int) *BitSet {
	return &BitSet{
		Bits:   make([]uint64, (size+63)/64),
		Length: size,
	}
}

// Set sets a bit at the given position
func (bs *BitSet) Set(pos int) {
	if pos >= 0 && pos < bs.Length {
		bs.Bits[pos/64] |= 1 << (pos % 64)
	}
}

// Clear clears a bit at the given position
func (bs *BitSet) Clear(pos int) {
	if pos >= 0 && pos < bs.Length {
		bs.Bits[pos/64] &^= 1 << (pos % 64)
	}
}

// IsSet returns true if the bit at position is set
func (bs *BitSet) IsSet(pos int) bool {
	if pos < 0 || pos >= bs.Length {
		return false
	}
	return (bs.Bits[pos/64] & (1 << (pos % 64))) != 0
}

// Or performs bitwise OR with another BitSet
func (bs *BitSet) Or(other *BitSet) {
	minLen := len(bs.Bits)
	if len(other.Bits) < minLen {
		minLen = len(other.Bits)
	}
	for i := 0; i < minLen; i++ {
		bs.Bits[i] |= other.Bits[i]
	}
}

// Copy copies another BitSet into this one
func (bs *BitSet) Copy(other *BitSet) {
	if len(bs.Bits) < len(other.Bits) {
		bs.Bits = make([]uint64, len(other.Bits))
	}
	copy(bs.Bits, other.Bits)
	bs.Length = other.Length
}

// NextBit finds the next set bit after lastPos (inclusive search starting from lastPos+1)
func (bs *BitSet) NextBit(lastPos int) int {
	pos := lastPos + 1
	if pos >= bs.Length {
		return -1
	}

	wordIndex := pos / 64
	bitIndex := pos % 64

	if wordIndex >= len(bs.Bits) {
		return -1
	}

	// Mask off bits we don't want in the current word
	word := bs.Bits[wordIndex] & (^uint64(0) << bitIndex)

	for {
		if word != 0 {
			// Find the first set bit in this word
			bitPos := wordIndex * 64
			for word&1 == 0 {
				word >>= 1
				bitPos++
			}
			if bitPos < bs.Length {
				return bitPos
			}
		}

		wordIndex++
		if wordIndex >= len(bs.Bits) {
			break
		}
		word = bs.Bits[wordIndex]
	}

	return -1
}

// Equal checks if two BitSets are equal
func (bs *BitSet) Equal(other *BitSet) bool {
	if bs.Length != other.Length {
		return false
	}

	words := (bs.Length + 63) / 64
	for i := 0; i < words; i++ {
		if i < len(bs.Bits) && i < len(other.Bits) {
			if bs.Bits[i] != other.Bits[i] {
				return false
			}
		} else if i < len(bs.Bits) && bs.Bits[i] != 0 {
			return false
		} else if i < len(other.Bits) && other.Bits[i] != 0 {
			return false
		}
	}

	return true
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
