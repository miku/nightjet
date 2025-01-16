package main

import (
	"fmt"
	"math/rand"
	"time"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// Function to generate a shade of green using ANSI escape codes
	greenShade := func() string {
		r := 0
		g := rand.Intn(156) + 100 // Green values between 100 and 255
		b := 0
		return fmt.Sprintf("\033[38;2;%d;%d;%dm█\033[0m", r, g, b)
	}

	// Trunk height
	trunkHeight := rand.Intn(3) + 4

	// Draw trunk
	for i := 0; i < trunkHeight; i++ {
		fmt.Println("     │")
	}

	// Draw leaves
	leafWidth := rand.Intn(6) + 7 // Odd numbers for centered leaves
	for i := 0; i < 3; i++ {      // 3 rows of leaves
		padding := (leafWidth - 1) / 2
		for j := 0; j < padding; j++ {
			fmt.Print(" ")
		}
		for j := 0; j < 1; j++ {
			fmt.Print(greenShade())
		}
		for j := 0; j < leafWidth-2; j++ {
			fmt.Print(greenShade())
		}
		for j := 0; j < 1; j++ {
			fmt.Print(greenShade())
		}
		fmt.Println()
	}
}
