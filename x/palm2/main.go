package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// ANSI color codes for different shades of green and brown
const (
	lightGreen  = "\033[38;2;144;238;144m" // Light green
	mediumGreen = "\033[38;2;34;139;34m"   // Medium green
	darkGreen   = "\033[38;2;0;100;0m"     // Dark green
	brown       = "\033[38;2;139;69;19m"   // Saddle brown
	reset       = "\033[0m"
)

// Unicode block characters
const (
	fullBlock  = "█"
	upperBlock = "▀"
	lowerBlock = "▄"
	leftBlock  = "▌"
	rightBlock = "▐"
)

func colorize(text, color string) string {
	return color + text + reset
}

func randomGreen() string {
	greens := []string{lightGreen, mediumGreen, darkGreen}
	return greens[rand.Intn(len(greens))]
}

func generateLeaf(length int, direction int) string {
	var leaf strings.Builder
	blocks := []string{fullBlock, upperBlock, lowerBlock}

	if direction > 0 { // Right-facing leaf
		leaf.WriteString(strings.Repeat(" ", 15))
		for i := 0; i < length; i++ {
			leaf.WriteString(colorize(blocks[rand.Intn(len(blocks))], randomGreen()))
		}
	} else { // Left-facing leaf
		for i := 0; i < length; i++ {
			leaf.WriteString(colorize(blocks[rand.Intn(len(blocks))], randomGreen()))
		}
		leaf.WriteString(strings.Repeat(" ", 15-length))
	}

	return leaf.String()
}

func main() {
	rand.Seed(time.Now().UnixNano())

	// Generate palm fronds (leaves)
	numLeaves := 7
	leaves := make([]string, numLeaves*2) // Double for left and right sides

	// Generate right-facing leaves
	for i := 0; i < numLeaves; i++ {
		length := rand.Intn(3) + 8 // Length between 8-10 blocks
		leaves[i] = generateLeaf(length, 1)
	}

	// Generate left-facing leaves
	for i := 0; i < numLeaves; i++ {
		length := rand.Intn(3) + 8 // Length between 8-10 blocks
		leaves[i+numLeaves] = generateLeaf(length, -1)
	}

	// Create slim trunk
	trunkHeight := 12
	trunk := make([]string, trunkHeight)
	for i := range trunk {
		// Add slight curve to trunk
		offset := int(2 * rand.Float64())
		trunk[i] = strings.Repeat(" ", 15+offset) + colorize("│", brown)
	}

	// Print the palm tree
	fmt.Println()

	// Print leaves with overlapping effect
	for i := 0; i < numLeaves*2; i++ {
		if i < numLeaves {
			// Right-facing leaves, angled downward
			fmt.Println(strings.Repeat(" ", i) + leaves[i])
		} else {
			// Left-facing leaves, angled downward
			fmt.Println(strings.Repeat(" ", numLeaves*2-i) + leaves[i])
		}
	}

	// Print trunk
	for _, line := range trunk {
		fmt.Println(line)
	}

	// Print base
	base := strings.Repeat(" ", 15) + colorize("╨", brown)
	fmt.Println(base)
	fmt.Println()
}
