package main

// BitSet represents a set of states during DFA construction
type BitSet struct {
	Bits   []uint64
	Length int
}

// NewBitSet creates a new bit set that can hold 'size' bits
func NewBitSet(size int) *BitSet {
	return &BitSet{
		Bits:   make([]uint64, (size+63)/64),
		Length: size,
	}
}

// Set sets a bit at the given position
func (bs *BitSet) Set(pos int) {
	if pos >= 0 && pos < bs.Length {
		bs.Bits[pos/64] |= 1 << (pos % 64)
	}
}

// Clear clears a bit at the given position
func (bs *BitSet) Clear(pos int) {
	if pos >= 0 && pos < bs.Length {
		bs.Bits[pos/64] &^= 1 << (pos % 64)
	}
}

// IsSet returns true if the bit at position is set
func (bs *BitSet) IsSet(pos int) bool {
	if pos < 0 || pos >= bs.Length {
		return false
	}
	return (bs.Bits[pos/64] & (1 << (pos % 64))) != 0
}

// Or performs bitwise OR with another BitSet
func (bs *BitSet) Or(other *BitSet) {
	minLen := len(bs.Bits)
	if len(other.Bits) < minLen {
		minLen = len(other.Bits)
	}
	for i := 0; i < minLen; i++ {
		bs.Bits[i] |= other.Bits[i]
	}
}

// Copy copies another BitSet into this one
func (bs *BitSet) Copy(other *BitSet) {
	if len(bs.Bits) < len(other.Bits) {
		bs.Bits = make([]uint64, len(other.Bits))
	}
	copy(bs.Bits, other.Bits)
	bs.Length = other.Length
}

// NextBit finds the next set bit after lastPos (inclusive search starting from lastPos+1)
func (bs *BitSet) NextBit(lastPos int) int {
	pos := lastPos + 1
	if pos >= bs.Length {
		return -1
	}

	wordIndex := pos / 64
	bitIndex := pos % 64

	if wordIndex >= len(bs.Bits) {
		return -1
	}

	// Mask off bits we don't want in the current word
	word := bs.Bits[wordIndex] & (^uint64(0) << bitIndex)

	for {
		if word != 0 {
			// Find the first set bit in this word
			bitPos := wordIndex * 64
			for word&1 == 0 {
				word >>= 1
				bitPos++
			}
			if bitPos < bs.Length {
				return bitPos
			}
		}

		wordIndex++
		if wordIndex >= len(bs.Bits) {
			break
		}
		word = bs.Bits[wordIndex]
	}

	return -1
}

// Equal checks if two BitSets are equal
func (bs *BitSet) Equal(other *BitSet) bool {
	if bs.Length != other.Length {
		return false
	}

	words := (bs.Length + 63) / 64
	for i := 0; i < words; i++ {
		if i < len(bs.Bits) && i < len(other.Bits) {
			if bs.Bits[i] != other.Bits[i] {
				return false
			}
		} else if i < len(bs.Bits) && bs.Bits[i] != 0 {
			return false
		} else if i < len(other.Bits) && other.Bits[i] != 0 {
			return false
		}
	}

	return true
}
