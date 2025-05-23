package main

import (
	"fmt"
)

// DFAState represents a single state in the DFA
type DFAState struct {
	// Transition table: next[char] -> next state index
	// Use -1 for no transition, >=0 for state index, <-1 for final state
	Next [256]int

	// For final states (when Next[char] < -1)
	PatternIndex  int    // which pattern this matches (-1 if not final)
	ReplaceString string // replacement text
	ToOffset      int    // how far back to start replacement
	FromOffset    int    // how far forward to continue after replacement
	FoundType     int    // 1=normal match, 2=start-of-line match
}

// FollowState represents what can follow in a pattern
type FollowState struct {
	Char         int // character code (or special like SpaceChar)
	PatternIndex int // which pattern this belongs to
	Position     int // position within that pattern
	Length       int // how many chars matched so far
}

// StateSet represents a set of possible states during DFA construction
type StateSet struct {
	States      *BitSet // which follow states are active
	TableOffset int     // if this is a final state, which pattern
	FoundOffset int     // how many characters matched
	FoundLength int     // length of best match found
}

// DFABuilder constructs the DFA from patterns
type DFABuilder struct {
	patterns     *PatternProcessor
	followStates []FollowState  // all possible follow states
	stateSets    []StateSet     // state sets during construction
	finalStates  []DFAState     // completed DFA states
	stateCache   map[string]int // cache for deduplicating states
	wordEndChars map[int]bool   // characters that end words
}

// NewDFABuilder creates a new DFA builder
func NewDFABuilder(patterns *PatternProcessor) *DFABuilder {
	builder := &DFABuilder{
		patterns:     patterns,
		followStates: make([]FollowState, 0),
		stateSets:    make([]StateSet, 0),
		finalStates:  make([]DFAState, 0),
		stateCache:   make(map[string]int),
		wordEndChars: make(map[int]bool),
	}

	// Initialize word-ending characters
	wordEndChars := " \t\n\r\v\f"
	for i := 0; i < len(wordEndChars); i++ {
		builder.wordEndChars[int(wordEndChars[i])] = true
	}
	// Also add the special character 0 (null/end of input)
	builder.wordEndChars[0] = true

	return builder
}

// BuildDFA constructs the complete DFA from patterns
func (b *DFABuilder) BuildDFA() ([]DFAState, error) {
	if err := b.patterns.ValidatePatterns(); err != nil {
		return nil, fmt.Errorf("pattern validation failed: %v", err)
	}
	// Step 1: Build follow states for all patterns
	if err := b.buildFollowStates(); err != nil {
		return nil, fmt.Errorf("failed to build follow states: %v", err)
	}
	// Step 2: Create initial state sets
	if err := b.createInitialStates(); err != nil {
		return nil, fmt.Errorf("failed to create initial states: %v", err)
	}
	// Step 3: Build the DFA by processing each state set
	if err := b.constructDFA(); err != nil {
		return nil, fmt.Errorf("failed to construct DFA: %v", err)
	}
	return b.finalStates, nil
}

// buildFollowStates creates follow states for all pattern positions
func (b *DFABuilder) buildFollowStates() error {
	followIndex := 1 // Start at 1, reserve 0 for special use

	for patternIdx, pattern := range b.patterns.GetPatterns() {
		parsedChars, err := b.patterns.ParsePattern(pattern.From)
		if err != nil {
			return fmt.Errorf("failed to parse pattern %d: %v", patternIdx, err)
		}

		// Create follow states for each position in the pattern
		for pos, char := range parsedChars {
			follow := FollowState{
				Char:         char.Char,
				PatternIndex: patternIdx,
				Position:     pos,
				Length:       pos + 1,
			}
			b.followStates = append(b.followStates, follow)
		}

		// Add end-of-pattern marker
		endFollow := FollowState{
			Char:         0, // end marker
			PatternIndex: patternIdx,
			Position:     len(parsedChars),
			Length:       len(parsedChars),
		}
		b.followStates = append(b.followStates, endFollow)

		followIndex += len(parsedChars) + 1
	}

	return nil
}

// createInitialStates sets up the starting states for the DFA
func (b *DFABuilder) createInitialStates() error {
	totalStates := len(b.followStates)
	// Create the starting state set (state 0)
	startSet := StateSet{
		States:      NewBitSet(totalStates),
		TableOffset: -1,
		FoundOffset: 0,
		FoundLength: 0,
	}
	// Create word-boundary state set (state 1)
	wordSet := StateSet{
		States:      NewBitSet(totalStates),
		TableOffset: -1,
		FoundOffset: 0,
		FoundLength: 0,
	}
	// Populate initial states based on pattern start conditions
	for i, follow := range b.followStates {
		if follow.Position == 0 { // Beginning of pattern
			pattern, _ := b.patterns.GetPattern(follow.PatternIndex)

			if pattern.StartsAtWord {
				// This pattern starts with \b or \^
				if follow.Char == StartOfLine {
					startSet.States.Set(i + 1) // Next state after start marker
				} else if follow.Char == SpaceChar {
					wordSet.States.Set(i + 1) // Next state after word boundary
				} else {
					// Pattern starts with \b but has content
					wordSet.States.Set(i)
				}
			} else {
				// Normal pattern, can start anywhere
				startSet.States.Set(i)
				wordSet.States.Set(i)
			}

			// Handle end-of-pattern immediately (for single-char patterns)
			if follow.Char == 0 {
				if startSet.TableOffset == -1 || follow.Length > startSet.FoundLength {
					startSet.TableOffset = follow.PatternIndex
					startSet.FoundOffset = 0
					startSet.FoundLength = follow.Length
				}
			}
		}
	}

	b.stateSets = append(b.stateSets, startSet, wordSet)
	return nil
}

// constructDFA builds the complete DFA by processing all state sets
func (b *DFABuilder) constructDFA() error {
	// Process each state set to create DFA states
	for setIndex := 0; setIndex < len(b.stateSets); setIndex++ {
		stateSet := &b.stateSets[setIndex]

		// Create the DFA state for this state set
		dfaState := DFAState{
			PatternIndex:  stateSet.TableOffset,
			ReplaceString: "",
			ToOffset:      0,
			FromOffset:    0,
			FoundType:     1,
		}

		// Initialize all transitions to -1 (no transition)
		for i := range dfaState.Next {
			dfaState.Next[i] = -1
		}

		// If this is a final state, set up replacement info
		if stateSet.TableOffset >= 0 {
			pattern, _ := b.patterns.GetPattern(stateSet.TableOffset)
			dfaState.ReplaceString = pattern.To
			dfaState.ToOffset = b.calculateToOffset(pattern, stateSet.FoundOffset)
			dfaState.FromOffset = b.calculateFromOffset(pattern, stateSet.FoundOffset)
			if pattern.StartsAtWord && stateSet.FoundOffset == 0 {
				dfaState.FoundType = 2 // start-of-line match
			}
		}

		// Build transitions for each possible input character
		if err := b.buildTransitions(&dfaState, stateSet); err != nil {
			return fmt.Errorf("failed to build transitions for state %d: %v", setIndex, err)
		}

		b.finalStates = append(b.finalStates, dfaState)
	}

	return nil
}

// buildTransitions creates transitions from a state set for each possible input character
func (b *DFABuilder) buildTransitions(dfaState *DFAState, stateSet *StateSet) error {
	// Track which characters have transitions
	usedChars := make(map[int]bool)

	// Find all characters that can transition from this state
	pos := -1
	for {
		pos = stateSet.States.NextBit(pos)
		if pos == -1 {
			break
		}
		if pos < len(b.followStates) {
			char := b.followStates[pos].Char
			usedChars[char] = true

			// Special handling for space character and word boundaries
			if char == SpaceChar {
				for wordChar := range b.wordEndChars {
					usedChars[wordChar] = true
				}
			}
		}
	}

	// Build transition for each used character
	for char := range usedChars {
		if char >= 0 && char < 256 {
			nextStateIndex, err := b.buildNextState(stateSet, char)
			if err != nil {
				return fmt.Errorf("failed to build next state for char %d: %v", char, err)
			}
			dfaState.Next[char] = nextStateIndex
		}
	}

	return nil
}

// buildNextState creates the next state for a given input character
func (b *DFABuilder) buildNextState(currentSet *StateSet, inputChar int) (int, error) {
	nextSet := StateSet{
		States:      NewBitSet(len(b.followStates)),
		TableOffset: -1,
		FoundOffset: 0,
		FoundLength: 0,
	}

	// Determine which follow states are reachable with this character
	pos := -1
	foundEnd := 0

	for {
		pos = currentSet.States.NextBit(pos)
		if pos == -1 {
			break
		}

		if pos >= len(b.followStates) {
			continue
		}

		follow := b.followStates[pos]
		matches := false

		// Check if this character matches the expected character
		if follow.Char == inputChar {
			matches = true
		} else if follow.Char == SpaceChar && b.wordEndChars[inputChar] {
			matches = true
		} else if follow.Char == EndOfLine && inputChar == 0 {
			matches = true
		}

		if matches {
			if follow.Char == 0 || (follow.Position+1 >= len(b.followStates)) {
				// End of pattern reached
				if foundEnd == 0 || follow.Length > foundEnd {
					foundEnd = follow.Length
					nextSet.TableOffset = follow.PatternIndex
					nextSet.FoundOffset = follow.Position + 1
					nextSet.FoundLength = follow.Length
				}
			} else {
				// Continue to next position in pattern
				nextSet.States.Set(pos + 1)
			}
		}
	}

	// Check if we can restart matching from the beginning
	if inputChar != 0 {
		// Add start states for patterns that can begin with this character
		startStates := &b.stateSets[0] // The initial start state
		nextSet.States.Or(startStates.States)
	}

	// Check if this state set already exists (deduplication)
	stateKey := b.stateSetKey(&nextSet)
	if existingIndex, exists := b.stateCache[stateKey]; exists {
		return existingIndex, nil
	}

	// This is a new state set
	newIndex := len(b.stateSets)
	b.stateSets = append(b.stateSets, nextSet)
	b.stateCache[stateKey] = newIndex

	// If this is a final state, return negative index to indicate final state
	if nextSet.TableOffset >= 0 {
		return -(newIndex + 2), nil // -2 offset to distinguish from -1 (no transition)
	}

	return newIndex, nil
}

// calculateToOffset determines how far back to start the replacement
func (b *DFABuilder) calculateToOffset(pattern Pattern, foundOffset int) int {
	// For patterns starting with word boundaries, adjust offset
	if pattern.StartsAtWord && foundOffset == 0 {
		return 1 // Start replacement after the boundary marker
	}
	return 0
}

// calculateFromOffset determines how far forward to continue after replacement
func (b *DFABuilder) calculateFromOffset(pattern Pattern, foundOffset int) int {
	offset := pattern.Length - foundOffset
	if pattern.EndsAtWord {
		offset-- // Account for end boundary marker
	}
	return offset
}

// stateSetKey generates a unique key for a state set (for deduplication)
func (b *DFABuilder) stateSetKey(set *StateSet) string {
	// Simple key based on which bits are set
	key := fmt.Sprintf("t%d:o%d:l%d:", set.TableOffset, set.FoundOffset, set.FoundLength)

	pos := -1
	for {
		pos = set.States.NextBit(pos)
		if pos == -1 {
			break
		}
		key += fmt.Sprintf("%d,", pos)
	}

	return key
}

// GetStateCount returns the number of DFA states created
func (b *DFABuilder) GetStateCount() int {
	return len(b.finalStates)
}

// GetFollowStateCount returns the number of follow states created
func (b *DFABuilder) GetFollowStateCount() int {
	return len(b.followStates)
}
