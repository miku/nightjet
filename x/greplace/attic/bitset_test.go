package main

import (
	"testing"
)

func TestNewBitSet(t *testing.T) {
	tests := []struct {
		size     int
		expected int // expected number of uint64 words
	}{
		{0, 0},
		{1, 1},
		{63, 1},
		{64, 1},
		{65, 2},
		{128, 2},
		{129, 3},
	}
	for _, tt := range tests {
		bs := NewBitSet(tt.size)
		if bs.Length != tt.size {
			t.Errorf("NewBitSet(%d): expected length %d, got %d", tt.size, tt.size, bs.Length)
		}
		if len(bs.Bits) != tt.expected {
			t.Errorf("NewBitSet(%d): expected %d words, got %d", tt.size, tt.expected, len(bs.Bits))
		}
	}
}

func TestBitSetBasicOperations(t *testing.T) {
	bs := NewBitSet(100)
	// Initially all bits should be clear
	for i := 0; i < 100; i++ {
		if bs.IsSet(i) {
			t.Errorf("bit %d should be clear initially", i)
		}
	}
	// Set some bits
	testBits := []int{0, 1, 7, 31, 32, 63, 64, 99}
	for _, bit := range testBits {
		bs.Set(bit)
		if !bs.IsSet(bit) {
			t.Errorf("bit %d should be set after Set()", bit)
		}
	}
	// Clear some bits
	clearBits := []int{1, 32, 99}
	for _, bit := range clearBits {
		bs.Clear(bit)
		if bs.IsSet(bit) {
			t.Errorf("bit %d should be clear after Clear()", bit)
		}
	}
	// Verify remaining bits are still set
	expectedSet := []int{0, 7, 31, 63, 64}
	for _, bit := range expectedSet {
		if !bs.IsSet(bit) {
			t.Errorf("bit %d should still be set", bit)
		}
	}
}

func TestBitSetBoundaryConditions(t *testing.T) {
	bs := NewBitSet(64)
	// Test out of bounds access - should not panic
	bs.Set(-1)   // should be ignored
	bs.Set(64)   // should be ignored
	bs.Set(1000) // should be ignored
	if bs.IsSet(-1) || bs.IsSet(64) || bs.IsSet(1000) {
		t.Error("out of bounds bits should return false")
	}
	// Test boundary bits
	bs.Set(0)
	bs.Set(63)
	if !bs.IsSet(0) || !bs.IsSet(63) {
		t.Error("boundary bits 0 and 63 should be settable")
	}
}

func TestBitSetNextBit(t *testing.T) {
	bs := NewBitSet(100)
	// Empty bitset should return -1
	if pos := bs.NextBit(-1); pos != -1 {
		t.Errorf("empty bitset NextBit(-1) should return -1, got %d", pos)
	}
	// Set some bits: 5, 10, 64, 95
	testBits := []int{5, 10, 64, 95}
	for _, bit := range testBits {
		bs.Set(bit)
	}
	// Test iteration through all set bits
	var found []int
	pos := -1
	for {
		pos = bs.NextBit(pos)
		if pos == -1 {
			break
		}
		found = append(found, pos)
	}
	if len(found) != len(testBits) {
		t.Errorf("expected %d bits, found %d: %v", len(testBits), len(found), found)
	}
	for i, expected := range testBits {
		if i >= len(found) || found[i] != expected {
			t.Errorf("expected bit %d at position %d, got %v", expected, i, found)
		}
	}
	// Test starting from specific positions
	pos = bs.NextBit(7) // Should find 10
	if pos != 10 {
		t.Errorf("NextBit(7) should return 10, got %d", pos)
	}
	pos = bs.NextBit(10) // Should find 64
	if pos != 64 {
		t.Errorf("NextBit(10) should return 64, got %d", pos)
	}
	pos = bs.NextBit(95) // Should return -1 (no more bits)
	if pos != -1 {
		t.Errorf("NextBit(95) should return -1, got %d", pos)
	}
}

func TestBitSetCopy(t *testing.T) {
	bs1 := NewBitSet(100)
	bs1.Set(5)
	bs1.Set(64)
	bs1.Set(99)

	bs2 := NewBitSet(50) // Different size initially
	bs2.Copy(bs1)

	if bs2.Length != bs1.Length {
		t.Errorf("after copy, length should be %d, got %d", bs1.Length, bs2.Length)
	}
	// Check that all bits match
	for i := 0; i < bs1.Length; i++ {
		if bs1.IsSet(i) != bs2.IsSet(i) {
			t.Errorf("Bit %d differs after copy: bs1=%v, bs2=%v", i, bs1.IsSet(i), bs2.IsSet(i))
		}
	}
	// Modifying bs2 should not affect bs1
	bs2.Set(10)
	if bs1.IsSet(10) {
		t.Error("modifying copied bitset should not affect original")
	}
}

func TestBitSetOr(t *testing.T) {
	bs1 := NewBitSet(100)
	bs1.Set(5)
	bs1.Set(10)

	bs2 := NewBitSet(100)
	bs2.Set(10) // Overlapping bit
	bs2.Set(20)

	bs1.Or(bs2)

	expectedBits := []int{5, 10, 20}
	for _, bit := range expectedBits {
		if !bs1.IsSet(bit) {
			t.Errorf("bit %d should be set after OR operation", bit)
		}
	}
	// Check that no unexpected bits are set
	count := 0
	pos := -1
	for {
		pos = bs1.NextBit(pos)
		if pos == -1 {
			break
		}
		count++
	}
	if count != len(expectedBits) {
		t.Errorf("expected %d bits set after OR, got %d", len(expectedBits), count)
	}
}

func TestBitSetEqual(t *testing.T) {
	bs1 := NewBitSet(100)
	bs2 := NewBitSet(100)
	// Empty bitsets should be equal
	if !bs1.Equal(bs2) {
		t.Error("empty bitsets should be equal")
	}
	// Set same bits
	testBits := []int{5, 10, 64, 95}
	for _, bit := range testBits {
		bs1.Set(bit)
		bs2.Set(bit)
	}
	if !bs1.Equal(bs2) {
		t.Error("Bitsets with same bits should be equal")
	}
	// Set different bit in bs2
	bs2.Set(50)
	if bs1.Equal(bs2) {
		t.Error("Bitsets with different bits should not be equal")
	}
	// Different sizes should not be equal
	bs3 := NewBitSet(50)
	for _, bit := range testBits {
		if bit < 50 {
			bs3.Set(bit)
		}
	}
	if bs1.Equal(bs3) {
		t.Error("Bitsets with different sizes should not be equal")
	}
}

func TestBitSetWordBoundaries(t *testing.T) {
	bs := NewBitSet(200)
	// Test bits around word boundaries (64-bit boundaries)
	boundaryBits := []int{62, 63, 64, 65, 126, 127, 128, 129}
	for _, bit := range boundaryBits {
		bs.Set(bit)
	}
	// Verify all boundary bits are set
	for _, bit := range boundaryBits {
		if !bs.IsSet(bit) {
			t.Errorf("Boundary bit %d should be set", bit)
		}
	}
	// Test NextBit across word boundaries
	var found []int
	pos := -1
	for {
		pos = bs.NextBit(pos)
		if pos == -1 {
			break
		}
		found = append(found, pos)
	}
	if len(found) != len(boundaryBits) {
		t.Errorf("Expected %d boundary bits, found %d: %v", len(boundaryBits), len(found), found)
	}
	for i, expected := range boundaryBits {
		if i >= len(found) || found[i] != expected {
			t.Errorf("Expected boundary bit %d at position %d, got %v", expected, i, found)
		}
	}
}

// Benchmark tests
func BenchmarkBitSetSet(b *testing.B) {
	bs := NewBitSet(10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bs.Set(i % 10000)
	}
}

func BenchmarkBitSetIsSet(b *testing.B) {
	bs := NewBitSet(10000)
	// Set every 10th bit
	for i := 0; i < 10000; i += 10 {
		bs.Set(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bs.IsSet(i % 10000)
	}
}

func BenchmarkBitSetNextBit(b *testing.B) {
	bs := NewBitSet(10000)
	// Set every 100th bit
	for i := 0; i < 10000; i += 100 {
		bs.Set(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pos := -1
		for {
			pos = bs.NextBit(pos)
			if pos == -1 {
				break
			}
		}
	}
}
