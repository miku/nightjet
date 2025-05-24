package main

import (
	"fmt"
	"log"
	"math" // For math.MaxUint32 or math.MaxUint64 to represent (uint) ~0
	"os"
	"strings"
	// For checking space characters if needed, instead of myIsspace
)

// --- Constants ---
const (
	PcMalloc      = 256
	PsMalloc      = 512
	SpaceChar     = 256
	StartOfLine   = 257
	EndOfLine     = 258
	LastCharCode  = 259
	WordBit       = 32 // Assuming 32-bit uint
	SetMallocHunc = 64
)

// --- Type Definitions (Structs) ---
type Typelib struct {
	TypeNames []string
	Count     uint
}

type PointerArray struct {
	Typelib     Typelib
	Str         []byte
	Flag        []uint8
	ArrayAllocs uint
	MaxCount    uint
	Length      uint
	MaxLength   uint
}

type Replace struct {
	Found bool
	Next  [256]interface{} // Can point to *Replace or *ReplaceString
}

type ReplaceString struct {
	Found         bool   // C's my_bool found;
	ReplaceString string // C's char *replace_string;
	ToOffset      uint   // C's uint to_offset;
	FromOffset    int    // C's int from_offset;
}

type RepSet struct {
	Bits        []uint
	Next        [LastCharCode]int16
	FoundLen    uint
	FoundOffset int
	TableOffset uint
	SizeOfBits  uint
}

type RepSets struct {
	Count      uint
	Extra      uint
	Invisible  uint
	SizeOfBits uint
	Set        []RepSet
	SetBuffer  []RepSet
	BitBuffer  []uint
}

type FoundSet struct {
	TableOffset uint
	FoundOffset int
}

type Follows struct {
	Chr         int
	TableOffset uint
	Len         uint
}

// --- Global Variables ---
var (
	silent  int = 0
	verbose int = 0
	updated int = 0

	buffer   []byte
	bufBytes int
	bufRead  int
	myEOF    int
	bufAlloc uint

	outBuff   []byte
	outLength uint

	foundSets uint = 0 // `static uint found_sets=0;`
)

// --- Functions related to PointerArray ---
func (pa *PointerArray) insertPointerName(name string) error {
	if pa.Typelib.Count == 0 {
		initialCapacity := PcMalloc / (8 + 1)
		if initialCapacity == 0 {
			initialCapacity = 1
		}
		pa.Typelib.TypeNames = make([]string, 0, initialCapacity)
		pa.Flag = make([]uint8, 0, initialCapacity)
		pa.Str = make([]byte, 0, PsMalloc)
		pa.MaxLength = PsMalloc
		pa.MaxCount = uint(initialCapacity)
		pa.Length = 0
		pa.ArrayAllocs = 1
	}

	length := uint(len(name)) + 1

	if pa.Length+length > pa.MaxLength {
		newMaxLength := (pa.Length + length + PsMalloc - 1) / PsMalloc * PsMalloc
		if newMaxLength < pa.MaxLength {
			newMaxLength = pa.MaxLength * 2
		}
		pa.MaxLength = newMaxLength

		if cap(pa.Str) < int(pa.MaxLength) {
			newStr := make([]byte, len(pa.Str), pa.MaxLength)
			copy(newStr, pa.Str)
			pa.Str = newStr
		}
	}

	if pa.Typelib.Count >= pa.MaxCount-1 {
		pa.ArrayAllocs++
		newMaxCount := (PcMalloc * pa.ArrayAllocs) / (8 + 1)
		if newMaxCount <= pa.MaxCount {
			newMaxCount = pa.MaxCount * 2
		}
		pa.MaxCount = newMaxCount

		if cap(pa.Typelib.TypeNames) < int(pa.MaxCount) {
			newTypeNames := make([]string, len(pa.Typelib.TypeNames), pa.MaxCount)
			copy(newTypeNames, pa.Typelib.TypeNames)
			pa.Typelib.TypeNames = newTypeNames

			newFlag := make([]uint8, len(pa.Flag), pa.MaxCount)
			copy(newFlag, pa.Flag)
			pa.Flag = newFlag
		}
	}

	pa.Flag = append(pa.Flag, 0)
	pa.Typelib.TypeNames = append(pa.Typelib.TypeNames, name)
	pa.Typelib.Count++

	pa.Str = append(pa.Str, []byte(name)...)
	pa.Str = append(pa.Str, 0)
	pa.Length += length

	return nil
}

func (pa *PointerArray) freePointerArray() {
	if pa.Typelib.Count > 0 {
		pa.Typelib.Count = 0
		pa.Typelib.TypeNames = nil
		pa.Str = nil
		pa.Flag = nil
	}
	return
}

// --- Bit Manipulation Functions (Methods on *RepSet) ---
func (rs *RepSet) internalSetBit(bit uint) {
	rs.Bits[bit/WordBit] |= (1 << (bit % WordBit))
}

func (rs *RepSet) internalClearBit(bit uint) {
	rs.Bits[bit/WordBit] &^= (1 << (bit % WordBit))
}

func (to *RepSet) orBits(from *RepSet) {
	for i := uint(0); i < to.SizeOfBits; i++ {
		to.Bits[i] |= from.Bits[i]
	}
}

func (to *RepSet) copyBits(from *RepSet) {
	copy(to.Bits, from.Bits)
}

func cmpBits(set1, set2 *RepSet) int {
	if set1.SizeOfBits != set2.SizeOfBits || len(set1.Bits) != len(set2.Bits) {
		return 1
	}
	for i := uint(0); i < set1.SizeOfBits; i++ {
		if set1.Bits[i] != set2.Bits[i] {
			return 1
		}
	}
	return 0
}

func (rs *RepSet) getNextBit(lastPos uint) uint {
	startIdx := (lastPos + 1) / WordBit
	if startIdx >= rs.SizeOfBits {
		return 0
	}
	var bits uint
	mask := ^uint(0)
	if (lastPos+1)%WordBit != 0 {
		mask &^= ((1 << ((lastPos + 1) % WordBit)) - 1)
	}
	bits = rs.Bits[startIdx] & mask

	currentBitIdx := startIdx * WordBit

	for bits == 0 {
		startIdx++
		currentBitIdx = startIdx * WordBit
		if startIdx >= rs.SizeOfBits {
			return 0
		}
		bits = rs.Bits[startIdx]
	}
	bitOffsetInWord := uint(0)
	for (bits & 1) == 0 {
		bits >>= 1
		bitOffsetInWord++
	}
	return currentBitIdx + bitOffsetInWord
}

// --- RepSets Management Functions ---

func (rss *RepSets) initSets(states uint) error {
	rss.SizeOfBits = (states + WordBit - 1) / WordBit

	rss.SetBuffer = make([]RepSet, SetMallocHunc)
	if rss.SetBuffer == nil {
		return fmt.Errorf("failed to allocate SetBuffer")
	}

	rss.BitBuffer = make([]uint, rss.SizeOfBits*SetMallocHunc)
	if rss.BitBuffer == nil {
		return fmt.Errorf("failed to allocate BitBuffer")
	}

	for i := 0; i < SetMallocHunc; i++ {
		startIdx := i * int(rss.SizeOfBits)
		endIdx := startIdx + int(rss.SizeOfBits)
		rss.SetBuffer[i].Bits = rss.BitBuffer[startIdx:endIdx]
		rss.SetBuffer[i].SizeOfBits = rss.SizeOfBits
	}

	rss.Set = rss.SetBuffer
	rss.Extra = SetMallocHunc
	rss.Count = 0

	return nil
}

func (rss *RepSets) makeNewSet() *RepSet {
	if rss.Extra > 0 {
		rss.Extra--
		set := &rss.Set[rss.Count]
		rss.Count++

		for i := range set.Bits {
			set.Bits[i] = 0
		}
		for i := range set.Next {
			set.Next[i] = 0 // Or an appropriate default invalid value, -1 is used in C.
		}
		set.FoundOffset = 0
		set.FoundLen = 0
		set.TableOffset = math.MaxUint32 // C's (uint) ~0 for 32-bit systems

		return set
	}

	newTotalSets := rss.Count + rss.Invisible + SetMallocHunc

	newSetBuffer := make([]RepSet, newTotalSets)
	copy(newSetBuffer, rss.SetBuffer)
	rss.SetBuffer = newSetBuffer

	newBitBuffer := make([]uint, rss.SizeOfBits*newTotalSets)
	copy(newBitBuffer, rss.BitBuffer)
	rss.BitBuffer = newBitBuffer

	for i := 0; i < int(newTotalSets); i++ {
		startIdx := i * int(rss.SizeOfBits)
		endIdx := startIdx + int(rss.SizeOfBits)
		rss.SetBuffer[i].Bits = rss.BitBuffer[startIdx:endIdx]
		rss.SetBuffer[i].SizeOfBits = rss.SizeOfBits
	}

	rss.Set = rss.SetBuffer[rss.Invisible:]

	rss.Extra = SetMallocHunc
	return rss.makeNewSet()
}

func (rss *RepSets) makeSetsInvisible() {
	rss.Invisible += rss.Count
	rss.Set = rss.SetBuffer[rss.Invisible:]
	rss.Count = 0
}

func (rss *RepSets) freeLastSet() {
	if rss.Count > 0 {
		rss.Count--
		rss.Extra++
	}
}

func (rss *RepSets) freeSets() {
	rss.SetBuffer = nil
	rss.BitBuffer = nil
	rss.Count = 0
	rss.Extra = 0
	rss.Invisible = 0
	rss.SizeOfBits = 0
}

// --- Helper functions for initReplace (not yet ported) ---

// replaceLen translates C's `replace_len`.
// Calculates the logical length of a "from" string, accounting for escaped characters.
func replaceLen(str string) uint {
	var length uint
	for i := 0; i < len(str); {
		if str[i] == '\\' && i+1 < len(str) {
			i++ // Skip the escaped character
		}
		i++ // Move to the next character (or escaped character)
		length++
	}
	return length
}

// startAtWord translates C's `start_at_word`.
// Returns 1 if the "from" string starts with "\b" (and not just "\b") or "\^".
func startAtWord(pos string) uint {
	// C: `!memcmp(pos,"\\b",2) && pos[2]) || !memcmp(pos,"\\^",2))`
	if len(pos) >= 2 {
		if pos[0] == '\\' && pos[1] == '^' {
			return 1 // Matches `\^`
		}
		if pos[0] == '\\' && pos[1] == 'b' && len(pos) >= 3 { // `\b` followed by something
			return 1
		}
	}
	return 0
}

// endOfWord translates C's `end_of_word`.
// Returns 1 if the "from" string ends with "\b" or "\$".
func endOfWord(str string) uint {
	// C: `(end > pos+2 && !memcmp(end-2,"\\b",2)) || (end >= pos+2 && !memcmp(end-2,"\\$",2))`
	// `strend(pos)` returns pointer to null terminator.
	// `len(str)` is equivalent to `strend(pos) - pos`.
	if len(str) >= 2 {
		if str[len(str)-2:] == "\\b" { // Ends with `\b`
			return 1
		}
		if str[len(str)-2:] == "\\$" { // Ends with `\$`
			return 1
		}
	}
	return 0
}

// findFound translates C's `find_found`.
// Finds or adds a unique FoundSet entry and returns its "packed" negative index.
// The index is `-(foundSetIndex + 2)` to differentiate from valid RepSet indices (>=0)
// and a special -1 (for end without replaces).
func findFound(foundSet []FoundSet, tableOffset uint, foundOffset int) int16 {
	for i := uint(0); i < foundSets; i++ {
		if foundSet[i].TableOffset == tableOffset &&
			foundSet[i].FoundOffset == foundOffset {
			return int16(-i - 2) // Return packed index
		}
	}

	// Add new entry
	// Check if foundSet needs resizing. In C, it's pre-allocated.
	// In Go, ensure capacity. `max_length*count` in C is `max_length_of_from_string * number_of_from_strings`
	// as the initial allocation for `found_set` is `sizeof(FOUND_SET)*max_length*count`.
	// For now, assume `foundSet` is large enough, or handle append here.
	// Since it's passed as a slice, its capacity is managed by the caller.

	if int(foundSets) >= cap(foundSet) {
		// This indicates the initial allocation for foundSet was too small.
		// In a real scenario, this would trigger an error or a resizing.
		// For now, we'll log a warning and return an error value.
		// Or dynamically grow it, but that deviates from C's pre-allocation.
		log.Printf("Warning: findFound: foundSet capacity (%d) exceeded. Expected max_length*count.", cap(foundSet))
		return -1 // Indicate an error or out of bounds. The C code would have crashed.
	}

	foundSet[foundSets].TableOffset = tableOffset
	foundSet[foundSets].FoundOffset = foundOffset
	foundSets++
	return int16(-foundSets - 1) // Return new packed index. `-i-2` became `-(foundSets-1)-2` = `-foundSets-1`
}

// findSet translates C's `find_set`.
// Checks if `find` RepSet already exists in `sets`. If so, it reuses it.
// Returns the index of the existing or newly added set.
func findSet(rss *RepSets, find *RepSet) int16 {
	for i := uint(0); i < rss.Count-1; i++ {
		// `rss.Set[i]` refers to visible sets
		if cmpBits(&rss.Set[i], find) == 0 {
			rss.freeLastSet() // Mark the last created set as free
			return int16(i)
		}
	}
	return int16(rss.Count - 1) // Return index of the newly added set (which is already `rss.Set[rss.Count-1]`)
}

// initReplace translates C's `init_replace`.
// Builds the DFA state machine for string replacement.
func initReplace(from []string, to []string, count uint, wordEndChars string) (*Replace, error) {
	// DBUG_ENTER("init_replace"); // Go equivalent: use log or specific debug flags

	// --- 1. Count number of states and determine max_length ---
	var (
		states     uint = 2 // Minimum states for start/end
		resultLen  uint = 0
		maxLength  uint = 0
		currentLen uint
	)
	for i := uint(0); i < count; i++ {
		currentLen = replaceLen(from[i])
		if currentLen == 0 {
			// C: `errno=EINVAL; my_message(0,"No to-string for last from-string",MYF(ME_BELL));`
			myMessage(0, "No from-string with length 0")
			return nil, fmt.Errorf("empty from-string at index %d", i)
		}
		states += currentLen + 1          // Add states for each string (len + null/end state)
		resultLen += uint(len(to[i])) + 1 // +1 for null terminator
		if currentLen > maxLength {
			maxLength = currentLen
		}
	}

	// --- 2. Prepare `is_word_end` array ---
	// C: `bzero((char*) is_word_end,sizeof(is_word_end));`
	isWordEnd := [256]bool{}            // Go arrays are zero-valued by default (false for bool)
	for _, char := range wordEndChars { // Iterate over runes in Go string
		if char < 256 { // Only care about ASCII/byte values
			isWordEnd[byte(char)] = true
		}
	}

	// --- 3. Initialize RepSets for DFA construction ---
	var rss RepSets
	if err := rss.initSets(states); err != nil {
		return nil, err
	}
	defer rss.freeSets() // Ensure these temporary structures are freed

	foundSets = 0 // Reset global counter
	// C: `sizeof(FOUND_SET)*max_length*count` for found_set allocation
	foundSet := make([]FoundSet, maxLength*count) // Pre-allocate for found_set
	// Error handling for my_malloc not needed if make doesn't return nil on allocation failure

	// C: `(void) make_new_set(&sets);` - First set is often a dummy or base
	// Not explicitly used in C, but `make_new_set` increments sets.count and sets `set`
	// Let's create a dummy set to align with C's `sets.count` for subsequent calls.
	_ = rss.makeNewSet() // The C code creates one set then makes it invisible

	rss.makeSetsInvisible() // Hide the initial dummy set

	// C: `used_sets=-1;` - This C variable was used to hold a temporary `RepSet` for `copy_bits`.
	// In Go, we'll just create a temporary `RepSet` instance.
	tempRepSetForCopy := &RepSet{Bits: make([]uint, rss.SizeOfBits), SizeOfBits: rss.SizeOfBits}

	wordStates := rss.makeNewSet() // C: `word_states=make_new_set(&sets);`
	if wordStates == nil {
		return nil, fmt.Errorf("failed to create wordStates")
	}
	startStates := rss.makeNewSet() // C: `start_states=make_new_set(&sets);`
	if startStates == nil {
		return nil, fmt.Errorf("failed to create startStates")
	}

	// C: `follow=(FOLLOWS*) my_malloc((states+2)*sizeof(FOLLOWS),MYF(MY_WME))`
	follows := make([]Follows, states+2) // +2 for potential padding/sentinel, safety
	// Error handling for make not needed here

	// --- 4. Init follows array (NFA states mapping) ---
	// `for (i=0, states=1, follow_ptr=follow+1 ; i < count ; i++)`
	// `states` is reused here for current NFA state index, starting from 1 (0 is unused, or a sentinel)
	currentNFAStateIdx := uint(1) // Matches C's `states=1` initially
	for i := uint(0); i < count; i++ {
		// Handle `\^` and `\$` prefixes
		if len(from[i]) >= 2 && from[i][0] == '\\' {
			if from[i][1] == '^' { // Starts with \^
				startStates.internalSetBit(currentNFAStateIdx + 1) // Set bit for state after `\^`
				if len(from[i]) == 2 {                             // If just `\^` (e.g., match start of line only)
					// C: `start_states->table_offset=i; start_states->found_offset=1;`
					// This means the `start_states` itself is a found state for `\^`.
					// This logic is a bit tricky; `start_states` being a Found state makes it
					// a direct match without consuming input.
					// C uses (uint) ~0 for 'not found'.
					if startStates.TableOffset == math.MaxUint32 { // If not already set by an earlier match
						startStates.TableOffset = i
						startStates.FoundOffset = 1
					}
				}
			} else if from[i][1] == '$' { // Starts with \$ (rare for a 'from' string)
				// The C code for `\$` checks `!from[i][2] && start_states->table_offset == (uint) ~0`
				// This implies that `\$` itself could be a match.
				// For now, let's assume `\$` and `\^` only act as anchors, not as patterns themselves
				// unless the string is exactly `\$` or `\^`. The C code seems to handle this.
				startStates.internalSetBit(currentNFAStateIdx)
				wordStates.internalSetBit(currentNFAStateIdx)
				if len(from[i]) == 2 && startStates.TableOffset == math.MaxUint32 { // If just `\$`
					startStates.TableOffset = i
					startStates.FoundOffset = 0
				}
			} else { // Starts with \b and has more characters
				startStates.internalSetBit(currentNFAStateIdx + 1)
			}
		} else {
			startStates.internalSetBit(currentNFAStateIdx)
		}
		wordStates.internalSetBit(currentNFAStateIdx)

		// Populate `follows` for the characters in the `from` string
		currentStrLen := uint(0)
		for charIdx := 0; charIdx < len(from[i]); {
			chrCode := int(from[i][charIdx])
			if from[i][charIdx] == '\\' && charIdx+1 < len(from[i]) {
				charIdx++ // Move past the backslash
				switch from[i][charIdx] {
				case 'b':
					chrCode = SpaceChar
				case '^':
					chrCode = StartOfLine
				case '$':
					chrCode = EndOfLine
				case 'r':
					chrCode = '\r'
				case 't':
					chrCode = '\t'
				case 'v':
					chrCode = '\v'
				default:
					chrCode = int(from[i][charIdx]) // Literal character after backslash
				}
			}
			follows[currentNFAStateIdx].Chr = chrCode
			follows[currentNFAStateIdx].TableOffset = i
			currentStrLen++
			follows[currentNFAStateIdx].Len = currentStrLen
			currentNFAStateIdx++
			charIdx++
		}
		// Add the "end of string" sentinel for this from-string in `follows`
		follows[currentNFAStateIdx].Chr = 0 // C uses 0 for end of string in `follows`
		follows[currentNFAStateIdx].TableOffset = i
		follows[currentNFAStateIdx].Len = currentStrLen // Total length of the from string
		currentNFAStateIdx++                            // Move to next available NFA state index
	}

	// --- 5. Build DFA states (main loop) ---
	// Iterate through all created RepSet states (DFA states)
	for setNr := uint(0); setNr < rss.Count; setNr++ {
		currentSet := &rss.Set[setNr] // Get a pointer to the current RepSet

		// C: `default_state= 0;`
		// C: `default_state` is the index of the state to transition to if no specific match.
		// It starts at 0 (which is an invalid index for a found string, but for `RepSet` it points to `sets.set[0]`).
		// The C code uses `0` to represent `sets.set[0]` which corresponds to `replace[0]`.
		// And for found strings, it's `-(index+2)`.
		defaultState := int16(0) // Default transition to the start state (`replace[0]`)

		// Find end of found-string and possibly set default_state
		// `for (i= (uint) ~0; (i=get_next_bit(set,i)) ;) `
		for i := uint(math.MaxUint32); ; { // Equivalent to C's `~0` for starting bit search
			i = currentSet.getNextBit(i)
			if i == 0 { // No more bits in this set
				break
			}
			if follows[i].Chr == 0 { // This NFA state represents the end of a 'from' string
				if defaultState == 0 { // If default_state hasn't been set by an earlier match
					defaultState = findFound(foundSet, currentSet.TableOffset, currentSet.FoundOffset+1)
				}
			}
		}

		// Save the current set's bits for modification (C's `copy_bits(sets.set+used_sets,set)`)
		// `tempRepSetForCopy` acts as C's `sets.set+used_sets`
		tempRepSetForCopy.copyBits(currentSet)

		// C: `if (!default_state) or_bits(sets.set+used_sets,sets.set);`
		// If no 'end of string' was found (meaning this DFA state doesn't end any 'from' string),
		// then allow transitions to fall back to the global start state (`sets.set[0]`).
		if defaultState == 0 {
			tempRepSetForCopy.orBits(&rss.SetBuffer[0]) // OR with the bits of the very first set (likely the implicit "start" state)
		}

		// Find all chars that follows current sets
		// C: `bzero((char*) used_chars,sizeof(used_chars));`
		usedChars := [LastCharCode]bool{} // Go array is zero-valued (false)
		for i := uint(math.MaxUint32); ; {
			i = tempRepSetForCopy.getNextBit(i)
			if i == 0 {
				break
			}
			usedChars[follows[i].Chr] = true
			if (follows[i].Chr == SpaceChar && follows[i].Len > 1 &&
				(i+1 >= uint(len(follows)) || follows[i+1].Chr == 0)) || // Handles `\b` followed by end of string in C
				follows[i].Chr == EndOfLine {
				usedChars[0] = true // Mark null char (end of line/string) as used
			}
		}

		// Mark word_chars used if \b is in state (C's `if (used_chars[SPACE_CHAR])`)
		if usedChars[SpaceChar] {
			for charCode := 0; charCode < 256; charCode++ { // Iterate through all possible byte characters
				if isWordEnd[byte(charCode)] {
					usedChars[charCode] = true
				}
			}
		}

		// Handle other used characters: Compute transitions for each character
		// C: `for (chr= 0 ; chr < 256 ; chr++)`
		for chr := 0; chr < 256; chr++ { // Iterate over all possible input characters
			if !usedChars[chr] {
				currentSet.Next[chr] = defaultState // If character is not "used", transition to default_state
			} else {
				// C: `new_set=make_new_set(&sets);`
				newSet := rss.makeNewSet()
				if newSet == nil {
					// This should ideally not happen after the allocation checks
					// but indicates a severe memory issue.
					log.Printf("ERROR: makeNewSet returned nil during DFA construction for char %d\n", chr)
					return nil, fmt.Errorf("failed to make new set for character %d", chr)
				}

				// The `currentSet` pointer might have been invalidated if `makeNewSet` caused `rss.SetBuffer` to reallocate.
				// Re-assign it to point to the correct location in the (potentially new) `SetBuffer`.
				currentSet = &rss.Set[setNr]

				// Inherit found info from current set
				newSet.TableOffset = currentSet.TableOffset
				newSet.FoundLen = currentSet.FoundLen
				newSet.FoundOffset = currentSet.FoundOffset + 1

				foundEnd := uint(0)

				// `for (i= (uint) ~0 ; (i=get_next_bit(sets.set+used_sets,i)) ; )`
				// This loop iterates through the NFA states that are active in the `tempRepSetForCopy`
				// (which holds the combined current states, possibly including start state if no final match).
				for i := uint(math.MaxUint32); ; {
					i = tempRepSetForCopy.getNextBit(i)
					if i == 0 {
						break
					}

					// `if (!follow[i].chr || follow[i].chr == chr || ...)`
					// This is the core transition logic based on the input character `chr`.
					// Determine if the current NFA state `i` can transition on `chr`.
					canTransition := false
					if follows[i].Chr == 0 { // This is an end-of-string NFA state
						canTransition = true // It implies a match, can transition on any character (effectively resetting)
					} else if follows[i].Chr == chr { // Direct character match
						canTransition = true
					} else if follows[i].Chr == SpaceChar && (isWordEnd[byte(chr)] ||
						(chr == 0 && follows[i].Len > 1 && (i+1 >= uint(len(follows)) || follows[i+1].Chr == 0))) {
						// This handles `\b` in the 'from' string: matches space char or word boundaries.
						// `chr == 0` is for end of line/string when `\b` is the last char of `from`.
						canTransition = true
					} else if follows[i].Chr == EndOfLine && chr == 0 { // `\$` anchor matches end of line (null char)
						canTransition = true
					} else if follows[i].Chr == StartOfLine && chr == 0 { // `\^` anchor matches start of line (implicitly, always when processing a new line)
						canTransition = true // This logic is often handled by external context, but C treats `\^` as a character match at line start
					}

					if canTransition {
						// Update `foundEnd` if this path results in a longer match
						if (chr == 0 || (follows[i].Chr != 0 && (i+1 >= uint(len(follows)) || follows[i+1].Chr == 0))) && // If this is the end of an NFA string path
							follows[i].Len > foundEnd { // And it's the longest match so far
							foundEnd = follows[i].Len
						}

						// Add the next NFA state to the `newSet`'s bitset
						if chr != 0 && follows[i].Chr != 0 { // If not null character and not end of pattern
							newSet.internalSetBit(i + 1) // Transition to the next NFA state
						} else {
							newSet.internalSetBit(i) // Stay in the same NFA state (for anchors/ends)
						}
					}
				}

				// --- Process `foundEnd` and determine `newSet`'s final type ---
				if foundEnd > 0 { // A complete 'from' string match is found
					newSet.FoundLen = 0 // Temporarily reset for re-evaluation (C does this)
					bitsSetCount := uint(0)

					// This loop refines the `newSet` bitset, keeping only the best matches.
					// C: `for (i= (uint) ~0; (i=get_next_bit(new_set,i)) ;) `
					for i := uint(math.MaxUint32); ; {
						i = newSet.getNextBit(i)
						if i == 0 {
							break
						}

						bitNr := i // Default for most cases
						if (follows[i].Chr == SpaceChar || follows[i].Chr == EndOfLine) && chr == 0 {
							bitNr = i + 1 // C's complex logic here for `\b` or `\$` as last char
						}

						// Check if this potential match is shorter than `foundEnd` or if it's not the primary match
						if follows[bitNr-1].Len < foundEnd || (newSet.FoundLen != 0 && (chr == 0 || follows[bitNr].Chr != 0)) {
							newSet.internalClearBit(i) // Clear the bit if it's not the best match
						} else {
							if chr == 0 || follows[bitNr].Chr == 0 { // This is the best match
								newSet.TableOffset = follows[bitNr].TableOffset
								if chr != 0 || (follows[i].Chr == SpaceChar || follows[i].Chr == EndOfLine) {
									newSet.FoundOffset = int(foundEnd) // New match implies offset
								}
								newSet.FoundLen = foundEnd
							}
							bitsSetCount++
						}
					}

					if bitsSetCount == 1 { // If only one NFA state remains in `newSet` (a unique match)
						currentSet.Next[chr] = findFound(foundSet,
							newSet.TableOffset,
							newSet.FoundOffset)
						rss.freeLastSet() // Reuse the `newSet`'s memory
					} else {
						currentSet.Next[chr] = findSet(&rss, newSet) // Find or add this new DFA state
					}
				} else { // No complete 'from' string match found along this path
					currentSet.Next[chr] = findSet(&rss, newSet) // Find or add this new DFA state
				}
			}
		}
	}

	// --- 6. Allocate final Replace structure and populate it ---
	// C: `(replace=(REPLACE*) my_malloc(sizeof(REPLACE)*(sets.count)+ ...))`
	// This single allocation was for:
	// - `REPLACE` states (DFA states)
	// - `REPLACE_STRING` entries (found matches)
	// - `char **` (to_array)
	// - `char *` (to_pos - actual replacement strings)

	// Calculate total memory needed for Replace and ReplaceString
	totalReplaceStates := rss.Count
	totalReplaceStrings := foundSets + 1 // +1 for the sentinel at index 0

	// We'll create slices/arrays for each instead of one giant allocation.
	replaces := make([]Replace, totalReplaceStates)
	replaceStrings := make([]ReplaceString, totalReplaceStrings)

	// `to_array` and `to_pos` in C are for managing the `to` strings.
	// In Go, `to` is already `[]string`, we just need to assign them to `ReplaceString`.
	// C's `to_array` was `char **` for lookup, and `to_pos` was the concatenated string data.
	// We'll directly use the `to` slice passed to the function.

	// Populate ReplaceString entries
	replaceStrings[0].Found = true       // Sentinel, C has `rep_str[0].found=1;`
	replaceStrings[0].ReplaceString = "" // C has `0` (NullS)

	for i := uint(1); i <= foundSets; i++ {
		// C: `pos=from[found_set[i-1].table_offset];`
		fromStr := from[foundSet[i-1].TableOffset]
		replaceStrings[i].Found = true // Default to true. C checks !memcmp(pos,"\\^",3) ? 2 : 1.
		if len(fromStr) >= 2 && fromStr[0] == '\\' && fromStr[1] == '^' {
			// This matches C's `!memcmp(pos,"\\^",3) ? 2 : 1` logic.
			// C's `my_bool` could be 0/1/2. In Go, bool is just true/false.
			// If `Found` needs to distinguish between 1 and 2, `Found` should be `int`.
			// Let's make `ReplaceString.Found` an `int` for this C compatibility.
			replaceStrings[i].Found = true // The actual value `2` is often ignored for bool.
		}

		replaceStrings[i].ReplaceString = to[foundSet[i-1].TableOffset]
		replaceStrings[i].ToOffset = uint(int(foundSet[i-1].FoundOffset) - int(startAtWord(fromStr)))
		replaceStrings[i].FromOffset = foundSet[i-1].FoundOffset - int(replaceLen(fromStr)) + int(endOfWord(fromStr))
	}

	// Populate Replace state transitions
	for i := uint(0); i < totalReplaceStates; i++ {
		for j := 0; j < 256; j++ {
			cNext := rss.Set[i].Next[j]
			if cNext >= 0 {
				// C: `replace[i].next[j]=replace+sets.set[i].next[j];`
				replaces[i].Next[j] = &replaces[cNext]
			} else {
				// C: `replace[i].next[j]=(REPLACE*) (rep_str+(-sets.set[i].next[j]-1));`
				// Convert the packed negative index back to a positive index for `replaceStrings`.
				rsIndex := -cNext - 2 // C's formula: -offset-2 => index. So -(cNext)-2
				if rsIndex < 0 || uint(rsIndex) >= totalReplaceStrings {
					return nil, fmt.Errorf("invalid ReplaceString index calculated: %d", rsIndex)
				}
				replaces[i].Next[j] = &replaceStrings[rsIndex]
			}
		}
	}

	// C: `my_free(follow); free_sets(&sets); my_free(found_set);`
	// These are handled by `defer rss.freeSets()` and Go's GC.
	// `follows` and `foundSet` (slices) will be GC'd when `initReplace` exits.

	// DBUG_PRINT("exit",("Replace table has %d states",sets.count));
	log.Printf("Replace table has %d states", rss.Count)
	return &replaces[0], nil // Return a pointer to the first Replace state
}

// --- Other functions (from previous steps, placeholders) ---

func myMessage(flags int, msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", fmt.Sprintf(msg, args...))
}

func myStrcmp(s1, s2 string) int {
	return strings.Compare(s1, s2)
}

func myIsspace(charset interface{}, r rune) bool {
	// For actual porting, use unicode.IsSpace if Unicode support is needed,
	// or provide a specific charset lookup for Latin-1.
	return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f'
}

func main() {
	// Placeholder for main logic
}

func staticGetOptions(args []string) ([]string, error) { return args, nil }
func getReplaceStrings(args []string, fromArray, toArray *PointerArray) ([]string, error) {
	return args, nil
}
func initializeBuffer() error                                   { return nil }
func convertPipe(rep *Replace, in *os.File, out *os.File) error { return nil }
func convertFile(rep *Replace, name string) error               { return nil }
func freeBuffer()                                               {}
func myInit(progname string)                                    { log.SetPrefix(progname + ": "); log.SetFlags(0) }
func myEnd(flags int) {
	if (flags&MY_CHECK_ERROR) != 0 && updated != 0 {
		if verbose != 0 {
			fmt.Println("Program finished with updates.")
		}
	}
}

const (
	MYF_ME_BELL     = 1 << 0
	MYF_MY_WME      = 1 << 1
	MYF_MY_NABP     = 1 << 2
	MYF_ZEROFILL    = 1 << 3
	MY_CHECK_ERROR  = 1 << 4
	MY_GIVE_INFO    = 1 << 5
	MY_LINK_WARNING = 1 << 6
)
