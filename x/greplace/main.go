package main

import (
	"fmt"
	"io" // For io.ReadFull, io.EOF
	"log"
	"math" // For math.MaxUint32
	"os"
	"strings"
	// For checking space characters
)

// --- Global Constants ---
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

// --- MYF_ Flags
// (Ensuring these are defined globally before any usage) ---
const (
	MYF_ME_BELL     = 1 << 0
	MYF_MY_WME      = 1 << 1
	MYF_MY_NABP     = 1 << 2
	MYF_ZEROFILL    = 1 << 3
	MY_CHECK_ERROR  = 1 << 4
	MY_GIVE_INFO    = 1 << 5
	MY_LINK_WARNING = 1 << 6
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
	Found         int // Corrected: Changed from bool to int to match C's usage (1 or 2)
	ReplaceString string
	ToOffset      uint
	FromOffset    int
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
	updated int = 0 // `static int updated=0;` - Indicates if any replacements were made

	// Buffer for file/pipe processing
	buffer   []byte // `static char *buffer;`
	bufBytes int    // `static int bufbytes;` - Number of bytes in the buffer.
	bufRead  int    // `static int bufread;` - Number of bytes to get with each read().
	myEOF    int    // `static int my_eof;` - Replaced C's `my_eof` (which was an int flag)
	bufAlloc uint   // `static uint bufalloc;` - Allocated size of `buffer`.
	// Output buffer
	outBuff   []byte     // `static char *out_buff;`
	outLength uint       // `static uint out_length;` - Allocated size of `out_buff`.
	foundSets uint   = 0 // `static uint found_sets=0;` - Count of unique found match results
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
	if set1.SizeOfBits != set2.SizeOfBits ||
		len(set1.Bits) != len(set2.Bits) {
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

// --- Helper functions for initReplace ---

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

func startAtWord(pos string) uint {
	if len(pos) >= 2 {
		if pos[0] == '\\' && pos[1] == '^' {
			return 1 // Matches `\^`
		}
		if pos[0] == '\\' && pos[1] == 'b' && len(pos) >= 3 {
			return 1
		}
	}
	return 0
}

func endOfWord(str string) uint {
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

func findFound(foundSet []FoundSet, tableOffset uint, foundOffset int) int16 {
	for i := uint(0); i < foundSets; i++ {
		if foundSet[i].TableOffset == tableOffset &&
			foundSet[i].FoundOffset == foundOffset {
			return int16(-i - 2) // Return packed index: C's formula -i-2
		}
	}

	if int(foundSets) >= cap(foundSet) {
		log.Printf("Warning: findFound: foundSet capacity (%d) exceeded. This will cause a panic if not handled by caller.", cap(foundSet))
		panic("foundSet capacity exceeded in findFound")
	}

	foundSet[foundSets].TableOffset = tableOffset
	foundSet[foundSets].FoundOffset = foundOffset
	foundSets++
	return int16(-foundSets - 1) // Return new packed index.
}

func findSet(rss *RepSets, find *RepSet) int16 {
	for i := uint(0); i < rss.Count-1; i++ {
		if cmpBits(&rss.Set[i], find) == 0 {
			rss.freeLastSet()
			return int16(i)
		}
	}
	return int16(rss.Count - 1)
}

func initReplace(from []string, to []string, count uint, wordEndChars string) ([]Replace, []ReplaceString, error) {
	log.Printf("initReplace: Initializing DFA for %d replacement pairs", count)
	var (
		states     uint = 2
		resultLen  uint = 0
		maxLength  uint = 0
		currentLen uint
	)
	for i := uint(0); i < count; i++ {
		currentLen = replaceLen(from[i])
		if currentLen == 0 {
			myMessage(0, "No from-string with length 0")
			return nil, nil, fmt.Errorf("empty from-string at index %d", i)
		}
		states += currentLen + 1
		resultLen += uint(len(to[i])) + 1
		if currentLen > maxLength {
			maxLength = currentLen
		}
		log.Printf("initReplace: Pair %d: from='%s' (len=%d), to='%s'", i, from[i], currentLen, to[i])
	}
	log.Printf("initReplace: Total states estimated: %d, Max from-string length: %d", states, maxLength)

	isWordEnd := [256]bool{}
	for _, char := range wordEndChars {
		if char < 256 {
			isWordEnd[byte(char)] = true
		}
	}

	var rss RepSets
	if err := rss.initSets(states); err != nil {
		return nil, nil, err
	}
	defer rss.freeSets()

	foundSets = 0
	foundSet := make([]FoundSet, maxLength*count+10) // Add some buffer

	// Create the initial state (this will be at index 0 in set_buffer)
	initialSet := rss.makeNewSet()
	if initialSet == nil {
		return nil, nil, fmt.Errorf("failed to create initial set")
	}

	// Make initial set invisible (this shifts the working sets)
	rss.makeSetsInvisible()

	// Now create the working sets
	wordStates := rss.makeNewSet()  // This becomes sets.set[0]
	startStates := rss.makeNewSet() // This becomes sets.set[1]
	if wordStates == nil || startStates == nil {
		return nil, nil, fmt.Errorf("failed to create word/start states")
	}

	follows := make([]Follows, states+2)

	// Build the follows array - this is the NFA representation
	currentNFAStateIdx := uint(1) // Start from state 1
	for i := uint(0); i < count; i++ {
		fromStr := from[i]

		// Handle special start patterns
		if len(fromStr) >= 2 && fromStr[0] == '\\' {
			if fromStr[1] == '^' {
				startStates.internalSetBit(currentNFAStateIdx + 1)
				if len(fromStr) == 2 {
					startStates.TableOffset = i
					startStates.FoundOffset = 1
				}
			} else if fromStr[1] == '$' {
				startStates.internalSetBit(currentNFAStateIdx)
				wordStates.internalSetBit(currentNFAStateIdx)
				if len(fromStr) == 2 {
					startStates.TableOffset = i
					startStates.FoundOffset = 0
				}
			} else if fromStr[1] == 'b' && len(fromStr) > 2 {
				startStates.internalSetBit(currentNFAStateIdx + 1)
			} else {
				startStates.internalSetBit(currentNFAStateIdx)
			}
		} else {
			startStates.internalSetBit(currentNFAStateIdx)
		}
		wordStates.internalSetBit(currentNFAStateIdx)

		// Process each character in the from-string
		currentStrLen := uint(0)
		for charIdx := 0; charIdx < len(fromStr); {
			chrCode := int(fromStr[charIdx])
			if fromStr[charIdx] == '\\' && charIdx+1 < len(fromStr) {
				charIdx++
				switch fromStr[charIdx] {
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
					chrCode = int(fromStr[charIdx])
				}
			}
			follows[currentNFAStateIdx].Chr = chrCode
			follows[currentNFAStateIdx].TableOffset = i
			currentStrLen++
			follows[currentNFAStateIdx].Len = currentStrLen
			currentNFAStateIdx++
			charIdx++
		}
		// Add the final state for this pattern
		follows[currentNFAStateIdx].Chr = 0 // End marker
		follows[currentNFAStateIdx].TableOffset = i
		follows[currentNFAStateIdx].Len = currentStrLen
		currentNFAStateIdx++
	}

	// Initialize the sets properly
	startStates.TableOffset = math.MaxUint32
	wordStates.TableOffset = math.MaxUint32

	// Build the DFA from the NFA
	tempRepSetForCopy := &RepSet{Bits: make([]uint, rss.SizeOfBits), SizeOfBits: rss.SizeOfBits}

	for setNr := uint(0); setNr < rss.Count; setNr++ {
		currentSet := &rss.Set[setNr]
		log.Printf("initReplace: Processing set %d (Count: %d)", setNr, rss.Count)

		// Find the default state for this set
		defaultState := int16(0)
		for i := uint(math.MaxUint32); ; {
			i = currentSet.getNextBit(i)
			if i == 0 {
				break
			}
			if int(i) < len(follows) && follows[i].Chr == 0 {
				if defaultState == 0 {
					defaultState = findFound(foundSet, follows[i].TableOffset, int(follows[i].Len))
					log.Printf("initReplace: Set %d: Found end state, defaultState=%d", setNr, defaultState)
				}
			}
		}

		// Copy current set for processing
		tempRepSetForCopy.copyBits(currentSet)

		// If no default state, or with the invisible initial set
		if defaultState == 0 {
			tempRepSetForCopy.orBits(&rss.SetBuffer[0])
		}

		// Find all characters that can transition from this state
		usedChars := [LastCharCode]bool{}
		for i := uint(math.MaxUint32); ; {
			i = tempRepSetForCopy.getNextBit(i)
			if i == 0 {
				break
			}
			if int(i) < len(follows) {
				usedChars[follows[i].Chr] = true
				// Special handling for SPACE_CHAR and END_OF_LINE
				if (follows[i].Chr == SpaceChar && follows[i].Len > 1 &&
					(int(i+1) >= len(follows) || follows[i+1].Chr == 0)) ||
					follows[i].Chr == EndOfLine {
					usedChars[0] = true
				}
			}
		}

		// If SPACE_CHAR is used, mark all word-end characters as used
		if usedChars[SpaceChar] {
			for charCode := 0; charCode < 256; charCode++ {
				if isWordEnd[byte(charCode)] {
					usedChars[charCode] = true
				}
			}
		}

		// Build transitions for each character
		for chr := 0; chr < 256; chr++ {
			if !usedChars[chr] {
				currentSet.Next[chr] = defaultState
			} else {
				newSet := rss.makeNewSet()
				if newSet == nil {
					return nil, nil, fmt.Errorf("failed to make new set for character %d", chr)
				}

				// Re-get currentSet as makeNewSet might reallocate
				currentSet = &rss.Set[setNr]

				newSet.TableOffset = currentSet.TableOffset
				newSet.FoundLen = currentSet.FoundLen
				newSet.FoundOffset = currentSet.FoundOffset + 1

				foundEnd := uint(0)

				// Process transitions for this character
				for i := uint(math.MaxUint32); ; {
					i = tempRepSetForCopy.getNextBit(i)
					if i == 0 {
						break
					}
					if int(i) >= len(follows) {
						continue
					}

					canTransition := false
					if follows[i].Chr == 0 {
						canTransition = true
					} else if follows[i].Chr == chr {
						canTransition = true
					} else if follows[i].Chr == SpaceChar && (isWordEnd[byte(chr)] ||
						(chr == 0 && follows[i].Len > 1 && (int(i+1) >= len(follows) || follows[i+1].Chr == 0))) {
						canTransition = true
					} else if follows[i].Chr == EndOfLine && chr == 0 {
						canTransition = true
					} else if follows[i].Chr == StartOfLine && chr == 0 {
						canTransition = true
					}

					if canTransition {
						if (chr == 0 || (follows[i].Chr != 0 && (int(i+1) >= len(follows) || follows[i+1].Chr == 0))) &&
							follows[i].Len > foundEnd {
							foundEnd = follows[i].Len
						}

						if chr != 0 && follows[i].Chr != 0 {
							newSet.internalSetBit(i + 1)
						} else {
							newSet.internalSetBit(i)
						}
					}
				}

				if foundEnd > 0 {
					newSet.FoundLen = 0
					bitsSetCount := uint(0)

					for i := uint(math.MaxUint32); ; {
						i = newSet.getNextBit(i)
						if i == 0 {
							break
						}
						if int(i) >= len(follows) {
							continue
						}

						bitNr := i
						if (follows[i].Chr == SpaceChar || follows[i].Chr == EndOfLine) && chr == 0 {
							bitNr = i + 1
						}

						if int(bitNr) == 0 || follows[bitNr-1].Len < foundEnd ||
							(newSet.FoundLen != 0 && (chr == 0 || (int(bitNr) < len(follows) && follows[bitNr].Chr != 0))) {
							newSet.internalClearBit(i)
						} else {
							if chr == 0 || (int(bitNr) < len(follows) && follows[bitNr].Chr == 0) {
								newSet.TableOffset = follows[bitNr].TableOffset
								if chr != 0 || (follows[i].Chr == SpaceChar || follows[i].Chr == EndOfLine) {
									newSet.FoundOffset = int(foundEnd)
								}
								newSet.FoundLen = foundEnd
							}
							bitsSetCount++
						}
					}

					if bitsSetCount == 1 {
						currentSet.Next[chr] = findFound(foundSet, newSet.TableOffset, newSet.FoundOffset)
						rss.freeLastSet()
						log.Printf("initReplace: Set %d, Char %d: Found final match, next state is %d", setNr, chr, currentSet.Next[chr])
					} else {
						currentSet.Next[chr] = findSet(&rss, newSet)
						log.Printf("initReplace: Set %d, Char %d: Next state is %d (found end, multiple bits)", setNr, chr, currentSet.Next[chr])
					}
				} else {
					currentSet.Next[chr] = findSet(&rss, newSet)
					log.Printf("initReplace: Set %d, Char %d: Next state is %d (no found end)", setNr, chr, currentSet.Next[chr])
				}
			}
		}
	}

	totalReplaceStates := rss.Count
	totalReplaceStrings := foundSets + 1

	log.Printf("initReplace: Building final structures: %d states, %d replace strings", totalReplaceStates, totalReplaceStrings)

	replaces := make([]Replace, totalReplaceStates)
	replaceStrings := make([]ReplaceString, totalReplaceStrings)

	// Set up the sentinel
	replaceStrings[0].Found = 1
	replaceStrings[0].ReplaceString = ""

	// Build the replace strings
	for i := uint(1); i <= foundSets; i++ {
		fromStr := from[foundSet[i-1].TableOffset]

		if len(fromStr) >= 2 && fromStr[0] == '\\' && fromStr[1] == '^' && len(fromStr) == 2 {
			replaceStrings[i].Found = 2
		} else {
			replaceStrings[i].Found = 1
		}

		replaceStrings[i].ReplaceString = to[foundSet[i-1].TableOffset]
		replaceStrings[i].ToOffset = uint(foundSet[i-1].FoundOffset - int(startAtWord(fromStr)))
		replaceStrings[i].FromOffset = foundSet[i-1].FoundOffset - int(replaceLen(fromStr)) + int(endOfWord(fromStr))
		log.Printf("initReplace: ReplaceString %d: from='%s', to='%s', toOffset=%d, fromOffset=%d",
			i, fromStr, replaceStrings[i].ReplaceString, replaceStrings[i].ToOffset, replaceStrings[i].FromOffset)
	}

	// Build the transition table
	for i := uint(0); i < totalReplaceStates; i++ {
		for j := 0; j < 256; j++ {
			cNext := rss.Set[i].Next[j]
			if cNext >= 0 {
				replaces[i].Next[j] = &replaces[cNext]
			} else {
				rsIndex := -cNext - 2
				if rsIndex < 0 || uint(rsIndex) >= totalReplaceStrings {
					return nil, nil, fmt.Errorf("invalid ReplaceString index calculated: %d", rsIndex)
				}
				replaces[i].Next[j] = &replaceStrings[rsIndex]
			}
		}
	}

	log.Printf("Replace table has %d states, %d replace strings", rss.Count, foundSets)
	return replaces, replaceStrings, nil
}

// --- Buffer Management for I/O ---

// initializeBuffer translates C's `initialize_buffer`.
// Sets up the input and output buffers.
func initializeBuffer() error {
	bufRead = 8192                       // Default read chunk size
	bufAlloc = uint(bufRead + bufRead/2) // C's bufalloc = bufread + bufread/2
	buffer = make([]byte, bufAlloc+1)    // +1 for sentinel byte (null terminator)
	bufBytes = 0
	myEOF = 0

	outLength = uint(bufRead) // Initial size for output buffer
	outBuff = make([]byte, outLength)
	if outBuff == nil { // In Go, make() typically panics on OOM, so nil check is less common but good
		return fmt.Errorf("failed to allocate outBuff")
	}
	return nil
}

// resetBuffer translates C's `reset_buffer`.
// Resets the state of the input buffer.
func resetBuffer() {
	bufBytes = 0
	myEOF = 0
}

// freeBuffer translates C's `free_buffer`.
// Releases the memory associated with the global buffers.
func freeBuffer() {
	buffer = nil
	outBuff = nil
}

// fillBufferRetaining translates C's `fill_buffer_retaining`.
// Fills the buffer from the reader, retaining the last `n` bytes at the beginning.
// Returns the number of new bytes read, or -1 on error.
func fillBufferRetaining(reader io.Reader, n int) int {
	// DBUG_ENTER("fill_buffer_retaining");
	// Go equivalent: log debug

	// See if we need to grow the buffer.
	// C: `if ((int) bufalloc - n <= bufread)`
	if int(bufAlloc)-n <= bufRead {
		for int(bufAlloc)-n <= bufRead {
			bufAlloc *= 2
			bufRead *= 2 // C also doubles bufread
		}
		// Reallocate buffer with new size.
		// `buffer = my_realloc(buffer, bufalloc+1, MYF(MY_WME));`
		newBuffer := make([]byte, bufAlloc+1)
		copy(newBuffer, buffer[:bufBytes]) // Copy existing data to the new buffer
		buffer = newBuffer
		if buffer == nil { // Check for allocation failure (though make typically panics)
			return -1
		}
	}

	// Shift stuff down.
	// `bmove(buffer,buffer+bufbytes-n,(uint) n);`
	// In Go, this is `copy(destination, source)`
	if n > 0 && bufBytes >= n {
		copy(buffer[0:n], buffer[bufBytes-n:bufBytes])
	}
	bufBytes = n // Update bufBytes to reflect only the retained bytes

	if myEOF != 0 { // If end-of-file was previously reached
		return 0 // No new bytes to read
	}

	// Read in new stuff.
	// `if ((i=(int) my_read(fd, (uchar*) buffer + bufbytes, (size_t) bufread, MYF(MY_WME))) < 0)`
	// `io.ReadFull` attempts to read exactly `bufRead` bytes.
	// `reader.Read` reads up to `bufRead` bytes.
	// The C `my_read` reads exactly `bufread` bytes if possible or returns less/error.
	// `reader.Read` is closer to `my_read`'s behavior (reads up to len(p) bytes).
	nRead, err := reader.Read(buffer[bufBytes : bufBytes+bufRead])
	if err != nil && err != io.EOF {
		// Log specific error for debugging
		log.Printf("Error reading from input: %v", err)
		return -1 // Indicate an error
	}

	// Kludge to pretend every nonempty file ends with a newline.
	// C: `if (i == 0 && bufbytes > 0 && buffer[bufbytes - 1] != '\n')`
	if nRead == 0 && bufBytes > 0 && buffer[bufBytes-1] != '\n' {
		myEOF = 1 // Mark EOF
		// C: `my_eof = i = 1;
		// buffer[bufbytes] = '\n';`
		// This means it pretends to read 1 byte (`\n`) if it was EOF and didn't end with newline.
		// This is a special case for `grep`-like behavior.
		buffer[bufBytes] = '\n'
		nRead = 1 // Pretend 1 byte was read for the artificial newline
	} else if err == io.EOF {
		myEOF = 1 // Mark EOF if actual EOF reached
	}

	bufBytes += nRead
	return nRead
}

// replaceStrings translates C's `replace_strings`.
// The core function that performs string replacements using the DFA.
// It modifies `out` (which points to `outBuff`) and adjusts `max_length` (`outLength`).
// Returns the actual length of the data written to `outBuff`, or -1 on error.
// (Using math.MaxUint32 for -1)
func replaceStrings(rep *Replace, out *[]byte, maxLength *uint, from []byte) uint {
	log.Printf("replaceStrings: Processing line (len %d): '%s'", len(from), string(from))
	var repPos interface{}
	repPos = rep // Initialize with the starting Replace state (which is &replaces[0])

	currentOutPtr := uint(0)               // Logical length of content in `*out`
	outBufferEndIdx := int(*maxLength) - 1 // Index of last valid byte in `*out`

	for fromPtr := 0; ; { // `fromPtr` is the current read position in `from`
		log.Printf("replaceStrings: Loop start: fromPtr=%d, currentOutPtr=%d, repPosType=%T", fromPtr, currentOutPtr, repPos)

		// Inner loop: Advance DFA state until a match is found (`rep_pos.Found` is true)
		for {
			// Type assert repPos to *Replace to access its 'Found' field and 'Next' array
			currentReplaceState, ok := repPos.(*Replace)
			if !ok {
				// If it's not *Replace, it must be *ReplaceString (a match)
				log.Printf("replaceStrings: Match found! repPos is *ReplaceString. Breaking inner loop.")
				break
			}
			if currentReplaceState.Found { // If Found is true for a *Replace (shouldn't happen with correct DFA)
				log.Printf("replaceStrings: Unexpected: *Replace state has Found=true. This indicates a DFA construction issue or logic error. Breaking.")
				break
			}

			// Determine the character to process for DFA transition
			var charToProcess byte
			if fromPtr < len(from) {
				charToProcess = from[fromPtr]
				log.Printf("replaceStrings: Processing char '%c' (byte %d) at fromPtr=%d", charToProcess, charToProcess, fromPtr)
			} else {
				charToProcess = 0 // End of input line, use null char for DFA
				log.Printf("replaceStrings: End of line, processing null char (byte %d) at fromPtr=%d", charToProcess, fromPtr)
			}

			// Advance DFA state: `rep_pos = rep_pos->next[(uchar) *from];`
			nextState := currentReplaceState.Next[charToProcess]
			log.Printf("replaceStrings: Transitioning from %T to %T for char %d", repPos, nextState, charToProcess)
			repPos = nextState

			// If current position in output buffer exceeds its capacity, reallocate.
			if int(currentOutPtr) >= outBufferEndIdx {
				*maxLength *= 2 // Double the output buffer size
				newOut := make([]byte, *maxLength)
				copy(newOut, (*out)[:currentOutPtr]) // Copy already processed output
				*out = newOut
				outBufferEndIdx = int(*maxLength) - 1
				log.Printf("replaceStrings: Output buffer reallocated to %d bytes.", *maxLength)
			}

			// Copy character from input to output unless it's a null sentinel marking end of line
			if fromPtr < len(from) {
				(*out)[currentOutPtr] = charToProcess
				currentOutPtr++
				fromPtr++
				log.Printf("replaceStrings: Copied char. currentOutPtr=%d, fromPtr=%d", currentOutPtr, fromPtr)
			} else {
				// If we've processed all input characters from `from`, but the DFA hasn't found a match
				// and is still asking for more input (i.e., `repPos.Found` is still false after processing `0`),
				// it implies the current line doesn't lead to a match and we are at its end.
				// This is the signal to exit the entire `replaceStrings` loop for this line.
				log.Printf("replaceStrings: End of input line reached without match. Returning current output length %d.", currentOutPtr)
				return currentOutPtr // No match found for the rest of the line, return current output length.
			}
		}

		// A match or end of line reached. `repPos` is either a `*ReplaceString` (found match)
		// or, if `repPos.Found` became true for a `*Replace` struct, it's an error.
		// The DFA construction should ensure `Found == true` only for `*ReplaceString` or `rep` (base).

		repString, isReplaceString := repPos.(*ReplaceString)

		// C: `if (!(rep_str = ((REPLACE_STRING*) rep_pos))->replace_string)`
		if !isReplaceString || repString.ReplaceString == "" { // Check if it's the sentinel `ReplaceString` (empty string)
			log.Printf("replaceStrings: Sentinel ReplaceString encountered (empty replace_string). Returning current output length %d.", currentOutPtr)
			return currentOutPtr // This is the length of processed part of the output buffer.
		}

		updated = 1 // Some char is replaced (C's updated=1)
		log.Printf("replaceStrings: Replacement detected! Original fromPtr: %d, currentOutPtr: %d", fromPtr, currentOutPtr)
		log.Printf("replaceStrings: ReplaceString: '%s', ToOffset: %d, FromOffset: %d", repString.ReplaceString, repString.ToOffset, repString.FromOffset)

		// C: `to-=rep_str->to_offset;`
		currentOutPtr -= repString.ToOffset // Adjust output pointer backward
		log.Printf("replaceStrings: Adjusted currentOutPtr back by %d to %d", repString.ToOffset, currentOutPtr)

		// Copy replacement string to output
		// C: `for (pos=rep_str->replace_string; *pos ; pos++)`
		for _, charRune := range repString.ReplaceString { // Iterate over runes in Go string
			char := byte(charRune) // Assuming single-byte characters like in original C code
			// Check output buffer capacity
			if int(currentOutPtr) >= outBufferEndIdx {
				*maxLength *= 2 // Double the output buffer size
				newOut := make([]byte, *maxLength)
				copy(newOut, (*out)[:currentOutPtr])
				*out = newOut
				outBufferEndIdx = int(*maxLength) - 1
				log.Printf("replaceStrings: Output buffer reallocated during replacement to %d bytes.", *maxLength)
			}
			(*out)[currentOutPtr] = char // Copy character
			currentOutPtr++
		}
		log.Printf("replaceStrings: Copied replacement string. New currentOutPtr=%d", currentOutPtr)

		// Adjust input pointer for the next scan.
		// C: `if (!*(from-=rep_str->from_offset) && rep_pos->found != 2)`
		fromPtr -= repString.FromOffset // `from` pointer adjustment
		if fromPtr < 0 {                // Ensure fromPtr doesn't go negative
			fromPtr = 0
		}
		log.Printf("replaceStrings: Adjusted fromPtr back by %d to %d", repString.FromOffset, fromPtr)

		// C's `!*(from-=rep_str->from_offset)` means the character at `from` (after adjustment) is null (end of line).
		// If `fromPtr` reaches the end of the input `from` slice (current line content) AND
		// the `repString.Found` flag is not 2 (which indicates `\^` only match that needs continuation).
		if fromPtr >= len(from) && repString.Found != 2 {
			log.Printf("replaceStrings: End of input reached after replacement. Returning current output length %d.", currentOutPtr)
			return currentOutPtr // Return actual length of processed output.
		}

		// Reset DFA state for next scan
		// C: `rep_pos=rep;` (rep is the base address of the DFA states, points to rep[0])
		repPos = rep // Reset to the initial state (which is &replaces[0])
		log.Printf("replaceStrings: Resetting repPos to initial state for next scan.")
	}
}

// --- File I/O and Conversion ---

// convertPipe translates C's `convert_pipe`.
// Processes input from a reader (stdin) to a writer (stdout).
func convertPipe(rep *Replace, in io.Reader, out io.Writer) int {
	log.Printf("convertPipe: Starting pipe conversion.")
	updated = 0 // Reset global updated flag
	retain := 0
	resetBuffer() // Reset global buffer state

	for {
		log.Printf("convertPipe: Calling fillBufferRetaining with retain=%d", retain)
		bytesRead := fillBufferRetaining(in, retain)
		log.Printf("convertPipe: fillBufferRetaining returned %d bytes. bufBytes=%d, myEOF=%d", bytesRead, bufBytes, myEOF)

		if bytesRead < 0 { // Error
			log.Printf("convertPipe: Error from fillBufferRetaining, returning 1.")
			return 1
		}
		if bytesRead == 0 && myEOF != 0 && bufBytes == 0 { // No more data and actual EOF, and buffer is empty
			log.Printf("convertPipe: End of file and empty buffer. Breaking loop.")
			break // Exit loop
		}
		if bufBytes == 0 && bytesRead == 0 && myEOF == 0 { // No data read, not EOF yet (can happen with empty input)
			log.Printf("convertPipe: No data read, not EOF. Could be stalled or empty input. Breaking.")
			break // Could be stalled, exit.
		}

		// C: `buffer[bufbytes]=0;` (Sentinel for C strings)
		// Go strings/slices don't rely on null terminators.
		// But the DFA logic uses `0` as end-of-string character.
		// Ensure a conceptual null terminator at `buffer[bufBytes]` for DFA logic.
		// Make sure `buffer` has capacity for `bufBytes+1`.
		// `bufAlloc+1` should cover it.
		if bufBytes < len(buffer) {
			buffer[bufBytes] = 0
			log.Printf("convertPipe: Added null sentinel at buffer[%d]", bufBytes)
		}

		endOfLinePtr := 0 // Index into `buffer` for current line processing
		for {
			startOfLinePtr := endOfLinePtr
			// Find end of line (newline or null terminator from sentinel)
			// C: `while (end_of_line[0] != '\n' && end_of_line[0])`
			log.Printf("convertPipe: Scanning for end of line from index %d (bufBytes=%d)", endOfLinePtr, bufBytes)
			for endOfLinePtr < bufBytes && buffer[endOfLinePtr] != '\n' && buffer[endOfLinePtr] != 0 {
				endOfLinePtr++
			}
			log.Printf("convertPipe: End of line found at index %d", endOfLinePtr)

			if endOfLinePtr == bufBytes { // Reached end of currently buffered data
				// C: `retain= (int) (end_of_line - start_of_line);
				// break;`
				retain = bufBytes - startOfLinePtr // Amount to retain for next read
				log.Printf("convertPipe: End of buffered data. Retaining %d bytes. Breaking inner loop.", retain)
				break // Break inner loop, go read more data
			}

			// C: `save_char=end_of_line[0];
			// end_of_line[0]=0; end_of_line++;`
			saveChar := buffer[endOfLinePtr] // Save the newline or null char
			log.Printf("convertPipe: Saved char '%c' (byte %d) at endOfLinePtr=%d", saveChar, saveChar, endOfLinePtr)
			// `buffer[endOfLinePtr] = 0` is conceptually done when we pass a subslice
			// (or handle it as a sentinel for DFA).
			// The `replaceStrings` function receives the slice up to this point.
			endOfLinePtr++ // Move past the newline/null for the next line start

			// Pass the line content to `replaceStrings`.
			// The `replaceStrings` function expects `from` as a byte slice.
			// The `from` slice should be just the content, not including the newline/null.
			lineContent := buffer[startOfLinePtr : endOfLinePtr-1]
			log.Printf("convertPipe: Calling replaceStrings for line: '%s'", string(lineContent))
			length := replaceStrings(rep, &outBuff, &outLength, lineContent)
			log.Printf("convertPipe: replaceStrings returned length %d", length)

			if length == math.MaxUint32 { // Error indicated by `math.MaxUint32` (-1 in C)
				log.Printf("convertPipe: Error from replaceStrings, returning 1.")
				return 1
			}

			// C: `if (!my_eof) out_buff[length++]=save_char;`
			// Append the saved newline/null character if not true EOF.
			if myEOF == 0 || (myEOF == 1 && saveChar == '\n') { // If not actual EOF, or it's a real newline at EOF
				if uint(len(outBuff)) <= length { // Ensure capacity
					// Dynamically grow outBuff if needed before appending single char
					*&outLength *= 2
					newOutBuff := make([]byte, *&outLength)
					copy(newOutBuff, outBuff[:length])
					outBuff = newOutBuff
					log.Printf("convertPipe: Output buffer reallocated for newline to %d bytes.", *&outLength)
				}
				outBuff[length] = saveChar // Add back the original line terminator
				length++
				log.Printf("convertPipe: Appended saved char. New length=%d", length)
			}

			// C: `if (my_fwrite(out, (uchar*) out_buff, length, MYF(MY_WME | MY_NABP)))`
			log.Printf("convertPipe: Writing %d bytes to output: '%s'", length, string(outBuff[:length]))
			_, err := out.Write(outBuff[:length])
			if err != nil {
				log.Printf("Error writing to output: %v", err)
				return 1
			}
		}
	}
	log.Printf("convertPipe: Pipe conversion finished successfully.")
	return 0 // Success
}

// convertFile translates C's `convert_file`.
// Opens a file, performs replacement, and writes to a temporary file, then renames.
func convertFile(rep *Replace, name string) int {
	log.Printf("convertFile: Starting file conversion for '%s'", name)
	var (
		in  *os.File
		out *os.File
		err error
	)

	orgName := name // Assuming name is the original path

	// check if name is a symlink (C's #ifdef HAVE_READLINK)
	if !myDisableSymlinks { // Assuming myDisableSymlinks global is available (mocked to false)
		if linkedPath, linkErr := os.Readlink(name); linkErr == nil {
			orgName = linkedPath // Follow symlink
			log.Printf("convertFile: Symlink detected, using original path '%s'", orgName)
		}
	}

	in, err = os.Open(orgName)
	if err != nil {
		myMessage(MYF_MY_WME, "Failed to open input file %s: %v", orgName, err)
		return 1
	}
	defer in.Close() // Ensure input file is closed

	// Go's `os.CreateTemp` handles this more simply.
	// It's good practice to create temp file in the same directory as original to allow atomic rename.
	tempFile, err := os.CreateTemp(os.TempDir(), "PR_") // "PR_" prefix, default temp directory
	if err != nil {
		myMessage(MYF_MY_WME, "Failed to create temporary file: %v", err)
		return 1
	}
	tempname := tempFile.Name() // Get the name of the temporary file
	defer os.Remove(tempname)   // Ensure temp file is cleaned up if rename fails or exits early
	log.Printf("convertFile: Created temporary file: '%s'", tempname)

	out = tempFile                        // os.CreateTemp returns *os.File, which can be used directly as a writer.
	errorVal := convertPipe(rep, in, out) // Perform replacement
	out.Close()                           // Explicitly close output before rename/delete
	log.Printf("convertFile: convertPipe finished with errorVal=%d", errorVal)

	if updated != 0 && errorVal == 0 { // C: `if (updated && ! error)`
		// C: `my_redel(org_name,tempname,MYF(MY_WME | MY_LINK_WARNING));`
		// Atomically replace the original file with the temporary one.
		err = os.Rename(tempname, orgName)
		if err != nil {
			myMessage(MYF_MY_WME|MY_LINK_WARNING, "Failed to rename temporary file to %s: %v", orgName, err)
			return 1
		}
		log.Printf("convertFile: Renamed '%s' to '%s'", tempname, orgName)
	} else {
		// C: `my_delete(tempname,MYF(MY_WME));`
		os.Remove(tempname) // Delete temporary file if not updated or if there was an error
		log.Printf("convertFile: Deleted temporary file '%s' (updated=%d, errorVal=%d)", tempname, updated, errorVal)
	}

	if silent == 0 && errorVal == 0 { // C: `if (!silent && ! error)`
		if updated != 0 {
			fmt.Printf("%s converted\n", name)
		} else if verbose != 0 {
			fmt.Printf("%s left unchanged\n", name)
		}
	}
	log.Printf("convertFile: Finished file conversion for '%s'", name)
	return errorVal // Return 0 for success, 1 for error
}

// --- General Utility Functions (Ensuring these are defined globally and correctly) ---

// myMessage is a placeholder for C's my_message function.
// CORRECTED: Enhanced to handle format arguments only when expected by the message string.
func myMessage(flags int, msg string, args ...interface{}) {
	// Heuristic: Check if the message string contains any format verbs.
	// If it does, use Sprintf. Otherwise, just print the message (and optional extra args).
	if strings.ContainsRune(msg, '%') {
		fmt.Fprintf(os.Stderr, "Error: %s\n", fmt.Sprintf(msg, args...))
	} else if len(args) > 0 {
		// If there are no format verbs but extra arguments were passed (like MYF_ME_BELL in C),
		// print the message and then the "extra" arguments for debugging/info.
		fmt.Fprintf(os.Stderr, "Error: %s (flags: %v)\n", msg, flags) // Or just `args` if flags is separate.
	} else {
		fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	}
	// The 'flags' parameter (like MYF_ME_BELL) is currently received but its specific behavior
	// (e.g., ringing a bell) is not implemented in this Go port, but it no longer causes errors.
}

// myInit is a placeholder for C's my_init.
func myInit(progname string) {
	myProgname = progname
	log.SetPrefix(progname + ": ")
	log.SetFlags(0) // No timestamp by default, adjust as needed
}

// myEnd is a placeholder for C's my_end.
func myEnd(flags int) {
	if (flags&MY_CHECK_ERROR) != 0 && updated != 0 {
		if verbose != 0 {
			fmt.Println("Program finished with updates.")
		}
	}
	// In Go, defer statements handle resource cleanup;
	// os.Exit terminates.
}

// Dummy for `strcmp` from C's `string.h` for use in `getReplaceStrings`
func myStrcmp(s1, s2 string) int {
	return strings.Compare(s1, s2)
}

// Dummy for `my_isspace` from `m_ctype.h` for use in `main`.
func myIsspace(charset interface{}, r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' ||
		r == '\r' || r == '\v' || r == '\f'
}

// `my_disable_symlinks` from C's `my_sys.h`
var myDisableSymlinks = false

// `my_progname` from C's `my_global.h`
var myProgname = "replace_strings" // Default, will be set in main

func main() {
	myInit(os.Args[0]) // Initialize program name for logging

	cliArgs := os.Args[1:] // All arguments after program name

	// Separate arguments into flags, replacement pairs, and explicit filenames.
	fromToPairs := []string{}
	finalFileNames := []string{}

	parsingFlagsAndFromTo := true // State flag to distinguish between initial args and args after '--'
	var help bool

	for i := 0; i < len(cliArgs); i++ {
		arg := cliArgs[i]
		if parsingFlagsAndFromTo && len(arg) > 1 && arg[0] == '-' {
			if arg == "--" {
				// Found the -- delimiter, switch to parsing explicit filenames
				parsingFlagsAndFromTo = false
				continue
			}
			// Process short flags (e.g., -s, -v)
			if arg[1] != '-' { // Not a long flag
				for j := 1; j < len(arg); j++ {
					switch arg[j] {
					case 's':
						silent = 1
					case 'v':
						verbose = 1
					case '#':
						log.Println("Debug flag detected, skipping remaining argument for DBUG_PUSH equivalent.")
						goto nextCliArg // Skip to next command-line argument
					case 'V': // Version flag
						// The original C code just prints and breaks.
						fmt.Printf("%s  Ver 1.4 for %s at %s\n", myProgname, "Go", "Unknown") // Placeholder for system info
						fallthrough                                                           // Fall through to 'I' or '?' for help text
					case 'I', '?':
						// The C code prints version info and then help text if -V is given or just help for -I/?
						if arg[j] == 'I' || arg[j] == '?' { // Only print full help if not already printed by -V
							fmt.Printf("%s  Ver 1.4 for %s at %s\n", myProgname, "Go", "Unknown") // Placeholder for system info
						}
						fmt.Println("This software comes with ABSOLUTELY NO WARRANTY. This is free software,\nand you are welcome to modify and redistribute it under the GPL license\n")
						fmt.Println("This program replaces strings in files or from stdin to stdout.\n" +
							"It accepts a list of from-string/to-string pairs and replaces\n" +
							"each occurrence of a from-string with the corresponding to-string.\n" +
							"The first occurrence of a found string is matched. If there is\n" +
							"more than one possibility for the string to replace, longer\n" +
							"matches are preferred before shorter matches.\n\n" +
							"A from-string can contain these special characters:\n" +
							"  \\^      Match start of line.\n" +
							"  \\$      Match end of line.\n" +
							"  \\b      Match space-character, start of line or end of line.\n" +
							"          For a end \\b the next replace starts locking at the end\n" +
							"          space-character. A \\b alone in a string matches only a\n" +
							"          space-character.\n")
						fmt.Printf("Usage: %s [-?svIV] from to from to ... -- [files]\n", myProgname)
						fmt.Println("or")
						fmt.Printf("Usage: %s [-?svIV] from to from to ... < fromfile > tofile\n", myProgname)
						fmt.Println("")
						fmt.Println("Options: -? or -I \"Info\"  -s \"silent\"      -v \"verbose\"")
						os.Exit(0) // Exit after printing help
					default:
						fmt.Fprintf(os.Stderr, "illegal option: -%c\n", arg[j])
						os.Exit(1)
					}
				}
				continue // Move to next `cliArgs` element after processing flag
			}
		}
		// If we are in explicit file mode or it's not a flag (and not '--'), add to appropriate list
		if parsingFlagsAndFromTo {
			fromToPairs = append(fromToPairs, arg)
		} else {
			finalFileNames = append(finalFileNames, arg)
		}
	nextCliArg:
	}

	if len(fromToPairs) == 0 && len(finalFileNames) == 0 && !help {
		myMessage(MYF_ME_BELL, "No replace options given")
		os.Exit(0)
	}

	// If no explicit files were given, but there are remaining args (which must be from/to pairs),
	// and the number of from/to pairs is odd, it's an error.
	if len(finalFileNames) == 0 && len(fromToPairs)%2 != 0 {
		myMessage(MYF_ME_BELL, "No to-string for last from-string")
		os.Exit(1)
	}

	var fromArray, toArray PointerArray
	for i := 0; i < len(fromToPairs); i += 2 {
		if err := fromArray.insertPointerName(fromToPairs[i]); err != nil {
			os.Exit(1)
		}
		if err := toArray.insertPointerName(fromToPairs[i+1]); err != nil {
			os.Exit(1)
		}
	}

	wordEndChars := make([]byte, 0, 256)
	for i := 1; i < 256; i++ {
		if myIsspace(nil, rune(i)) {
			wordEndChars = append(wordEndChars, byte(i))
		}
	}

	replaces, replaceStringStructs, err := initReplace(fromArray.Typelib.TypeNames, toArray.Typelib.TypeNames, fromArray.Typelib.Count, string(wordEndChars))
	if err != nil {
		log.Fatalf("Failed to initialize replace: %v", err)
	}

	fromArray.freePointerArray()
	toArray.freePointerArray()

	if err := initializeBuffer(); err != nil {
		os.Exit(1)
	}

	errorVal := 0
	if len(finalFileNames) == 0 {
		// No explicit filenames means process stdin to stdout
		errorVal = convertPipe(replace, replaceStringStructs, os.Stdin, os.Stdout)
	} else {
		// Process each specified file
		for _, fileName := range finalFileNames {
			errorVal = convertFile(replace, replaceStringStructs, fileName)
			if errorVal != 0 {
				break
			}
		}
	}

	freeBuffer()
	myEnd(verbose)
	if errorVal != 0 {
		os.Exit(2)
	}
	os.Exit(0)
}
