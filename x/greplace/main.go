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
	// See if we need to grow the buffer.
	if int(bufAlloc)-n <= bufRead {
		for int(bufAlloc)-n <= bufRead {
			bufAlloc *= 2
			bufRead *= 2
		}
		newBuffer := make([]byte, bufAlloc+1)
		copy(newBuffer, buffer[:bufBytes])
		buffer = newBuffer
		if buffer == nil {
			return -1
		}
	}

	// Shift stuff down.
	if n > 0 && bufBytes >= n {
		copy(buffer[0:n], buffer[bufBytes-n:bufBytes])
	}
	bufBytes = n

	if myEOF != 0 {
		return 0
	}

	// Read in new stuff.
	nRead, err := reader.Read(buffer[bufBytes : bufBytes+bufRead])
	if err != nil && err != io.EOF {
		log.Printf("Error reading from input: %v", err)
		return -1
	}

	// Kludge to pretend every nonempty file ends with a newline.
	if nRead == 0 && bufBytes > 0 && buffer[bufBytes-1] != '\n' {
		myEOF = 1
		buffer[bufBytes] = '\n'
		nRead = 1
	} else if err == io.EOF {
		myEOF = 1
	}

	bufBytes += nRead
	return nRead
}

// replaceStrings performs string replacements using the DFA.
func replaceStrings(allReplaces []Replace, allReplaceStrings []ReplaceString, out *[]byte, maxLength *uint, from []byte) uint {
	log.Printf("replaceStrings: Processing line (len %d): '%s'", len(from), string(from))

	var repPos interface{}
	if len(allReplaces) > 1 {
		repPos = &allReplaces[1] // Start at the state corresponding to start_states
	} else {
		log.Printf("replaceStrings: Warning: Not enough replace states (%d) for rep+1. Starting from allReplaces[0].", len(allReplaces))
		repPos = &allReplaces[0]
	}

	currentOutPtr := uint(0)
	outBufferEndIdx := int(*maxLength) - 1

	for fromPtr := 0; ; {
		log.Printf("replaceStrings: Loop start: fromPtr=%d, currentOutPtr=%d, repPosType=%T", fromPtr, currentOutPtr, repPos)

		// Inner loop: Advance DFA state until a match is found
		for {
			currentReplaceState, ok := repPos.(*Replace)
			if !ok {
				log.Printf("replaceStrings: Match found! repPos is *ReplaceString. Breaking inner loop.")
				break
			}
			if currentReplaceState.Found {
				log.Printf("replaceStrings: Unexpected: *Replace state has Found=true. Breaking.")
				break
			}

			var charToProcess byte
			if fromPtr < len(from) {
				charToProcess = from[fromPtr]
				log.Printf("replaceStrings: Processing char '%c' (byte %d) at fromPtr=%d", charToProcess, charToProcess, fromPtr)
			} else {
				charToProcess = 0
				log.Printf("replaceStrings: End of line, processing null char (byte %d) at fromPtr=%d", charToProcess, fromPtr)
			}

			nextState := currentReplaceState.Next[charToProcess]
			log.Printf("replaceStrings: Transitioning from %T to %T for char %d", repPos, nextState, charToProcess)
			repPos = nextState

			if int(currentOutPtr) >= outBufferEndIdx {
				*maxLength *= 2
				newOut := make([]byte, *maxLength)
				copy(newOut, (*out)[:currentOutPtr])
				*out = newOut
				outBufferEndIdx = int(*maxLength) - 1
				log.Printf("replaceStrings: Output buffer reallocated to %d bytes.", *maxLength)
			}

			if fromPtr < len(from) {
				(*out)[currentOutPtr] = charToProcess
				currentOutPtr++
				fromPtr++
				log.Printf("replaceStrings: Copied char. currentOutPtr=%d, fromPtr=%d", currentOutPtr, fromPtr)
			} else {
				log.Printf("replaceStrings: End of input line reached without match. Returning current output length %d.", currentOutPtr)
				return currentOutPtr
			}
		}

		repString, isReplaceString := repPos.(*ReplaceString)
		if !isReplaceString {
			log.Printf("replaceStrings: Critical Error: repPos is not *ReplaceString after breaking inner loop. Type: %T", repPos)
			return math.MaxUint32
		}

		if repString.ReplaceString == "" {
			log.Printf("replaceStrings: Sentinel ReplaceString encountered (empty replace_string). Returning current output length %d.", currentOutPtr)
			return currentOutPtr
		}

		updated = 1
		log.Printf("replaceStrings: Replacement detected! Original fromPtr: %d, currentOutPtr: %d", fromPtr, currentOutPtr)
		log.Printf("replaceStrings: ReplaceString: '%s', ToOffset: %d, FromOffset: %d", repString.ReplaceString, repString.ToOffset, repString.FromOffset)

		currentOutPtr -= repString.ToOffset
		log.Printf("replaceStrings: Adjusted currentOutPtr back by %d to %d", repString.ToOffset, currentOutPtr)

		for _, charRune := range repString.ReplaceString {
			char := byte(charRune)
			if int(currentOutPtr) >= outBufferEndIdx {
				*maxLength *= 2
				newOut := make([]byte, *maxLength)
				copy(newOut, (*out)[:currentOutPtr])
				*out = newOut
				outBufferEndIdx = int(*maxLength) - 1
				log.Printf("replaceStrings: Output buffer reallocated during replacement to %d bytes.", *maxLength)
			}
			(*out)[currentOutPtr] = char
			currentOutPtr++
		}
		log.Printf("replaceStrings: Copied replacement string. New currentOutPtr=%d", currentOutPtr)

		fromPtr -= repString.FromOffset
		if fromPtr < 0 {
			fromPtr = 0
		}
		log.Printf("replaceStrings: Adjusted fromPtr back by %d to %d", repString.FromOffset, fromPtr)

		if fromPtr >= len(from) && repString.Found != 2 {
			log.Printf("replaceStrings: End of input reached after replacement. Returning current output length %d.", currentOutPtr)
			return currentOutPtr
		}

		repPos = &allReplaces[0]
		log.Printf("replaceStrings: Resetting repPos to initial state for next scan.")
	}
}

// convertPipe translates C's `convert_pipe`.
// Processes input from a reader (stdin) to a writer (stdout).
func convertPipe(replaces []Replace, replaceStringStructs []ReplaceString, in io.Reader, out io.Writer) int {
	log.Printf("convertPipe: Starting pipe conversion.")
	updated = 0
	retain := 0
	resetBuffer()

	for {
		log.Printf("convertPipe: Calling fillBufferRetaining with retain=%d", retain)
		bytesRead := fillBufferRetaining(in, retain)
		log.Printf("convertPipe: fillBufferRetaining returned %d bytes. bufBytes=%d, myEOF=%d", bytesRead, bufBytes, myEOF)

		if bytesRead < 0 {
			log.Printf("convertPipe: Error from fillBufferRetaining, returning 1.")
			return 1
		}
		if bytesRead == 0 && myEOF != 0 && bufBytes == 0 {
			log.Printf("convertPipe: End of file and empty buffer. Breaking loop.")
			break
		}
		if bufBytes == 0 && bytesRead == 0 && myEOF == 0 {
			log.Printf("convertPipe: No data read, not EOF. Could be stalled or empty input. Breaking.")
			break
		}

		if bufBytes < len(buffer) {
			buffer[bufBytes] = 0
			log.Printf("convertPipe: Added null sentinel at buffer[%d]", bufBytes)
		}

		endOfLinePtr := 0
		for {
			startOfLinePtr := endOfLinePtr
			log.Printf("convertPipe: Scanning for end of line from index %d (bufBytes=%d)", endOfLinePtr, bufBytes)
			for endOfLinePtr < bufBytes && buffer[endOfLinePtr] != '\n' && buffer[endOfLinePtr] != 0 {
				endOfLinePtr++
			}
			log.Printf("convertPipe: End of line found at index %d", endOfLinePtr)

			if endOfLinePtr == bufBytes {
				retain = bufBytes - startOfLinePtr
				log.Printf("convertPipe: End of buffered data. Retaining %d bytes. Breaking inner loop.", retain)
				break
			}

			saveChar := buffer[endOfLinePtr]
			log.Printf("convertPipe: Saved char '%c' (byte %d) at endOfLinePtr=%d", saveChar, saveChar, endOfLinePtr)
			endOfLinePtr++

			lineContent := buffer[startOfLinePtr : endOfLinePtr-1]
			log.Printf("convertPipe: Calling replaceStrings for line: '%s'", string(lineContent))
			length := replaceStrings(replaces, replaceStringStructs, &outBuff, &outLength, lineContent)
			log.Printf("convertPipe: replaceStrings returned length %d", length)

			if length == math.MaxUint32 {
				log.Printf("convertPipe: Error from replaceStrings, returning 1.")
				return 1
			}

			if myEOF == 0 || (myEOF == 1 && saveChar == '\n') {
				if uint(len(outBuff)) <= length {
					*&outLength *= 2
					newOutBuff := make([]byte, *&outLength)
					copy(newOutBuff, outBuff[:length])
					outBuff = newOutBuff
					log.Printf("convertPipe: Output buffer reallocated for newline to %d bytes.", *&outLength)
				}
				outBuff[length] = saveChar
				length++
				log.Printf("convertPipe: Appended saved char. New length=%d", length)
			}

			log.Printf("convertPipe: Writing %d bytes to output: '%s'", length, string(outBuff[:length]))
			_, err := out.Write(outBuff[:length])
			if err != nil {
				log.Printf("Error writing to output: %v", err)
				return 1
			}
		}
	}
	log.Printf("convertPipe: Pipe conversion finished successfully.")
	return 0
}

// myMessage is a placeholder for C's my_message function.
func myMessage(flags int, msg string, args ...interface{}) {
	if strings.ContainsRune(msg, '%') {
		fmt.Fprintf(os.Stderr, "Error: %s\n", fmt.Sprintf(msg, args...))
	} else if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "Error: %s (flags: %v)\n", msg, flags)
	} else {
		fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	}
}

// myIsspace dummy for space character checking
func myIsspace(charset interface{}, r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' ||
		r == '\r' || r == '\v' || r == '\f'
}

func main() {
	// Simple argument parsing: pairs of from/to strings
	args := os.Args[1:]

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: %s from1 to1 [from2 to2 ...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Reads from stdin and writes to stdout\n")
		os.Exit(1)
	}

	if len(args)%2 != 0 {
		fmt.Fprintf(os.Stderr, "Error: Arguments must be pairs of from/to strings\n")
		os.Exit(1)
	}

	// Build from and to arrays
	var fromArray, toArray PointerArray
	for i := 0; i < len(args); i += 2 {
		if err := fromArray.insertPointerName(args[i]); err != nil {
			fmt.Fprintf(os.Stderr, "Error adding from string: %v\n", err)
			os.Exit(1)
		}
		if err := toArray.insertPointerName(args[i+1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error adding to string: %v\n", err)
			os.Exit(1)
		}
	}

	// Build word end characters (spaces)
	wordEndChars := make([]byte, 0, 256)
	for i := 1; i < 256; i++ {
		if myIsspace(nil, rune(i)) {
			wordEndChars = append(wordEndChars, byte(i))
		}
	}

	// Initialize the replacement DFA
	replaces, replaceStringStructs, err := initReplace(
		fromArray.Typelib.TypeNames,
		toArray.Typelib.TypeNames,
		fromArray.Typelib.Count,
		string(wordEndChars))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize replacement: %v\n", err)
		os.Exit(1)
	}

	// Clean up arrays
	fromArray.freePointerArray()
	toArray.freePointerArray()

	// Initialize buffers
	if err := initializeBuffer(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize buffer: %v\n", err)
		os.Exit(1)
	}

	// Process stdin to stdout
	errorVal := convertPipe(replaces, replaceStringStructs, os.Stdin, os.Stdout)

	// Clean up
	freeBuffer()

	if errorVal != 0 {
		os.Exit(2)
	}
}
