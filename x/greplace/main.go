package main

import (
	"bytes"
	"fmt"
	"io" // For io.ReadFull, io.EOF
	"log"
	"math" // For math.MaxUint32
	"os"
	"strings"
	// For checking space characters
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

// Placeholder for C's MYF flags (Moved to top level to ensure definition before use)
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
	outBuff   []byte // `static char *out_buff;`
	outLength uint   // `static uint out_length;` - Allocated size of `out_buff`.

	foundSets uint = 0 // `static uint found_sets=0;` - Count of unique found match results
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
	return int16(-foundSets - 1) // Return new packed index. C's `(-i-2)` becomes `-(foundSets-1)-2 = -foundSets-1` for the new element.
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

func initReplace(from []string, to []string, count uint, wordEndChars string) (*Replace, error) {
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
			return nil, fmt.Errorf("empty from-string at index %d", i)
		}
		states += currentLen + 1
		resultLen += uint(len(to[i])) + 1
		if currentLen > maxLength {
			maxLength = currentLen
		}
	}

	isWordEnd := [256]bool{}
	for _, char := range wordEndChars {
		if char < 256 {
			isWordEnd[byte(char)] = true
		}
	}

	var rss RepSets
	if err := rss.initSets(states); err != nil {
		return nil, err
	}
	defer rss.freeSets()

	foundSets = 0
	foundSet := make([]FoundSet, maxLength*count)

	_ = rss.makeNewSet()

	rss.makeSetsInvisible()

	tempRepSetForCopy := &RepSet{Bits: make([]uint, rss.SizeOfBits), SizeOfBits: rss.SizeOfBits}

	wordStates := rss.makeNewSet()
	if wordStates == nil {
		return nil, fmt.Errorf("failed to create wordStates")
	}
	startStates := rss.makeNewSet()
	if startStates == nil {
		return nil, fmt.Errorf("failed to create startStates")
	}

	follows := make([]Follows, states+2)

	currentNFAStateIdx := uint(1)
	for i := uint(0); i < count; i++ {
		if len(from[i]) >= 2 && from[i][0] == '\\' {
			if from[i][1] == '^' {
				startStates.internalSetBit(currentNFAStateIdx + 1)
				if len(from[i]) == 2 {
					if startStates.TableOffset == math.MaxUint32 {
						startStates.TableOffset = i
						startStates.FoundOffset = 1
					}
				}
			} else if from[i][1] == '$' {
				startStates.internalSetBit(currentNFAStateIdx)
				wordStates.internalSetBit(currentNFAStateIdx)
				if len(from[i]) == 2 && startStates.TableOffset == math.MaxUint32 {
					startStates.TableOffset = i
					startStates.FoundOffset = 0
				}
			} else {
				startStates.internalSetBit(currentNFAStateIdx + 1)
			}
		} else {
			startStates.internalSetBit(currentNFAStateIdx)
		}
		wordStates.internalSetBit(currentNFAStateIdx)

		currentStrLen := uint(0)
		for charIdx := 0; charIdx < len(from[i]); {
			chrCode := int(from[i][charIdx])
			if from[i][charIdx] == '\\' && charIdx+1 < len(from[i]) {
				charIdx++
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
					chrCode = int(from[i][charIdx])
				}
			}
			follows[currentNFAStateIdx].Chr = chrCode
			follows[currentNFAStateIdx].TableOffset = i
			currentStrLen++
			follows[currentNFAStateIdx].Len = currentStrLen
			currentNFAStateIdx++
			charIdx++
		}
		follows[currentNFAStateIdx].Chr = 0
		follows[currentNFAStateIdx].TableOffset = i
		follows[currentNFAStateIdx].Len = currentStrLen
		currentNFAStateIdx++
	}

	for setNr := uint(0); setNr < rss.Count; setNr++ {
		currentSet := &rss.Set[setNr]

		defaultState := int16(0)

		for i := uint(math.MaxUint32); ; {
			i = currentSet.getNextBit(i)
			if i == 0 {
				break
			}
			if follows[i].Chr == 0 {
				if defaultState == 0 {
					defaultState = findFound(foundSet, currentSet.TableOffset, currentSet.FoundOffset+1)
				}
			}
		}

		tempRepSetForCopy.copyBits(currentSet)

		if defaultState == 0 {
			tempRepSetForCopy.orBits(&rss.SetBuffer[0])
		}

		usedChars := [LastCharCode]bool{}
		for i := uint(math.MaxUint32); ; {
			i = tempRepSetForCopy.getNextBit(i)
			if i == 0 {
				break
			}
			usedChars[follows[i].Chr] = true
			if (follows[i].Chr == SpaceChar && follows[i].Len > 1 &&
				(i+1 >= uint(len(follows)) || follows[i+1].Chr == 0)) ||
				follows[i].Chr == EndOfLine {
				usedChars[0] = true
			}
		}

		if usedChars[SpaceChar] {
			for charCode := 0; charCode < 256; charCode++ {
				if isWordEnd[byte(charCode)] {
					usedChars[charCode] = true
				}
			}
		}

		for chr := 0; chr < 256; chr++ {
			if !usedChars[chr] {
				currentSet.Next[chr] = defaultState
			} else {
				newSet := rss.makeNewSet()
				if newSet == nil {
					log.Printf("ERROR: makeNewSet returned nil during DFA construction for char %d\n", chr)
					return nil, fmt.Errorf("failed to make new set for character %d", chr)
				}

				currentSet = &rss.Set[setNr]

				newSet.TableOffset = currentSet.TableOffset
				newSet.FoundLen = currentSet.FoundLen
				newSet.FoundOffset = currentSet.FoundOffset + 1

				foundEnd := uint(0)

				for i := uint(math.MaxUint32); ; {
					i = tempRepSetForCopy.getNextBit(i)
					if i == 0 {
						break
					}

					canTransition := false
					if follows[i].Chr == 0 {
						canTransition = true
					} else if follows[i].Chr == chr {
						canTransition = true
					} else if follows[i].Chr == SpaceChar && (isWordEnd[byte(chr)] ||
						(chr == 0 && follows[i].Len > 1 && (i+1 >= uint(len(follows)) || follows[i+1].Chr == 0))) {
						canTransition = true
					} else if follows[i].Chr == EndOfLine && chr == 0 {
						canTransition = true
					} else if follows[i].Chr == StartOfLine && chr == 0 {
						canTransition = true
					}

					if canTransition {
						if (chr == 0 || (follows[i].Chr != 0 && (i+1 >= uint(len(follows)) || follows[i+1].Chr == 0))) &&
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

						bitNr := i
						if (follows[i].Chr == SpaceChar || follows[i].Chr == EndOfLine) && chr == 0 {
							bitNr = i + 1
						}

						if follows[bitNr-1].Len < foundEnd || (newSet.FoundLen != 0 && (chr == 0 || follows[bitNr].Chr != 0)) {
							newSet.internalClearBit(i)
						} else {
							if chr == 0 || follows[bitNr].Chr == 0 {
								newSet.TableOffset = follows[bitNr].TableOffset
								if chr != 0 || (follows[i].Chr == SpaceChar ||
									follows[i].Chr == EndOfLine) {
									newSet.FoundOffset = int(foundEnd)
								}
								newSet.FoundLen = foundEnd
							}
							bitsSetCount++
						}
					}

					if bitsSetCount == 1 {
						currentSet.Next[chr] = findFound(foundSet,
							newSet.TableOffset,
							newSet.FoundOffset)
						rss.freeLastSet()
					} else {
						currentSet.Next[chr] = findSet(&rss, newSet)
					}
				} else {
					currentSet.Next[chr] = findSet(&rss, newSet)
				}
			}
		}
	}

	totalReplaceStates := rss.Count
	totalReplaceStrings := foundSets + 1

	replaces := make([]Replace, totalReplaceStates)
	replaceStrings := make([]ReplaceString, totalReplaceStrings)

	replaceStrings[0].Found = 1
	replaceStrings[0].ReplaceString = ""

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
	}

	for i := uint(0); i < totalReplaceStates; i++ {
		for j := 0; j < 256; j++ {
			cNext := rss.Set[i].Next[j]
			if cNext >= 0 {
				replaces[i].Next[j] = &replaces[cNext]
			} else {
				rsIndex := -cNext - 2
				if rsIndex < 0 || uint(rsIndex) >= totalReplaceStrings {
					return nil, fmt.Errorf("invalid ReplaceString index calculated: %d", rsIndex)
				}
				replaces[i].Next[j] = &replaceStrings[rsIndex]
			}
		}
	}

	log.Printf("Replace table has %d states", rss.Count)
	return &replaces[0], nil
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
	if outBuff == nil { // In Go, make() typically panics on OOM, so nil check is less common but good practice
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
	// DBUG_ENTER("fill_buffer_retaining"); // Go equivalent: log debug

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
	// `io.ReadFull` attempts to read exactly `bufRead` bytes. `reader.Read` reads up to `bufRead` bytes.
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
		// C: `my_eof = i = 1; buffer[bufbytes] = '\n';`
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
// Returns the actual length of the data written to `outBuff`, or -1 on error. (Using math.MaxUint32 for -1)
func replaceStrings(rep *Replace, out *[]byte, maxLength *uint, from []byte) uint {
	// CORRECTED: repPos must be an interface{} to allow type assertions to both *Replace and *ReplaceString.
	var repPos interface{}
	repPos = rep // Initialize with the starting Replace state (which is &replaces[0])

	currentOutPtr := uint(0)               // Logical length of content in `*out`
	outBufferEndIdx := int(*maxLength) - 1 // Index of last valid byte in `*out`

	for fromPtr := 0; ; { // `fromPtr` is the current read position in `from`
		// Inner loop: Advance DFA state until a match is found (`rep_pos.Found` is true)
		for {
			// Type assert repPos to *Replace to access its 'Found' field and 'Next' array
			currentReplaceState, ok := repPos.(*Replace)
			if !ok || currentReplaceState.Found { // If it's not *Replace (i.e., it's *ReplaceString) or if Found is true, break inner loop
				break
			}

			// Determine the character to process for DFA transition
			var charToProcess byte
			if fromPtr < len(from) {
				charToProcess = from[fromPtr]
			} else {
				charToProcess = 0 // End of input line, use null char for DFA
			}

			// Advance DFA state: `rep_pos = rep_pos->next[(uchar) *from];`
			repPos = currentReplaceState.Next[charToProcess]

			// If current position in output buffer exceeds its capacity, reallocate.
			if int(currentOutPtr) >= outBufferEndIdx {
				*maxLength *= 2 // Double the output buffer size
				newOut := make([]byte, *maxLength)
				copy(newOut, (*out)[:currentOutPtr]) // Copy already processed output
				*out = newOut
				outBufferEndIdx = int(*maxLength) - 1
			}

			// Copy character from input to output unless it's a null sentinel marking end of line
			if fromPtr < len(from) {
				(*out)[currentOutPtr] = from[fromPtr]
				currentOutPtr++
				fromPtr++
			} else {
				// If we've processed all input characters from `from`, but the DFA hasn't found a match
				// and is still asking for more input (i.e., `repPos.Found` is still false after processing `0`),
				// it implies the current line doesn't lead to a match and we are at its end.
				// This is the signal to exit the entire `replaceStrings` loop for this line.
				return currentOutPtr // No match found for the rest of the line, return current output length.
			}
		}

		// A match or end of line reached. `repPos` is either a `*ReplaceString` (found match)
		// or, if `repPos.Found` became true for a `*Replace` struct, it's an error.
		// The DFA construction should ensure `Found == true` only for `*ReplaceString` or `rep` (base).

		repString, isReplaceString := repPos.(*ReplaceString)

		// C: `if (!(rep_str = ((REPLACE_STRING*) rep_pos))->replace_string)`
		if !isReplaceString || repString.ReplaceString == "" { // Check if it's the sentinel `ReplaceString` (empty string)
			return currentOutPtr // This is the length of processed part of the output buffer.
		}

		updated = 1 // Some char is replaced (C's updated=1)

		// C: `to-=rep_str->to_offset;`
		currentOutPtr -= repString.ToOffset // Adjust output pointer backward

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
			}
			(*out)[currentOutPtr] = char // Copy character
			currentOutPtr++
		}

		// Adjust input pointer for the next scan.
		// C: `if (!*(from-=rep_str->from_offset) && rep_pos->found != 2)`
		fromPtr -= repString.FromOffset // `from` pointer adjustment
		if fromPtr < 0 {                // Ensure fromPtr doesn't go negative
			fromPtr = 0
		}

		// C's `!*(from-=rep_str->from_offset)` means the character at `from` (after adjustment) is null (end of line).
		// If `fromPtr` reaches the end of the input `from` slice (current line content) AND
		// the `repString.Found` flag is not 2 (which indicates `\^` only match that needs continuation).
		if fromPtr >= len(from) && repString.Found != 2 {
			return currentOutPtr // Return actual length of processed output.
		}

		// Reset DFA state for next scan
		// C: `rep_pos=rep;` (rep is the base address of the DFA states, points to rep[0])
		repPos = rep // Reset to the initial state (which is &replaces[0])
	}
}

// --- File I/O and Conversion ---

// convertPipe translates C's `convert_pipe`.
// Processes input from a reader (stdin) to a writer (stdout).
func convertPipe(rep *Replace, in io.Reader, out io.Writer) int {
	// DBUG_ENTER("convert_pipe"); // Go equivalent: log debug

	updated = 0 // Reset global updated flag
	retain := 0
	resetBuffer() // Reset global buffer state

	for {
		// C: `while ((error=fill_buffer_retaining(my_fileno(in),retain)) > 0)`
		// In Go, fillBufferRetaining returns bytes read or -1 for error.
		bytesRead := fillBufferRetaining(in, retain)
		if bytesRead < 0 { // Error
			return 1
		}
		if bytesRead == 0 && myEOF != 0 && bufBytes == 0 { // No more data and actual EOF, and buffer is empty
			break // Exit loop
		}
		if bufBytes == 0 && bytesRead == 0 && myEOF == 0 { // No data read, not EOF yet (can happen with empty input)
			break // Could be stalled, exit.
		}

		// C: `buffer[bufbytes]=0;` (Sentinel for C strings)
		// Go strings/slices don't rely on null terminators.
		// But the DFA logic uses `0` as end-of-string character.
		// Ensure a conceptual null terminator at `buffer[bufBytes]` for DFA logic.
		// Make sure `buffer` has capacity for `bufBytes+1`. `bufAlloc+1` should cover it.
		if bufBytes < len(buffer) {
			buffer[bufBytes] = 0
		}

		endOfLinePtr := 0 // Index into `buffer` for current line processing
		for {
			startOfLinePtr := endOfLinePtr
			// Find end of line (newline or null terminator from sentinel)
			// C: `while (end_of_line[0] != '\n' && end_of_line[0])`
			for endOfLinePtr < bufBytes && buffer[endOfLinePtr] != '\n' && buffer[endOfLinePtr] != 0 {
				endOfLinePtr++
			}

			if endOfLinePtr == bufBytes { // Reached end of currently buffered data
				// C: `retain= (int) (end_of_line - start_of_line); break;`
				retain = bufBytes - startOfLinePtr // Amount to retain for next read
				break                              // Break inner loop, go read more data
			}

			// C: `save_char=end_of_line[0]; end_of_line[0]=0; end_of_line++;`
			saveChar := buffer[endOfLinePtr] // Save the newline or null char
			// `buffer[endOfLinePtr] = 0` is conceptually done when we pass a subslice
			// (or handle it as a sentinel for DFA).
			// The `replaceStrings` function receives the slice up to this point.
			endOfLinePtr++ // Move past the newline/null for the next line start

			// Pass the line content to `replaceStrings`.
			// The `replaceStrings` function expects `from` as a byte slice.
			// The `from` slice should be just the content, not including the newline/null.
			lineContent := buffer[startOfLinePtr : endOfLinePtr-1]
			length := replaceStrings(rep, &outBuff, &outLength, lineContent)
			if length == math.MaxUint32 { // Error indicated by `math.MaxUint32` (-1 in C)
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
				}
				outBuff[length] = saveChar // Add back the original line terminator
				length++
			}

			// C: `if (my_fwrite(out, (uchar*) out_buff, length, MYF(MY_WME | MY_NABP)))`
			_, err := out.Write(outBuff[:length])
			if err != nil {
				log.Printf("Error writing to output: %v", err)
				return 1
			}
		}
	}
	return 0 // Success
}

// convertFile translates C's `convert_file`.
// Opens a file, performs replacement, and writes to a temporary file, then renames.
func convertFile(rep *Replace, name string) int {
	// DBUG_ENTER("convert_file"); // Go equivalent: log debug

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

	out = tempFile // os.CreateTemp returns *os.File, which can be used directly as a writer.

	errorVal := convertPipe(rep, in, out) // Perform replacement
	out.Close()                           // Explicitly close output before rename/delete

	if updated != 0 && errorVal == 0 { // C: `if (updated && ! error)`
		// C: `my_redel(org_name,tempname,MYF(MY_WME | MY_LINK_WARNING));`
		// Atomically replace the original file with the temporary one.
		err = os.Rename(tempname, orgName)
		if err != nil {
			myMessage(MYF_MY_WME|MYF_MY_LINK_WARNING, "Failed to rename temporary file to %s: %v", orgName, err)
			return 1
		}
	} else {
		// C: `my_delete(tempname,MYF(MY_WME));`
		os.Remove(tempname) // Delete temporary file if not updated or if there was an error
	}

	if silent == 0 && errorVal == 0 { // C: `if (!silent && ! error)`
		if updated != 0 {
			fmt.Printf("%s converted\n", name)
		} else if verbose != 0 {
			fmt.Printf("%s left unchanged\n", name)
		}
	}
	return errorVal // Return 0 for success, 1 for error
}

// --- Main function and general utilities ---

// myMessage is a placeholder for C's my_message function.
func myMessage(flags int, msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", fmt.Sprintf(msg, args...))
}

// Dummy for `strcmp` from C's `string.h` for use in `getReplaceStrings`
func myStrcmp(s1, s2 string) int {
	return strings.Compare(s1, s2)
}

// Dummy for `my_isspace` from `m_ctype.h` for use in `main`.
func myIsspace(charset interface{}, r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f'
}

// `my_disable_symlinks` from C's `my_sys.h`
var myDisableSymlinks = false

// `my_progname` from C's `my_global.h`
var myProgname = "replace_strings" // Default, will be set in main

// staticGetOptions translates C's `static_get_options`.
func staticGetOptions(argc *int, argv *[]string) error {
	help := 0
	version := 0

	args := *argv // Get the current slice of arguments

	myProgname = args[0]
	args = args[1:]
	*argc--

	i := 0
	for i < len(args) && len(args[i]) > 1 && args[i][0] == '-' && args[i][1] != '-' {
		pos := args[i]
		for j := 1; j < len(pos); j++ {
			char := pos[j]
			switch char {
			case 's':
				silent = 1
			case 'v':
				verbose = 1
			case '#':
				j = len(pos)
			case 'V':
				version = 1
				fallthrough
			case 'I', '?':
				help = 1
				fmt.Printf("%s Ver 1.4 for %s at %s\n", myProgname, "GO_OS", "GO_ARCH")
				if version == 1 {
					break
				}
				fmt.Println("This software comes with ABSOLUTELY NO WARRANTY. This is free software,\nand you are welcome to modify and redistribute it under the GPL license\n")
				fmt.Println("This program replaces strings in files or from stdin to stdout.\n" +
					"It accepts a list of from-string/to-string pairs and replaces\n" +
					"each occurrence of a from-string with the corresponding to-string.\n" +
					"The first occurrence of a found string is matched. If there is\n" +
					"more than one possibility for the string to replace, longer\n" +
					"matches are preferred before shorter matches.\n\n" +
					"A from-string can contain these special characters:\n" +
					"  \\^       Match start of line.\n" +
					"  \\$       Match end of line.\n" +
					"  \\b       Match space-character, start of line or end of line.\n" +
					"           For a end \\b the next replace starts locking at the end\n" +
					"           space-character. A \\b alone in a string matches only a\n" +
					"           space-character.\n")
				fmt.Printf("Usage: %s [-?svIV] from to from to ... -- [files]\n", myProgname)
				fmt.Println("or")
				fmt.Printf("Usage: %s [-?svIV] from to from to ... < fromfile > tofile\n", myProgname)
				fmt.Println("\nOptions: -? or -I \"Info\"  -s \"silent\"     -v \"verbose\"")
				os.Exit(0)
			default:
				fmt.Fprintf(os.Stderr, "illegal option: -%c\n", char)
				return fmt.Errorf("illegal option: -%c", char)
			}
		}
		i++
	}
	*argv = args[i:]
	*argc = len(*argv)

	if *argc == 0 {
		if help == 0 {
			myMessage(0, "No replace options given", MYF_ME_BELL)
		}
		os.Exit(0)
	}
	return nil
}

// getReplaceStrings translates C's `get_replace_strings`.
func getReplaceStrings(argc *int, argv *[]string, fromArray, toArray *PointerArray) error {
	args := *argv

	i := 0
	for i < len(args) {
		pos := args[i]
		if len(pos) >= 2 && pos[0] == '-' && pos[1] == '-' {
			if len(pos) == 2 {
				break
			}
		}

		err := fromArray.insertPointerName(pos)
		if err != nil {
			return err
		}
		i++
		*argc--

		if i >= len(args) || (len(args[i]) == 2 && args[i][0] == '-' && args[i][1] == '-') {
			myMessage(0, "No to-string for last from-string", MYF_ME_BELL)
			return fmt.Errorf("missing to-string for last from-string")
		}

		err = toArray.insertPointerName(args[i])
		if err != nil {
			return err
		}
		i++
		*argc--
	}

	if i < len(args) && len(args[i]) == 2 && args[i][0] == '-' && args[i][1] == '-' {
		i++
		*argc--
	}

	*argv = args[i:]
	return nil
}

// --- Main Program ---
func main() {
	myInit(os.Args[0])

	args := os.Args
	argc := len(args)

	if err := staticGetOptions(&argc, &args); err != nil {
		os.Exit(1)
	}

	var fromArray, toArray PointerArray
	if err := getReplaceStrings(&argc, &args, &fromArray, &toArray); err != nil {
		os.Exit(1)
	}

	var wordEndCharsBuffer bytes.Buffer
	for i := 1; i < 256; i++ {
		if myIsspace(nil, rune(i)) {
			wordEndCharsBuffer.WriteByte(byte(i))
		}
	}
	wordEndChars := wordEndCharsBuffer.String()

	replace, err := initReplace(fromArray.Typelib.TypeNames, toArray.Typelib.TypeNames, fromArray.Typelib.Count, wordEndChars)
	if err != nil {
		log.Fatalf("Error initializing replacement: %v", err)
		os.Exit(1)
	}

	fromArray.freePointerArray()
	toArray.freePointerArray()

	if err := initializeBuffer(); err != nil {
		log.Fatalf("Error initializing buffers: %v", err)
		os.Exit(1)
	}
	defer freeBuffer()

	errorResult := 0

	if argc == 0 {
		errorResult = convertPipe(replace, os.Stdin, os.Stdout)
	} else {
		for _, fileName := range args {
			if err := convertFile(replace, fileName); err != 0 {
				errorResult = err
				break
			}
		}
	}

	flags := MY_CHECK_ERROR
	if verbose != 0 {
		flags |= MY_GIVE_INFO
	}
	myEnd(flags)

	if errorResult != 0 {
		os.Exit(2)
	} else {
		os.Exit(0)
	}
}
