package main

import (
	"fmt"
	"log"
	"os"
	"strings"
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
	Next  [256]interface{}
}

type ReplaceString struct {
	Found         bool
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
	updated int = 0

	buffer   []byte
	bufBytes int
	bufRead  int
	myEOF    int
	bufAlloc uint

	outBuff   []byte
	outLength uint

	foundSets uint = 0
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

// initSets translates C's `init_sets`.
// Initializes the RepSets structure by allocating initial buffers for RepSet states and their bitsets.
func (rss *RepSets) initSets(states uint) error {
	// `bzero((char*) sets,sizeof(*sets));`
	// In Go, struct fields are zero-valued by default upon declaration, so this is handled.

	rss.SizeOfBits = (states + 7) / 8 // Size of `Bits` array in `uint`s, assuming `uint` is 8 bytes in C (which is `sizeof(uint)`)
	// C: ((states+7)/8) assumes 1 byte per bit in calculation, but then uses sizeof(uint)
	// If uint is 4 bytes, then `SizeOfBits` is (states + 31) / 32, which is `(states + WordBit - 1) / WordBit`.
	// The C code actually uses `sizeof(uint) * sets->size_of_bits` for allocation,
	// implying `SizeOfBits` is calculated as `(states + 8*sizeof(uint) - 1) / (8*sizeof(uint))`.
	// Let's use `(states + WordBit - 1) / WordBit` for Go to be precise.
	rss.SizeOfBits = (states + WordBit - 1) / WordBit

	// Allocate initial RepSetBuffer (array of RepSet structs)
	// `my_malloc(sizeof(REP_SET)*SET_MALLOC_HUNC, MYF(MY_WME))`
	rss.SetBuffer = make([]RepSet, SetMallocHunc)
	if rss.SetBuffer == nil {
		return fmt.Errorf("failed to allocate SetBuffer")
	}

	// Allocate initial BitBuffer (raw uints for all bitsets)
	// `my_malloc(sizeof(uint)*sets->size_of_bits*SET_MALLOC_HUNC, MYF(MY_WME))`
	rss.BitBuffer = make([]uint, rss.SizeOfBits*SetMallocHunc)
	if rss.BitBuffer == nil {
		// In C, you'd free SetBuffer here. In Go, it's eligible for GC.
		return fmt.Errorf("failed to allocate BitBuffer")
	}

	// Initialize pointers within SetBuffer to point to parts of BitBuffer.
	// This mirrors `sets->set_buffer[i].bits=bit_buffer; bit_buffer+=sets->size_of_bits;`
	// in `make_new_set` logic, but done proactively here for the initial chunk.
	// This setup ensures `SetBuffer[i].Bits` is a slice pointing to a unique segment of `BitBuffer`.
	for i := 0; i < SetMallocHunc; i++ {
		startIdx := i * int(rss.SizeOfBits)
		endIdx := startIdx + int(rss.SizeOfBits)
		// Each RepSet.Bits gets a sub-slice from the shared BitBuffer
		rss.SetBuffer[i].Bits = rss.BitBuffer[startIdx:endIdx]
		rss.SetBuffer[i].SizeOfBits = rss.SizeOfBits // Important for bit ops
	}

	// Initialize `Set` to point to the active part of `SetBuffer`.
	// Initially, `Set` is the same as `SetBuffer` because `invisible` is 0.
	rss.Set = rss.SetBuffer // All allocated sets are initially "visible" and active.
	rss.Extra = SetMallocHunc
	rss.Count = 0 // No sets currently in use after init

	return nil
}

// makeNewSet translates C's `make_new_set`.
// Returns a pointer to a new, initialized RepSet, handling reallocation if necessary.
func (rss *RepSets) makeNewSet() *RepSet {
	// `if (sets->extra)`
	if rss.Extra > 0 {
		rss.Extra--
		// Get the next available set from the current `Set` slice
		set := &rss.Set[rss.Count] // Get a pointer to the next available element
		rss.Count++

		// bzero fields (Go's `make` and struct initialization handle this, but explicit zeroing
		// for slices or fields is good if re-using allocated memory).
		// For the Bits slice, make sure it's actually zeroed.
		for i := range set.Bits {
			set.Bits[i] = 0 // Clear all bits
		}
		// Zero the `Next` array
		for i := range set.Next {
			set.Next[i] = 0 // Or -1 if that's the default invalid state
		}
		set.FoundOffset = 0
		set.FoundLen = 0
		set.TableOffset = ^uint(0) // C's (uint) ~0 for max_uint

		// set.SizeOfBits should already be correct from `initSets` or previous allocation.
		return set
	}

	// `count=sets->count+sets->invisible+SET_MALLOC_HUNC;`
	newTotalSets := rss.Count + rss.Invisible + SetMallocHunc

	// Reallocate `SetBuffer` (the underlying array of RepSet structs)
	// `my_realloc((uchar*) sets->set_buffer, sizeof(REP_SET)*count, MYF(MY_WME))`
	newSetBuffer := make([]RepSet, newTotalSets)
	copy(newSetBuffer, rss.SetBuffer) // Copy existing sets
	rss.SetBuffer = newSetBuffer

	// Reallocate `BitBuffer` (the raw uints for all bitsets)
	// `my_realloc((uchar*) sets->bit_buffer, (sizeof(uint)*sets->size_of_bits)*count, MYF(MY_WME))`
	newBitBuffer := make([]uint, rss.SizeOfBits*newTotalSets)
	copy(newBitBuffer, rss.BitBuffer) // Copy existing bit data
	rss.BitBuffer = newBitBuffer

	// Re-assign `Bits` slices within the `SetBuffer` to point to the new `BitBuffer`
	// This loop is crucial because reallocating `BitBuffer` means the old slices
	// stored in `RepSet.Bits` are no longer valid.
	for i := 0; i < int(newTotalSets); i++ {
		startIdx := i * int(rss.SizeOfBits)
		endIdx := startIdx + int(rss.SizeOfBits)
		rss.SetBuffer[i].Bits = rss.BitBuffer[startIdx:endIdx]
		rss.SetBuffer[i].SizeOfBits = rss.SizeOfBits // Ensure this is set for all
	}

	// Update `Set` to point into the potentially new `SetBuffer`
	rss.Set = rss.SetBuffer[rss.Invisible:] // Update based on `Invisible` offset

	rss.Extra = SetMallocHunc
	return rss.makeNewSet() // Recursively call to get a fresh set from the newly allocated buffer
}

// makeSetsInvisible translates C's `make_sets_invisible`.
// Marks the currently used sets as "invisible" by adjusting the `Set` slice,
// allowing `makeNewSet` to start allocating from the beginning of the `SetBuffer` (relative to `Invisible`).
func (rss *RepSets) makeSetsInvisible() {
	rss.Invisible += rss.Count              // Add current count to invisible
	rss.Set = rss.SetBuffer[rss.Invisible:] // Shift the `Set` slice view
	rss.Count = 0                           // Reset count of visible sets
}

// freeLastSet translates C's `free_last_set`.
// Decrements the count of active sets, conceptually "freeing" the last one used.
// It doesn't deallocate memory but makes it available for reuse.
func (rss *RepSets) freeLastSet() {
	if rss.Count > 0 {
		rss.Count--
		rss.Extra++
	}
}

// freeSets translates C's `free_sets`.
// Releases the main allocated buffers for `RepSet` structs and their `Bits`.
func (rss *RepSets) freeSets() {
	// In Go, setting slices to nil makes their underlying arrays eligible for GC.
	rss.SetBuffer = nil
	rss.BitBuffer = nil
	// rss.Set doesn't need to be nilled explicitly as it's a subslice of SetBuffer.
	// Zero out counts for good measure.
	rss.Count = 0
	rss.Extra = 0
	rss.Invisible = 0
	rss.SizeOfBits = 0
}

// --- Other functions (from previous steps, placeholders) ---

func myMessage(flags int, msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", fmt.Sprintf(msg, args...))
}

func myStrcmp(s1, s2 string) int {
	return strings.Compare(s1, s2)
}

func myIsspace(charset interface{}, r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f'
}

func main() {
	// Example usage for testing RepSets management:
	// var rss RepSets
	// err := rss.initSets(100) // Initialize for 100 states
	// if err != nil {
	// 	log.Fatalf("Error initializing RepSets: %v", err)
	// }
	// defer rss.freeSets() // Ensure cleanup

	// fmt.Printf("Initial rss: Count=%d, Extra=%d, Invisible=%d, SizeOfBits=%d, SetBufferLen=%d, BitBufferLen=%d\n",
	// 	rss.Count, rss.Extra, rss.Invisible, rss.SizeOfBits, len(rss.SetBuffer), len(rss.BitBuffer))

	// s1 := rss.makeNewSet()
	// if s1 == nil {
	// 	log.Fatalf("Failed to make new set 1")
	// }
	// s1.internalSetBit(5)
	// fmt.Printf("After s1: Count=%d, Extra=%d, SetBufferLen=%d\n", rss.Count, rss.Extra, len(rss.SetBuffer))

	// s2 := rss.makeNewSet()
	// if s2 == nil {
	// 	log.Fatalf("Failed to make new set 2")
	// }
	// fmt.Printf("After s2: Count=%d, Extra=%d, SetBufferLen=%d\n", rss.Count, rss.Extra, len(rss.SetBuffer))

	// rss.makeSetsInvisible()
	// fmt.Printf("After makeSetsInvisible: Count=%d, Extra=%d, Invisible=%d, SetBufferLen=%d\n",
	// 	rss.Count, rss.Extra, rss.Invisible, len(rss.SetBuffer))

	// s3 := rss.makeNewSet() // Should allocate from invisible section now
	// if s3 == nil {
	// 	log.Fatalf("Failed to make new set 3")
	// }
	// fmt.Printf("After s3: Count=%d, Extra=%d, Invisible=%d, SetBufferLen=%d\n",
	// 	rss.Count, rss.Extra, rss.Invisible, len(rss.SetBuffer))

	// for i := 0; i < SetMallocHunc*2; i++ { // Force reallocation
	// 	_ = rss.makeNewSet()
	// }
	// fmt.Printf("After many new sets (potential reallocation): Count=%d, Extra=%d, SetBufferLen=%d, BitBufferLen=%d\n",
	// 	rss.Count, rss.Extra, len(rss.SetBuffer), len(rss.BitBuffer))

	// rss.freeSets()
	// fmt.Printf("After freeSets: SetBuffer=%v, BitBuffer=%v\n", rss.SetBuffer, rss.BitBuffer)
}

func staticGetOptions(args []string) ([]string, error) { return args, nil }
func getReplaceStrings(args []string, fromArray, toArray *PointerArray) ([]string, error) {
	return args, nil
}
func initReplace(from []string, to []string, count uint, wordEndChars []byte) (*Replace, error) {
	return nil, nil
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
