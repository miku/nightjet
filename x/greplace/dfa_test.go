package main

import (
	"testing"
)

func TestNewDFABuilder(t *testing.T) {
	pp := NewPatternProcessor()
	pp.AddPattern("hello", "hi")

	builder := NewDFABuilder(pp)

	if builder == nil {
		t.Fatal("NewDFABuilder returned nil")
	}

	if builder.patterns != pp {
		t.Error("DFABuilder should reference the pattern processor")
	}

	if len(builder.followStates) != 0 {
		t.Error("DFABuilder should start with empty follow states")
	}

	if len(builder.finalStates) != 0 {
		t.Error("DFABuilder should start with empty final states")
	}
}

func TestBuildFollowStates(t *testing.T) {
	pp := NewPatternProcessor()
	pp.AddPattern("ab", "xy")          // Simple 2-char pattern
	pp.AddPattern("\\bhi\\b", "hello") // Pattern with word boundaries

	builder := NewDFABuilder(pp)
	err := builder.buildFollowStates()

	if err != nil {
		t.Fatalf("buildFollowStates failed: %v", err)
	}

	// Should have follow states for:
	// Pattern 0: 'a', 'b', end (3 states)
	// Pattern 1: SpaceChar, 'h', 'i', SpaceChar, end (5 states)
	// Total: 8 states
	expectedStates := 8
	if len(builder.followStates) != expectedStates {
		t.Errorf("Expected %d follow states, got %d", expectedStates, len(builder.followStates))
	}

	// Check first pattern follow states
	if builder.followStates[0].Char != int('a') {
		t.Errorf("First follow state should be 'a', got %d", builder.followStates[0].Char)
	}
	if builder.followStates[0].PatternIndex != 0 {
		t.Errorf("First follow state should be pattern 0, got %d", builder.followStates[0].PatternIndex)
	}
	if builder.followStates[0].Position != 0 {
		t.Errorf("First follow state should be position 0, got %d", builder.followStates[0].Position)
	}

	// Check that end markers exist
	endFound := false
	for _, follow := range builder.followStates {
		if follow.Char == 0 && follow.PatternIndex == 0 {
			endFound = true
			break
		}
	}
	if !endFound {
		t.Error("Should have end marker for first pattern")
	}
}

func TestCreateInitialStates(t *testing.T) {
	pp := NewPatternProcessor()
	pp.AddPattern("hello", "hi")
	pp.AddPattern("\\bworld", "earth")

	builder := NewDFABuilder(pp)
	builder.buildFollowStates()
	err := builder.createInitialStates()

	if err != nil {
		t.Fatalf("createInitialStates failed: %v", err)
	}

	// Should have 2 initial state sets: start and word
	if len(builder.stateSets) != 2 {
		t.Errorf("Expected 2 initial state sets, got %d", len(builder.stateSets))
	}

	startSet := builder.stateSets[0]
	wordSet := builder.stateSets[1]

	// Start set should have bits set for beginning of patterns
	hasStartBits := false
	pos := -1
	for {
		pos = startSet.States.NextBit(pos)
		if pos == -1 {
			break
		}
		hasStartBits = true
		break
	}
	if !hasStartBits {
		t.Error("Start set should have some bits set")
	}

	// Word set should also have bits set
	hasWordBits := false
	pos = -1
	for {
		pos = wordSet.States.NextBit(pos)
		if pos == -1 {
			break
		}
		hasWordBits = true
		break
	}
	if !hasWordBits {
		t.Error("Word set should have some bits set")
	}
}

func TestBuildDFASimple(t *testing.T) {
	pp := NewPatternProcessor()
	pp.AddPattern("a", "x") // Single character pattern

	builder := NewDFABuilder(pp)
	states, err := builder.BuildDFA()

	if err != nil {
		t.Fatalf("BuildDFA failed: %v", err)
	}

	if len(states) == 0 {
		t.Error("BuildDFA should create at least one state")
	}

	// Should have created some follow states
	if builder.GetFollowStateCount() == 0 {
		t.Error("Should have created follow states")
	}

	// Should have a reasonable number of DFA states
	stateCount := builder.GetStateCount()
	if stateCount < 2 || stateCount > 10 {
		t.Errorf("Expected 2-10 DFA states for simple pattern, got %d", stateCount)
	}
}

func TestBuildDFAMultiplePatterns(t *testing.T) {
	pp := NewPatternProcessor()
	pp.AddPattern("hello", "hi")
	pp.AddPattern("world", "earth")
	pp.AddPattern("test", "exam")

	builder := NewDFABuilder(pp)
	states, err := builder.BuildDFA()

	if err != nil {
		t.Fatalf("BuildDFA failed: %v", err)
	}

	if len(states) == 0 {
		t.Error("BuildDFA should create states for multiple patterns")
	}

	// Check that we have a reasonable number of states
	stateCount := builder.GetStateCount()
	if stateCount < 2 {
		t.Errorf("Expected at least 2 states for multiple patterns, got %d", stateCount)
	}

	// Verify follow states were created for all patterns
	followCount := builder.GetFollowStateCount()
	expectedMinFollow := len("hello") + len("world") + len("test") + 3 // +3 for end markers
	if followCount < expectedMinFollow {
		t.Errorf("Expected at least %d follow states, got %d", expectedMinFollow, followCount)
	}
}

func TestBuildDFAWithWordBoundaries(t *testing.T) {
	pp := NewPatternProcessor()
	pp.AddPattern("\\bhello\\b", "hi")

	builder := NewDFABuilder(pp)
	states, err := builder.BuildDFA()

	if err != nil {
		t.Fatalf("BuildDFA failed with word boundaries: %v", err)
	}

	if len(states) == 0 {
		t.Error("BuildDFA should create states for word boundary patterns")
	}

	// Should create follow states for: SpaceChar, h, e, l, l, o, SpaceChar, end
	expectedFollowStates := 8
	followCount := builder.GetFollowStateCount()
	if followCount != expectedFollowStates {
		t.Errorf("Expected %d follow states for word boundary pattern, got %d",
			expectedFollowStates, followCount)
	}
}

func TestBuildDFAEmptyPatterns(t *testing.T) {
	pp := NewPatternProcessor()

	builder := NewDFABuilder(pp)
	_, err := builder.BuildDFA()

	if err == nil {
		t.Error("BuildDFA should fail with empty patterns")
	}
}

func TestStateSetKey(t *testing.T) {
	pp := NewPatternProcessor()
	pp.AddPattern("test", "exam")

	builder := NewDFABuilder(pp)

	// Create two identical state sets
	set1 := StateSet{
		States:      NewBitSet(10),
		TableOffset: 0,
		FoundOffset: 1,
		FoundLength: 4,
	}
	set1.States.Set(1)
	set1.States.Set(3)

	set2 := StateSet{
		States:      NewBitSet(10),
		TableOffset: 0,
		FoundOffset: 1,
		FoundLength: 4,
	}
	set2.States.Set(1)
	set2.States.Set(3)

	// Should generate the same key
	key1 := builder.stateSetKey(&set1)
	key2 := builder.stateSetKey(&set2)

	if key1 != key2 {
		t.Errorf("Identical state sets should have same key: %q vs %q", key1, key2)
	}

	// Different state set should have different key
	set3 := StateSet{
		States:      NewBitSet(10),
		TableOffset: 1, // Different table offset
		FoundOffset: 1,
		FoundLength: 4,
	}
	set3.States.Set(1)
	set3.States.Set(3)

	key3 := builder.stateSetKey(&set3)
	if key1 == key3 {
		t.Error("Different state sets should have different keys")
	}
}

func TestCalculateOffsets(t *testing.T) {
	pp := NewPatternProcessor()
	pp.AddPattern("hello", "hi")
	pp.AddPattern("\\bworld\\b", "earth")

	builder := NewDFABuilder(pp)

	// Test normal pattern
	pattern1, _ := pp.GetPattern(0)
	toOffset1 := builder.calculateToOffset(pattern1, 0)
	fromOffset1 := builder.calculateFromOffset(pattern1, 0)

	if toOffset1 != 0 {
		t.Errorf("Normal pattern should have toOffset 0, got %d", toOffset1)
	}
	if fromOffset1 != pattern1.Length {
		t.Errorf("Normal pattern fromOffset should be %d, got %d", pattern1.Length, fromOffset1)
	}

	// Test word boundary pattern
	pattern2, _ := pp.GetPattern(1)
	toOffset2 := builder.calculateToOffset(pattern2, 0)
	fromOffset2 := builder.calculateFromOffset(pattern2, 0)

	if toOffset2 != 1 {
		t.Errorf("Word boundary pattern should have toOffset 1, got %d", toOffset2)
	}
	// Should account for end boundary marker
	expectedFromOffset := pattern2.Length - 1 // -1 for end boundary
	if fromOffset2 != expectedFromOffset {
		t.Errorf("Word boundary pattern fromOffset should be %d, got %d", expectedFromOffset, fromOffset2)
	}
}

// Benchmark tests
func BenchmarkBuildDFASimple(b *testing.B) {
	pp := NewPatternProcessor()
	pp.AddPattern("hello", "hi")
	pp.AddPattern("world", "earth")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder := NewDFABuilder(pp)
		builder.BuildDFA()
	}
}

func BenchmarkBuildDFAComplex(b *testing.B) {
	pp := NewPatternProcessor()
	pp.AddPattern("\\bhello\\b", "hi")
	pp.AddPattern("\\^start", "begin")
	pp.AddPattern("end\\$", "finish")
	pp.AddPattern("test", "exam")
	pp.AddPattern("example", "sample")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder := NewDFABuilder(pp)
		builder.BuildDFA()
	}
}
