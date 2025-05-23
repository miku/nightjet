package main

import (
	"fmt"
	"io"
	"strings"
)

// ReplacementEngine executes the DFA against input text to perform replacements
type ReplacementEngine struct {
	dfa      []DFAState
	patterns *PatternProcessor
	updated  bool // tracks if any replacements were made
}

// NewReplacementEngine creates a new replacement engine
func NewReplacementEngine(dfa []DFAState, patterns *PatternProcessor) *ReplacementEngine {
	return &ReplacementEngine{
		dfa:      dfa,
		patterns: patterns,
		updated:  false,
	}
}

// ReplaceString performs string replacement on input text
func (re *ReplacementEngine) ReplaceString(input string) (string, bool, error) {
	if len(re.dfa) == 0 {
		return input, false, fmt.Errorf("no DFA states available")
	}

	re.updated = false
	output := make([]byte, 0, len(input)*2) // Start with double capacity
	inputBytes := []byte(input)
	pos := 0

	for pos < len(inputBytes) {
		// Try to find a match starting at current position
		matchLen, replacement, err := re.findMatchFixed(inputBytes, pos)
		if err != nil {
			return "", false, err
		}

		if matchLen > 0 {
			// Found a match, perform replacement
			output = append(output, []byte(replacement)...)
			pos += matchLen
			re.updated = true
		} else {
			// No match, copy original character
			output = append(output, inputBytes[pos])
			pos++
		}
	}

	return string(output), re.updated, nil
}

// findMatch runs the DFA from the given position to find the longest match
func (re *ReplacementEngine) findMatch(input []byte, startPos int) (int, string, error) {
	if startPos >= len(input) {
		return 0, "", nil
	}

	currentState := 0 // Start with state 0
	pos := startPos
	bestMatch := ""
	bestLength := 0

	// Run the DFA
	for pos <= len(input) {
		var inputChar int
		if pos < len(input) {
			inputChar = int(input[pos])
		} else {
			inputChar = 0 // End of input
		}

		// Check current state for final state
		if currentState < len(re.dfa) {
			state := &re.dfa[currentState]

			// If this is a final state, record the match
			if state.PatternIndex >= 0 {
				matchLength := pos - startPos - state.FromOffset
				if matchLength > 0 && matchLength > bestLength {
					bestMatch = state.ReplaceString
					bestLength = matchLength
				}
			}
		}

		// If at end of input, break
		if pos >= len(input) {
			break
		}

		// Get next state
		nextState, err := re.getNextState(currentState, inputChar)
		if err != nil {
			return 0, "", err
		}

		if nextState == -1 {
			// No more transitions possible
			break
		}

		currentState = nextState
		pos++
	}

	return bestLength, bestMatch, nil
}

func (re *ReplacementEngine) findMatchFixed(input []byte, startPos int) (int, string, error) {
	if startPos >= len(input) {
		return 0, "", nil
	}

	// Simple brute force approach for now to verify the patterns work
	patterns := re.patterns.GetPatterns()

	bestLength := 0
	bestReplacement := ""

	// Try each pattern at the current position
	for _, pattern := range patterns {
		if re.matchesAt(input, startPos, pattern.From) {
			if len(pattern.From) > bestLength {
				bestLength = len(pattern.From)
				bestReplacement = pattern.To
			}
		}
	}

	return bestLength, bestReplacement, nil
}

// SimpleDFAReplacementEngine - let's also create a working DFA version
func (re *ReplacementEngine) findMatchDFA(input []byte, startPos int) (int, string, error) {
	if startPos >= len(input) {
		return 0, "", nil
	}

	currentState := 0 // Start with state 0
	pos := startPos
	bestMatch := ""
	bestLength := 0

	// Run the DFA
	for pos <= len(input) && currentState >= 0 && currentState < len(re.dfa) {
		// Check if current state is a final state
		state := &re.dfa[currentState]
		if state.PatternIndex >= 0 {
			// This is a final state - we found a match
			matchLength := pos - startPos
			if matchLength > bestLength {
				bestMatch = state.ReplaceString
				bestLength = matchLength
			}
		}

		// If we're at the end of input, break
		if pos >= len(input) {
			break
		}

		// Get the next character
		inputChar := int(input[pos])

		// Get next state
		nextState := state.Next[inputChar]

		// Handle different types of next states
		if nextState == -1 {
			// No transition - end of matching
			break
		} else if nextState < -1 {
			// This indicates a final state reference
			finalStateIndex := -(nextState + 2)
			if finalStateIndex >= 0 && finalStateIndex < len(re.dfa) {
				// Found a final state
				finalState := &re.dfa[finalStateIndex]
				if finalState.PatternIndex >= 0 {
					matchLength := pos - startPos + 1
					if matchLength > bestLength {
						bestMatch = finalState.ReplaceString
						bestLength = matchLength
					}
				}
			}
			break
		} else {
			// Normal state transition
			currentState = nextState
		}

		pos++
	}

	return bestLength, bestMatch, nil
}

// Add this helper method
func (re *ReplacementEngine) matchesAt(input []byte, pos int, pattern string) bool {
	if pos+len(pattern) > len(input) {
		return false
	}

	for i, char := range []byte(pattern) {
		if input[pos+i] != char {
			return false
		}
	}

	return true
}

// getNextState returns the next state for given current state and input character
func (re *ReplacementEngine) getNextState(stateIndex int, char int) (int, error) {
	if stateIndex < 0 || stateIndex >= len(re.dfa) {
		return -1, fmt.Errorf("invalid state index: %d", stateIndex)
	}

	if char < 0 || char >= 256 {
		// Handle special characters or out-of-range
		if char == 0 { // End of input
			return re.dfa[stateIndex].Next[0], nil
		}
		return -1, nil
	}

	nextState := re.dfa[stateIndex].Next[char]

	// Handle final states (negative indices)
	if nextState < -1 {
		// This is a final state reference
		finalStateIndex := -(nextState + 2)
		if finalStateIndex >= 0 && finalStateIndex < len(re.dfa) {
			return finalStateIndex, nil
		}
	}

	return nextState, nil
}

// ReplaceReader performs replacement on data from a reader, writing to a writer
func (re *ReplacementEngine) ReplaceReader(reader io.Reader, writer io.Writer) (bool, error) {
	re.updated = false

	// Read input in chunks
	buffer := make([]byte, 8192)
	var accumulated []byte

	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			accumulated = append(accumulated, buffer[:n]...)
		}

		if err == io.EOF {
			break
		} else if err != nil {
			return false, fmt.Errorf("error reading input: %v", err)
		}
	}

	// Process the accumulated data
	result, updated, err := re.ReplaceString(string(accumulated))
	if err != nil {
		return false, err
	}

	// Write the result
	_, writeErr := writer.Write([]byte(result))
	if writeErr != nil {
		return false, fmt.Errorf("error writing output: %v", writeErr)
	}

	re.updated = updated
	return updated, nil
}

// ReplaceInPlace performs replacement on a string slice, modifying it in place
func (re *ReplacementEngine) ReplaceInPlace(lines []string) (bool, error) {
	anyUpdated := false

	for i, line := range lines {
		result, updated, err := re.ReplaceString(line)
		if err != nil {
			return false, fmt.Errorf("error processing line %d: %v", i, err)
		}

		if updated {
			lines[i] = result
			anyUpdated = true
		}
	}

	re.updated = anyUpdated
	return anyUpdated, nil
}

// WasUpdated returns true if the last operation made any replacements
func (re *ReplacementEngine) WasUpdated() bool {
	return re.updated
}

// ValidateDFA checks that the DFA is well-formed
func (re *ReplacementEngine) ValidateDFA() error {
	if len(re.dfa) == 0 {
		return fmt.Errorf("DFA is empty")
	}

	for i, state := range re.dfa {
		// Check that final states have valid pattern indices
		if state.PatternIndex >= 0 {
			if state.PatternIndex >= re.patterns.PatternCount() {
				return fmt.Errorf("state %d has invalid pattern index %d", i, state.PatternIndex)
			}
		}

		// Check transition table
		for char, nextState := range state.Next {
			if nextState >= len(re.dfa) {
				return fmt.Errorf("state %d char %d has invalid next state %d", i, char, nextState)
			}
			// Note: negative values are allowed (final states or no transition)
		}
	}

	return nil
}

// GetStats returns statistics about the replacement engine
func (re *ReplacementEngine) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	stats["dfa_states"] = len(re.dfa)
	stats["patterns"] = re.patterns.PatternCount()

	// Count final states
	finalStates := 0
	for _, state := range re.dfa {
		if state.PatternIndex >= 0 {
			finalStates++
		}
	}
	stats["final_states"] = finalStates

	// Count total transitions
	transitions := 0
	for _, state := range re.dfa {
		for _, nextState := range state.Next {
			if nextState != -1 {
				transitions++
			}
		}
	}
	stats["transitions"] = transitions

	return stats
}

// DebugState returns debug information about a specific DFA state
func (re *ReplacementEngine) DebugState(stateIndex int) (string, error) {
	if stateIndex < 0 || stateIndex >= len(re.dfa) {
		return "", fmt.Errorf("invalid state index: %d", stateIndex)
	}

	state := re.dfa[stateIndex]
	var debug strings.Builder

	debug.WriteString(fmt.Sprintf("State %d:\n", stateIndex))
	debug.WriteString(fmt.Sprintf("  PatternIndex: %d\n", state.PatternIndex))
	debug.WriteString(fmt.Sprintf("  ReplaceString: %q\n", state.ReplaceString))
	debug.WriteString(fmt.Sprintf("  ToOffset: %d\n", state.ToOffset))
	debug.WriteString(fmt.Sprintf("  FromOffset: %d\n", state.FromOffset))
	debug.WriteString(fmt.Sprintf("  FoundType: %d\n", state.FoundType))

	debug.WriteString("  Transitions:\n")
	for char, nextState := range state.Next {
		if nextState != -1 {
			if char >= 32 && char <= 126 {
				debug.WriteString(fmt.Sprintf("    '%c' (%d) -> %d\n", char, char, nextState))
			} else {
				debug.WriteString(fmt.Sprintf("    (%d) -> %d\n", char, nextState))
			}
		}
	}

	return debug.String(), nil
}

// CompleteReplacer combines pattern processing, DFA building, and replacement
type CompleteReplacer struct {
	patterns *PatternProcessor
	engine   *ReplacementEngine
}

// NewCompleteReplacer creates a complete replacement system
func NewCompleteReplacer() *CompleteReplacer {
	return &CompleteReplacer{
		patterns: NewPatternProcessor(),
		engine:   nil,
	}
}

// AddPattern adds a replacement pattern
func (cr *CompleteReplacer) AddPattern(from, to string) error {
	return cr.patterns.AddPattern(from, to)
}

// Compile builds the DFA and prepares for replacement
func (cr *CompleteReplacer) Compile() error {
	if cr.patterns.PatternCount() == 0 {
		return fmt.Errorf("no patterns defined")
	}

	// Build the DFA
	builder := NewDFABuilder(cr.patterns)
	dfa, err := builder.BuildDFA()
	if err != nil {
		return fmt.Errorf("failed to build DFA: %v", err)
	}

	// Create the replacement engine
	cr.engine = NewReplacementEngine(dfa, cr.patterns)

	// Validate the DFA
	if err := cr.engine.ValidateDFA(); err != nil {
		return fmt.Errorf("DFA validation failed: %v", err)
	}

	return nil
}

// Replace performs string replacement
func (cr *CompleteReplacer) Replace(input string) (string, bool, error) {
	if cr.engine == nil {
		return "", false, fmt.Errorf("replacer not compiled - call Compile() first")
	}

	return cr.engine.ReplaceString(input)
}

// ReplaceReader performs replacement on data from reader to writer
func (cr *CompleteReplacer) ReplaceReader(reader io.Reader, writer io.Writer) (bool, error) {
	if cr.engine == nil {
		return false, fmt.Errorf("replacer not compiled - call Compile() first")
	}

	return cr.engine.ReplaceReader(reader, writer)
}

// GetStats returns statistics about the compiled replacer
func (cr *CompleteReplacer) GetStats() map[string]interface{} {
	if cr.engine == nil {
		return map[string]interface{}{
			"compiled": false,
			"patterns": cr.patterns.PatternCount(),
		}
	}

	stats := cr.engine.GetStats()
	stats["compiled"] = true
	return stats
}
