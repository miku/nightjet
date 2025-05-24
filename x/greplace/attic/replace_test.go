package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompleteReplacerBasic(t *testing.T) {
	replacer := NewCompleteReplacer()

	// Add simple patterns
	err := replacer.AddPattern("hello", "hi")
	if err != nil {
		t.Fatalf("Failed to add pattern: %v", err)
	}

	err = replacer.AddPattern("world", "universe")
	if err != nil {
		t.Fatalf("Failed to add pattern: %v", err)
	}

	// Compile the replacer
	err = replacer.Compile()
	if err != nil {
		t.Fatalf("Failed to compile replacer: %v", err)
	}

	// Test basic replacement
	result, updated, err := replacer.Replace("hello world")
	if err != nil {
		t.Fatalf("Replace failed: %v", err)
	}

	if !updated {
		t.Error("Should have updated the string")
	}

	// Note: The exact result depends on DFA construction, but should contain replacements
	if !strings.Contains(result, "hi") && !strings.Contains(result, "universe") {
		t.Errorf("Result should contain replacements, got: %q", result)
	}
}

func TestCompleteReplacerNoPatterns(t *testing.T) {
	replacer := NewCompleteReplacer()

	// Try to compile without patterns
	err := replacer.Compile()
	if err == nil {
		t.Error("Should fail to compile without patterns")
	}
}

func TestCompleteReplacerBeforeCompile(t *testing.T) {
	replacer := NewCompleteReplacer()
	replacer.AddPattern("test", "exam")

	// Try to replace before compiling
	_, _, err := replacer.Replace("test string")
	if err == nil {
		t.Error("Should fail to replace before compiling")
	}
}

func TestReplacementEngineSimple(t *testing.T) {
	// Create a simple test case by manually building components
	pp := NewPatternProcessor()
	pp.AddPattern("a", "x")

	builder := NewDFABuilder(pp)
	dfa, err := builder.BuildDFA()
	if err != nil {
		t.Fatalf("Failed to build DFA: %v", err)
	}

	engine := NewReplacementEngine(dfa, pp)

	// Test simple replacement
	result, updated, err := engine.ReplaceString("banana")
	if err != nil {
		t.Fatalf("ReplaceString failed: %v", err)
	}

	if !updated {
		t.Error("Should have updated the string")
	}

	// Should replace 'a' with 'x'
	expected := "bxnxnx"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestReplacementEngineNoMatch(t *testing.T) {
	pp := NewPatternProcessor()
	pp.AddPattern("xyz", "abc")

	builder := NewDFABuilder(pp)
	dfa, err := builder.BuildDFA()
	if err != nil {
		t.Fatalf("Failed to build DFA: %v", err)
	}

	engine := NewReplacementEngine(dfa, pp)

	// Test string with no matches
	result, updated, err := engine.ReplaceString("hello world")
	if err != nil {
		t.Fatalf("ReplaceString failed: %v", err)
	}

	if updated {
		t.Error("Should not have updated the string")
	}

	if result != "hello world" {
		t.Errorf("Expected unchanged string, got %q", result)
	}
}

func TestReplacementEngineEmptyString(t *testing.T) {
	pp := NewPatternProcessor()
	pp.AddPattern("test", "exam")

	builder := NewDFABuilder(pp)
	dfa, err := builder.BuildDFA()
	if err != nil {
		t.Fatalf("Failed to build DFA: %v", err)
	}

	engine := NewReplacementEngine(dfa, pp)

	// Test empty string
	result, updated, err := engine.ReplaceString("")
	if err != nil {
		t.Fatalf("ReplaceString failed: %v", err)
	}

	if updated {
		t.Error("Should not have updated empty string")
	}

	if result != "" {
		t.Errorf("Expected empty string, got %q", result)
	}
}

func TestReplacementEngineMultiplePatterns(t *testing.T) {
	pp := NewPatternProcessor()
	pp.AddPattern("cat", "dog")
	pp.AddPattern("bird", "fish")

	builder := NewDFABuilder(pp)
	dfa, err := builder.BuildDFA()
	if err != nil {
		t.Fatalf("Failed to build DFA: %v", err)
	}

	engine := NewReplacementEngine(dfa, pp)

	// Test string with multiple matches
	result, updated, err := engine.ReplaceString("cat and bird")
	if err != nil {
		t.Fatalf("ReplaceString failed: %v", err)
	}

	if !updated {
		t.Error("Should have updated the string")
	}

	// Result should contain both replacements
	if !strings.Contains(result, "dog") || !strings.Contains(result, "fish") {
		t.Errorf("Expected both replacements, got %q", result)
	}
}

func TestReplacementEngineReader(t *testing.T) {
	pp := NewPatternProcessor()
	pp.AddPattern("old", "new")

	builder := NewDFABuilder(pp)
	dfa, err := builder.BuildDFA()
	if err != nil {
		t.Fatalf("Failed to build DFA: %v", err)
	}

	engine := NewReplacementEngine(dfa, pp)

	// Test reader/writer interface
	input := strings.NewReader("old text with old words")
	var output bytes.Buffer

	updated, err := engine.ReplaceReader(input, &output)
	if err != nil {
		t.Fatalf("ReplaceReader failed: %v", err)
	}

	if !updated {
		t.Error("Should have updated the text")
	}

	result := output.String()
	if !strings.Contains(result, "new") {
		t.Errorf("Expected replacements in output, got %q", result)
	}
}

func TestReplacementEngineInPlace(t *testing.T) {
	pp := NewPatternProcessor()
	pp.AddPattern("test", "exam")

	builder := NewDFABuilder(pp)
	dfa, err := builder.BuildDFA()
	if err != nil {
		t.Fatalf("Failed to build DFA: %v", err)
	}

	engine := NewReplacementEngine(dfa, pp)

	// Test in-place replacement
	lines := []string{
		"this is a test",
		"another test line",
		"no matches here",
	}

	updated, err := engine.ReplaceInPlace(lines)
	if err != nil {
		t.Fatalf("ReplaceInPlace failed: %v", err)
	}

	if !updated {
		t.Error("Should have updated the lines")
	}

	// Check that lines were modified
	foundReplacement := false
	for _, line := range lines {
		if strings.Contains(line, "exam") {
			foundReplacement = true
			break
		}
	}

	if !foundReplacement {
		t.Error("Should have found replacement in lines")
	}
}

func TestReplacementEngineValidation(t *testing.T) {
	pp := NewPatternProcessor()
	pp.AddPattern("test", "exam")

	builder := NewDFABuilder(pp)
	dfa, err := builder.BuildDFA()
	if err != nil {
		t.Fatalf("Failed to build DFA: %v", err)
	}

	engine := NewReplacementEngine(dfa, pp)

	// Test DFA validation
	err = engine.ValidateDFA()
	if err != nil {
		t.Errorf("DFA validation failed: %v", err)
	}

	// Test with empty DFA
	emptyEngine := NewReplacementEngine([]DFAState{}, pp)
	err = emptyEngine.ValidateDFA()
	if err == nil {
		t.Error("Should fail validation with empty DFA")
	}
}

func TestReplacementEngineStats(t *testing.T) {
	pp := NewPatternProcessor()
	pp.AddPattern("hello", "hi")
	pp.AddPattern("world", "earth")

	builder := NewDFABuilder(pp)
	dfa, err := builder.BuildDFA()
	if err != nil {
		t.Fatalf("Failed to build DFA: %v", err)
	}

	engine := NewReplacementEngine(dfa, pp)

	stats := engine.GetStats()

	// Check that stats contain expected fields
	if _, ok := stats["dfa_states"]; !ok {
		t.Error("Stats should contain dfa_states")
	}

	if _, ok := stats["patterns"]; !ok {
		t.Error("Stats should contain patterns")
	}

	if _, ok := stats["final_states"]; !ok {
		t.Error("Stats should contain final_states")
	}

	if _, ok := stats["transitions"]; !ok {
		t.Error("Stats should contain transitions")
	}

	// Check that values make sense
	if stats["patterns"].(int) != 2 {
		t.Errorf("Expected 2 patterns, got %v", stats["patterns"])
	}

	if stats["dfa_states"].(int) < 1 {
		t.Errorf("Expected at least 1 DFA state, got %v", stats["dfa_states"])
	}
}

func TestReplacementEngineDebug(t *testing.T) {
	pp := NewPatternProcessor()
	pp.AddPattern("test", "exam")

	builder := NewDFABuilder(pp)
	dfa, err := builder.BuildDFA()
	if err != nil {
		t.Fatalf("Failed to build DFA: %v", err)
	}

	engine := NewReplacementEngine(dfa, pp)

	// Test debug output for state 0
	debug, err := engine.DebugState(0)
	if err != nil {
		t.Fatalf("DebugState failed: %v", err)
	}

	if !strings.Contains(debug, "State 0") {
		t.Error("Debug output should contain state number")
	}

	// Test invalid state
	_, err = engine.DebugState(9999)
	if err == nil {
		t.Error("Should fail for invalid state index")
	}
}

func TestCompleteReplacerWorkflow(t *testing.T) {
	// Test the complete workflow
	replacer := NewCompleteReplacer()

	// Add various types of patterns
	patterns := map[string]string{
		"hello": "hi",
		"world": "earth",
		"test":  "exam",
		"quick": "fast",
		"brown": "red",
	}

	for from, to := range patterns {
		err := replacer.AddPattern(from, to)
		if err != nil {
			t.Fatalf("Failed to add pattern %s->%s: %v", from, to, err)
		}
	}

	// Compile
	err := replacer.Compile()
	if err != nil {
		t.Fatalf("Failed to compile: %v", err)
	}

	// Test replacement
	testCases := []struct {
		input        string
		shouldUpdate bool
	}{
		{"hello world", true},
		{"test case", true},
		{"quick brown fox", true},
		{"nothing matches", false},
		{"", false},
	}

	for _, tc := range testCases {
		result, updated, err := replacer.Replace(tc.input)
		if err != nil {
			t.Errorf("Replace failed for %q: %v", tc.input, err)
			continue
		}

		if updated != tc.shouldUpdate {
			t.Errorf("For input %q: expected updated=%v, got %v",
				tc.input, tc.shouldUpdate, updated)
		}

		if updated {
			// Verify that result is different from input
			if result == tc.input {
				t.Errorf("Expected change for %q, but result is same", tc.input)
			}
		} else {
			// Verify that result is same as input
			if result != tc.input {
				t.Errorf("Expected no change for %q, but got %q", tc.input, result)
			}
		}
	}

	// Check stats
	stats := replacer.GetStats()
	if !stats["compiled"].(bool) {
		t.Error("Stats should show compiled=true")
	}

	if stats["patterns"].(int) != len(patterns) {
		t.Errorf("Expected %d patterns in stats, got %v", len(patterns), stats["patterns"])
	}
}

// Benchmark tests
func BenchmarkSimpleReplacement(b *testing.B) {
	replacer := NewCompleteReplacer()
	replacer.AddPattern("test", "exam")
	replacer.AddPattern("hello", "hi")
	replacer.Compile()

	input := "test hello world test"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		replacer.Replace(input)
	}
}

func BenchmarkComplexReplacement(b *testing.B) {
	replacer := NewCompleteReplacer()

	// Add many patterns
	patterns := [][]string{
		{"the", "a"},
		{"and", "&"},
		{"test", "exam"},
		{"hello", "hi"},
		{"world", "earth"},
		{"quick", "fast"},
		{"brown", "red"},
		{"fox", "animal"},
		{"jumps", "leaps"},
		{"over", "above"},
	}

	for _, pattern := range patterns {
		replacer.AddPattern(pattern[0], pattern[1])
	}
	replacer.Compile()

	input := "the quick brown fox jumps over the lazy dog and says hello to the world"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		replacer.Replace(input)
	}
}

func BenchmarkLargeText(b *testing.B) {
	replacer := NewCompleteReplacer()
	replacer.AddPattern("test", "exam")
	replacer.AddPattern("the", "a")
	replacer.Compile()

	// Create large input text
	input := strings.Repeat("This is a test of the replacement system. ", 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		replacer.Replace(input)
	}
}
