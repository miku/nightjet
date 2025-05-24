package main

import (
	"strings"
	"testing"
)

func TestNewPatternProcessor(t *testing.T) {
	pp := NewPatternProcessor()

	if pp == nil {
		t.Fatal("NewPatternProcessor() returned nil")
	}

	if pp.PatternCount() != 0 {
		t.Errorf("new processor should have 0 patterns, got %d", pp.PatternCount())
	}

	// Test word-end characters are initialized
	if !pp.IsWordEndChar(' ') || !pp.IsWordEndChar('\t') || !pp.IsWordEndChar('\n') {
		t.Error("word-end characters not properly initialized")
	}

	if pp.IsWordEndChar('a') || pp.IsWordEndChar('5') {
		t.Error("non-word-end characters incorrectly marked as word-end")
	}
}

func TestAddPattern(t *testing.T) {
	pp := NewPatternProcessor()

	// Test adding valid patterns
	tests := []struct {
		from, to string
		wantErr  bool
	}{
		{"hello", "hi", false},
		{"world", "universe", false},
		{"\\btest\\b", "exam", false},
		{"\\^start", "beginning", false},
		{"end\\$", "finish", false},
		{"", "empty", true}, // empty from pattern should fail
	}

	for i, tt := range tests {
		err := pp.AddPattern(tt.from, tt.to)
		if (err != nil) != tt.wantErr {
			t.Errorf("test %d: AddPattern(%q, %q) error = %v, wantErr %v",
				i, tt.from, tt.to, err, tt.wantErr)
		}
	}

	// Should have added 5 valid patterns (skipped the empty one)
	if pp.PatternCount() != 5 {
		t.Errorf("expected 5 patterns, got %d", pp.PatternCount())
	}
}

func TestCalculateEffectiveLength(t *testing.T) {
	pp := NewPatternProcessor()

	tests := []struct {
		input    string
		expected int
	}{
		{"hello", 5},
		{"\\bhello\\b", 7},     // \b + h + e + l + l + o + \b
		{"\\^start", 6},        // \^ + s + t + a + r + t
		{"end\\$", 4},          // e + n + d + \$
		{"\\t\\r\\n", 3},       // \t + \r + \n
		{"a\\\\b", 3},          // a + \\ + b
		{"\\btest\\bmore", 10}, // \b + t + e + s + t + \b + m + o + r + e
		{"", 0},
	}

	for i, tt := range tests {
		result := pp.calculateEffectiveLength(tt.input)
		if result != tt.expected {
			t.Errorf("test %d: calculateEffectiveLength(%q) = %d, want %d",
				i, tt.input, result, tt.expected)
		}
	}
}

func TestStartsAtWord(t *testing.T) {
	pp := NewPatternProcessor()

	tests := []struct {
		pattern  string
		expected bool
	}{
		{"\\bhello", true},
		{"\\^start", true},
		{"hello", false},
		{"\\b", false},       // just the boundary marker, no content
		{"\\^", false},       // just the boundary marker, no content
		{"a\\bhello", false}, // \b not at start
		{"\\btest\\b", true}, // starts with \b
	}

	for i, tt := range tests {
		result := pp.startsAtWord(tt.pattern)
		if result != tt.expected {
			t.Errorf("test %d: startsAtWord(%q) = %v, want %v",
				i, tt.pattern, result, tt.expected)
		}
	}
}

func TestEndsAtWord(t *testing.T) {
	pp := NewPatternProcessor()

	tests := []struct {
		pattern  string
		expected bool
	}{
		{"hello\\b", true},
		{"end\\$", true},
		{"hello", false},
		{"\\b", false},       // just the boundary marker
		{"\\$", false},       // just the boundary marker
		{"hello\\ba", false}, // \b not at end
		{"\\btest\\b", true}, // ends with \b
	}

	for i, tt := range tests {
		result := pp.endsAtWord(tt.pattern)
		if result != tt.expected {
			t.Errorf("test %d: endsAtWord(%q) = %v, want %v",
				i, tt.pattern, result, tt.expected)
		}
	}
}

func TestParsePattern(t *testing.T) {
	pp := NewPatternProcessor()

	tests := []struct {
		pattern  string
		expected []ParsedChar
	}{
		{
			"abc",
			[]ParsedChar{
				{int('a'), false},
				{int('b'), false},
				{int('c'), false},
			},
		},
		{
			"\\bhello\\b",
			[]ParsedChar{
				{SpaceChar, true},
				{int('h'), false},
				{int('e'), false},
				{int('l'), false},
				{int('l'), false},
				{int('o'), false},
				{SpaceChar, true},
			},
		},
		{
			"\\^start\\$",
			[]ParsedChar{
				{StartOfLine, true},
				{int('s'), false},
				{int('t'), false},
				{int('a'), false},
				{int('r'), false},
				{int('t'), false},
				{EndOfLine, true},
			},
		},
		{
			"\\t\\r\\n",
			[]ParsedChar{
				{int('\t'), false},
				{int('\r'), false},
				{int('\n'), false},
			},
		},
		{
			"a\\\\b",
			[]ParsedChar{
				{int('a'), false},
				{int('\\'), false},
				{int('b'), false},
			},
		},
	}

	for i, tt := range tests {
		result, err := pp.ParsePattern(tt.pattern)
		if err != nil {
			t.Errorf("test %d: ParsePattern(%q) returned error: %v", i, tt.pattern, err)
			continue
		}

		if len(result) != len(tt.expected) {
			t.Errorf("test %d: ParsePattern(%q) length = %d, want %d",
				i, tt.pattern, len(result), len(tt.expected))
			continue
		}

		for j, expected := range tt.expected {
			if result[j].Char != expected.Char || result[j].IsSpecial != expected.IsSpecial {
				t.Errorf("test %d char %d: ParsePattern(%q) = {%d, %v}, want {%d, %v}",
					i, j, tt.pattern, result[j].Char, result[j].IsSpecial,
					expected.Char, expected.IsSpecial)
			}
		}
	}
}

func TestGetPattern(t *testing.T) {
	pp := NewPatternProcessor()

	// Add some patterns
	pp.AddPattern("hello", "hi")
	pp.AddPattern("world", "universe")

	// Test valid indices
	pattern, err := pp.GetPattern(0)
	if err != nil {
		t.Errorf("GetPattern(0) returned error: %v", err)
	}
	if pattern.From != "hello" || pattern.To != "hi" {
		t.Errorf("GetPattern(0) = {%q, %q}, want {%q, %q}",
			pattern.From, pattern.To, "hello", "hi")
	}

	pattern, err = pp.GetPattern(1)
	if err != nil {
		t.Errorf("GetPattern(1) returned error: %v", err)
	}
	if pattern.From != "world" || pattern.To != "universe" {
		t.Errorf("GetPattern(1) = {%q, %q}, want {%q, %q}",
			pattern.From, pattern.To, "world", "universe")
	}

	// Test invalid indices
	_, err = pp.GetPattern(-1)
	if err == nil {
		t.Error("GetPattern(-1) should return error")
	}

	_, err = pp.GetPattern(2)
	if err == nil {
		t.Error("GetPattern(2) should return error")
	}
}

func TestValidatePatterns(t *testing.T) {
	pp := NewPatternProcessor()

	// Empty processor should fail validation
	err := pp.ValidatePatterns()
	if err == nil {
		t.Error("empty processor should fail validation")
	}

	// Add valid patterns
	pp.AddPattern("hello", "hi")
	pp.AddPattern("\\bworld\\b", "universe")

	// Should pass validation
	err = pp.ValidatePatterns()
	if err != nil {
		t.Errorf("valid patterns should pass validation, got error: %v", err)
	}
}

func TestPatternProcessorString(t *testing.T) {
	pp := NewPatternProcessor()
	pp.AddPattern("hello", "hi")
	pp.AddPattern("\\bworld\\b", "universe")

	str := pp.String()

	// Check that string contains expected information
	if !strings.Contains(str, "2 patterns") {
		t.Error("String() should mention number of patterns")
	}
	if !strings.Contains(str, "hello") || !strings.Contains(str, "hi") {
		t.Error("String() should contain pattern details")
	}
	if !strings.Contains(str, "world") || !strings.Contains(str, "universe") {
		t.Error("String() should contain all patterns")
	}
}

func TestClear(t *testing.T) {
	pp := NewPatternProcessor()

	// Add some patterns
	pp.AddPattern("hello", "hi")
	pp.AddPattern("world", "universe")

	if pp.PatternCount() != 2 {
		t.Errorf("expected 2 patterns before clear, got %d", pp.PatternCount())
	}

	// Clear patterns
	pp.Clear()

	if pp.PatternCount() != 0 {
		t.Errorf("expected 0 patterns after clear, got %d", pp.PatternCount())
	}
}

func TestProcessSpecialChar(t *testing.T) {
	pp := NewPatternProcessor()

	tests := []struct {
		char, nextChar byte
		expected       int
	}{
		{'\\', 'b', SpaceChar},
		{'\\', '^', StartOfLine},
		{'\\', '$', EndOfLine},
		{'\\', 'r', int('\r')},
		{'\\', 't', int('\t')},
		{'\\', 'v', int('\v')},
		{'\\', 'n', int('\n')},
		{'\\', '\\', int('\\')},
		{'a', 'b', int('a')},  // non-backslash should return first char
		{'\\', 'x', int('x')}, // unknown escape should return literal
	}

	for i, tt := range tests {
		result := pp.ProcessSpecialChar(tt.char, tt.nextChar)
		if result != tt.expected {
			t.Errorf("test %d: ProcessSpecialChar(%q, %q) = %d, want %d",
				i, tt.char, tt.nextChar, result, tt.expected)
		}
	}
}

func TestPatternMetadata(t *testing.T) {
	pp := NewPatternProcessor()

	// Test various patterns and their metadata
	tests := []struct {
		from, to                                 string
		expectedLength                           int
		expectedStartsAtWord, expectedEndsAtWord bool
	}{
		{"hello", "hi", 5, false, false},
		{"\\bhello", "hi", 6, true, false},
		{"hello\\b", "hi", 6, false, true},
		{"\\bhello\\b", "hi", 7, true, true},
		{"\\^start", "begin", 6, true, false},
		{"end\\$", "finish", 4, false, true},
		{"\\^line\\$", "row", 6, true, true},
	}

	for i, tt := range tests {
		pp.Clear()
		err := pp.AddPattern(tt.from, tt.to)
		if err != nil {
			t.Errorf("test %d: AddPattern returned error: %v", i, err)
			continue
		}

		pattern, err := pp.GetPattern(0)
		if err != nil {
			t.Errorf("test %d: GetPattern returned error: %v", i, err)
			continue
		}

		if pattern.Length != tt.expectedLength {
			t.Errorf("test %d: pattern length = %d, want %d",
				i, pattern.Length, tt.expectedLength)
		}

		if pattern.StartsAtWord != tt.expectedStartsAtWord {
			t.Errorf("test %d: pattern StartsAtWord = %v, want %v",
				i, pattern.StartsAtWord, tt.expectedStartsAtWord)
		}

		if pattern.EndsAtWord != tt.expectedEndsAtWord {
			t.Errorf("test %d: pattern EndsAtWord = %v, want %v",
				i, pattern.EndsAtWord, tt.expectedEndsAtWord)
		}
	}
}

// Benchmark tests
func BenchmarkAddPattern(b *testing.B) {
	pp := NewPatternProcessor()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pp.Clear()
		pp.AddPattern("test", "example")
		pp.AddPattern("\\bhello\\b", "hi")
		pp.AddPattern("\\^start", "begin")
	}
}

func BenchmarkParsePattern(b *testing.B) {
	pp := NewPatternProcessor()
	pattern := "\\bhello\\bworld\\^test\\$more"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pp.ParsePattern(pattern)
	}
}
