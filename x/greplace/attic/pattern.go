package main

import (
	"fmt"
	"strings"
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
	From         string
	To           string
	Length       int  // effective length after processing escapes
	StartsAtWord bool // pattern starts with \b or \^
	EndsAtWord   bool // pattern ends with \b or \$
}

// ParsedChar represents a character in a pattern after escape processing
type ParsedChar struct {
	Char      int  // the character code (may be special like SpaceChar)
	IsSpecial bool // true if this was an escape sequence
}

// PatternProcessor handles the parsing and preprocessing of patterns
type PatternProcessor struct {
	patterns     []Pattern
	wordEndChars map[byte]bool // characters that end words
}

// NewPatternProcessor creates a new pattern processor
func NewPatternProcessor() *PatternProcessor {
	pp := &PatternProcessor{
		patterns:     make([]Pattern, 0),
		wordEndChars: make(map[byte]bool),
	}

	// Initialize word-ending characters (whitespace)
	wordEndChars := " \t\n\r\v\f"
	for i := 0; i < len(wordEndChars); i++ {
		pp.wordEndChars[wordEndChars[i]] = true
	}

	return pp
}

// AddPattern adds a from->to replacement pattern
func (pp *PatternProcessor) AddPattern(from, to string) error {
	if len(from) == 0 {
		return fmt.Errorf("empty from pattern not allowed")
	}
	pattern := Pattern{
		From:         from,
		To:           to,
		Length:       pp.calculateEffectiveLength(from),
		StartsAtWord: pp.startsAtWord(from),
		EndsAtWord:   pp.endsAtWord(from),
	}

	pp.patterns = append(pp.patterns, pattern)
	return nil
}

// ParsePattern converts a pattern string into a sequence of parsed characters
func (pp *PatternProcessor) ParsePattern(pattern string) ([]ParsedChar, error) {
	var result []ParsedChar
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '\\' && i+1 < len(pattern) {
			// Handle escape sequence
			nextChar := pattern[i+1]
			charCode, isSpecial := pp.processEscapeSequence(nextChar)
			result = append(result, ParsedChar{
				Char:      charCode,
				IsSpecial: isSpecial,
			})
			i++ // skip the escaped character
		} else {
			// Regular character
			result = append(result, ParsedChar{
				Char:      int(pattern[i]),
				IsSpecial: false,
			})
		}
	}
	return result, nil
}

// processEscapeSequence handles escape sequences like \b, \^, \$, etc.
func (pp *PatternProcessor) processEscapeSequence(char byte) (int, bool) {
	switch char {
	case 'b':
		return SpaceChar, true
	case '^':
		return StartOfLine, true
	case '$':
		return EndOfLine, true
	case 'r':
		return int('\r'), false
	case 't':
		return int('\t'), false
	case 'v':
		return int('\v'), false
	case 'n':
		return int('\n'), false
	case 'f':
		return int('\f'), false
	case '\\':
		return int('\\'), false
	default:
		// For any other escaped character, return it literally
		return int(char), false
	}
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

// startsAtWord checks if pattern starts with word boundary (\b or \^)
func (pp *PatternProcessor) startsAtWord(pattern string) bool {
	if len(pattern) >= 2 {
		if pattern[0] == '\\' && (pattern[1] == 'b' || pattern[1] == '^') {
			// Check if there's more after the boundary marker
			return len(pattern) > 2
		}
	}
	return false
}

// endsAtWord checks if pattern ends with word boundary (\b or \$)
func (pp *PatternProcessor) endsAtWord(pattern string) bool {
	if len(pattern) >= 2 {
		lastChar := pattern[len(pattern)-1]
		secondLastChar := pattern[len(pattern)-2]
		if secondLastChar == '\\' && (lastChar == 'b' || lastChar == '$') {
			// Check if there's content before the boundary marker
			return len(pattern) > 2
		}
	}
	return false
}

// IsWordEndChar returns true if the character ends a word
func (pp *PatternProcessor) IsWordEndChar(char byte) bool {
	return pp.wordEndChars[char]
}

// GetPatterns returns the current patterns
func (pp *PatternProcessor) GetPatterns() []Pattern {
	return pp.patterns
}

// GetPattern returns a specific pattern by index
func (pp *PatternProcessor) GetPattern(index int) (Pattern, error) {
	if index < 0 || index >= len(pp.patterns) {
		return Pattern{}, fmt.Errorf("pattern index %d out of range", index)
	}
	return pp.patterns[index], nil
}

// PatternCount returns the number of patterns
func (pp *PatternProcessor) PatternCount() int {
	return len(pp.patterns)
}

// ValidatePatterns checks all patterns for consistency
func (pp *PatternProcessor) ValidatePatterns() error {
	if len(pp.patterns) == 0 {
		return fmt.Errorf("no patterns defined")
	}

	for i, pattern := range pp.patterns {
		if pattern.Length == 0 {
			return fmt.Errorf("pattern %d has zero effective length", i)
		}

		// Validate that the pattern can be parsed
		_, err := pp.ParsePattern(pattern.From)
		if err != nil {
			return fmt.Errorf("pattern %d parse error: %v", i, err)
		}
	}

	return nil
}

// String returns a string representation of the pattern processor
func (pp *PatternProcessor) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("PatternProcessor with %d patterns:\n", len(pp.patterns)))

	for i, pattern := range pp.patterns {
		sb.WriteString(fmt.Sprintf("  [%d] '%s' -> '%s' (len=%d, start=%v, end=%v)\n",
			i, pattern.From, pattern.To, pattern.Length,
			pattern.StartsAtWord, pattern.EndsAtWord))
	}

	return sb.String()
}

// Clear removes all patterns
func (pp *PatternProcessor) Clear() {
	pp.patterns = pp.patterns[:0]
}

// ProcessSpecialChar converts escape sequences to internal character codes
// This is kept for compatibility with the existing interface
func (pp *PatternProcessor) ProcessSpecialChar(char byte, nextChar byte) int {
	if char != '\\' {
		return int(char)
	}

	charCode, _ := pp.processEscapeSequence(nextChar)
	return charCode
}
