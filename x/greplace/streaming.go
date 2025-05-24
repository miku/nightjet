package main

import (
	"fmt"
	"io"
)

const (
	// Buffer sizes for streaming processing
	ReadBufferSize  = 64 * 1024 // 64KB read chunks
	WriteBufferSize = 64 * 1024 // 64KB write buffer
	MaxPatternLen   = 1024      // Maximum pattern length for overlap handling
)

// StreamingReplacementEngine processes input in fixed-size chunks to avoid memory growth
type StreamingReplacementEngine struct {
	dfa           []DFAState
	patterns      *PatternProcessor
	updated       bool
	maxPatternLen int

	// Reusable buffers to avoid allocations
	readBuf    []byte
	writeBuf   []byte
	overlapBuf []byte
}

// NewStreamingReplacementEngine creates a memory-efficient replacement engine
func NewStreamingReplacementEngine(dfa []DFAState, patterns *PatternProcessor) *StreamingReplacementEngine {
	// Calculate maximum pattern length for overlap handling
	maxLen := 0
	for _, pattern := range patterns.GetPatterns() {
		if len(pattern.From) > maxLen {
			maxLen = len(pattern.From)
		}
	}
	if maxLen > MaxPatternLen {
		maxLen = MaxPatternLen
	}

	return &StreamingReplacementEngine{
		dfa:           dfa,
		patterns:      patterns,
		updated:       false,
		maxPatternLen: maxLen,
		readBuf:       make([]byte, ReadBufferSize),
		writeBuf:      make([]byte, 0, WriteBufferSize),
		overlapBuf:    make([]byte, 0, maxLen*2),
	}
}

// ReplaceStream performs streaming replacement from reader to writer
func (sre *StreamingReplacementEngine) ReplaceStream(reader io.Reader, writer io.Writer) (bool, error) {
	sre.updated = false

	// Reset reusable buffers
	sre.writeBuf = sre.writeBuf[:0]
	sre.overlapBuf = sre.overlapBuf[:0]

	for {
		// Read next chunk
		n, err := reader.Read(sre.readBuf)
		if n == 0 {
			if err == io.EOF {
				break
			}
			if err != nil {
				return false, fmt.Errorf("read error: %v", err)
			}
			continue
		}

		// Combine overlap from previous chunk with new data
		currentChunk := sre.combineChunks(sre.readBuf[:n])

		// Process this chunk
		processedData, overlap, updated := sre.processChunk(currentChunk)

		if updated {
			sre.updated = true
		}

		// Write processed data (excluding overlap for next iteration)
		if len(processedData) > 0 {
			err = sre.writeBuffer(writer, processedData)
			if err != nil {
				return false, fmt.Errorf("write error: %v", err)
			}
		}

		// Save overlap for next iteration
		sre.overlapBuf = sre.overlapBuf[:0]
		sre.overlapBuf = append(sre.overlapBuf, overlap...)

		// Check for read completion
		if err == io.EOF {
			// Process final overlap
			if len(sre.overlapBuf) > 0 {
				finalData, _, finalUpdated := sre.processChunk(sre.overlapBuf)
				if finalUpdated {
					sre.updated = true
				}
				if len(finalData) > 0 {
					err = sre.writeBuffer(writer, finalData)
					if err != nil {
						return false, fmt.Errorf("final write error: %v", err)
					}
				}
			}
			break
		}
	}

	return sre.updated, nil
}

// combineChunks combines overlap buffer with new chunk data
func (sre *StreamingReplacementEngine) combineChunks(newData []byte) []byte {
	if len(sre.overlapBuf) == 0 {
		return newData
	}

	// Reuse a temporary buffer to combine data
	combined := make([]byte, len(sre.overlapBuf)+len(newData))
	copy(combined, sre.overlapBuf)
	copy(combined[len(sre.overlapBuf):], newData)

	return combined
}

// processChunk processes a chunk of data, returning processed data and overlap
func (sre *StreamingReplacementEngine) processChunk(chunk []byte) (processedData []byte, overlap []byte, updated bool) {
	if len(chunk) == 0 {
		return nil, nil, false
	}

	// Reserve overlap region (last maxPatternLen bytes)
	overlapStart := len(chunk)
	if len(chunk) > sre.maxPatternLen {
		overlapStart = len(chunk) - sre.maxPatternLen
	}

	// Process the main part (excluding overlap region)
	mainData := chunk[:overlapStart]
	overlapData := chunk[overlapStart:]

	// Apply replacements to main data
	result, wasUpdated := sre.replaceInBytes(mainData)

	return result, overlapData, wasUpdated
}

// replaceInBytes performs replacement on a byte slice using minimal allocations
func (sre *StreamingReplacementEngine) replaceInBytes(data []byte) ([]byte, bool) {
	if len(data) == 0 {
		return data, false
	}

	// Reset write buffer for reuse
	sre.writeBuf = sre.writeBuf[:0]

	pos := 0
	updated := false

	for pos < len(data) {
		// Try to find a match at current position using brute force for now
		matchLen, replacement := sre.findMatchAtPosition(data, pos)

		if matchLen > 0 {
			// Found a match - append replacement
			sre.writeBuf = append(sre.writeBuf, []byte(replacement)...)
			pos += matchLen
			updated = true
		} else {
			// No match - copy original byte
			sre.writeBuf = append(sre.writeBuf, data[pos])
			pos++
		}

		// Prevent write buffer from growing too large
		if len(sre.writeBuf) > WriteBufferSize {
			// This shouldn't happen with normal patterns, but safety check
			break
		}
	}

	// Return a copy of the processed data
	result := make([]byte, len(sre.writeBuf))
	copy(result, sre.writeBuf)

	return result, updated
}

// findMatchAtPosition finds the longest pattern match at the given position
func (sre *StreamingReplacementEngine) findMatchAtPosition(data []byte, pos int) (int, string) {
	if pos >= len(data) {
		return 0, ""
	}

	patterns := sre.patterns.GetPatterns()
	bestLength := 0
	bestReplacement := ""

	// Try each pattern at current position
	for _, pattern := range patterns {
		if sre.matchesAtPosition(data, pos, pattern.From) {
			if len(pattern.From) > bestLength {
				bestLength = len(pattern.From)
				bestReplacement = pattern.To
			}
		}
	}

	return bestLength, bestReplacement
}

// matchesAtPosition checks if a pattern matches at the given position
func (sre *StreamingReplacementEngine) matchesAtPosition(data []byte, pos int, pattern string) bool {
	if pos+len(pattern) > len(data) {
		return false
	}

	patternBytes := []byte(pattern)
	for i, b := range patternBytes {
		if data[pos+i] != b {
			return false
		}
	}

	return true
}

// writeBuffer writes data to the writer, handling partial writes
func (sre *StreamingReplacementEngine) writeBuffer(writer io.Writer, data []byte) error {
	pos := 0
	for pos < len(data) {
		n, err := writer.Write(data[pos:])
		if err != nil {
			return err
		}
		pos += n
	}
	return nil
}
