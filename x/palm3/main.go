package main

import (
	"fmt"
	"math/rand"
	"time"
)

// ANSI escape codes for colors
const (
	reset = "\033[0m"
	green = "\033[32m"
	brown = "\033[33m"
)

// Randomize leaf patterns
func randomLeaf() string {
	leafPatterns := []string{
		"   /  |  \\",
		"  /   |   \\",
		"  \\   |   /",
		"   \\__|__/",
	}
	return leafPatterns[rand.Intn(len(leafPatterns))]
}

func main() {
	rand.Seed(time.Now().UnixNano())

	// Palm tree leaves
	fmt.Println(green + "    __ __" + reset)
	for i := 0; i < 4; i++ {
		fmt.Println(green + randomLeaf() + reset)
	}

	// Trunk
	for i := 0; i < 6; i++ {
		fmt.Println(brown + "      |" + reset)
	}

	// Base
	fmt.Println(brown + "     / \\" + reset)
	fmt.Println(brown + "    /   \\" + reset)
	fmt.Println(brown + "   /     \\" + reset)
}
