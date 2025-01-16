package main

import (
	"fmt"
	"math/rand"
	"time"
)

// ANSI color codes for green shades
var greenShades = []string{
	"\033[38;5;28m", // Dark green
	"\033[38;5;34m", // Medium green
	"\033[38;5;40m", // Light green
	"\033[38;5;46m", // Bright green
}

const reset = "\033[0m"

func randomShade() string {
	return greenShades[rand.Intn(len(greenShades))]
}

func printPalmTree() {
	rand.Seed(time.Now().UnixNano())

	// Leaves (top of the tree)
	for i := 0; i < 6; i++ {
		padding := 6 - i
		line := ""
		for j := 0; j < i*2+1; j++ {
			line += randomShade() + "█" + reset
		}
		fmt.Printf("%s%s\n", spaces(padding), line)
	}

	// Trunk (middle of the tree)
	for i := 0; i < 8; i++ {
		fmt.Printf("%s%s\n", spaces(6), "\033[38;5;94m█\033[0m") // Brownish trunk
	}

	// Ground (base of the tree)
	fmt.Printf("\033[38;5;94m%s\n%s\n%s\033[0m\n", spaces(4)+"███", spaces(3)+"█████", spaces(2)+"███████")
}

func spaces(count int) string {
	return fmt.Sprintf("%*s", count, "")
}

func main() {
	printPalmTree()
}
