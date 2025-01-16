package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/fatih/color"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// Define colors
	green := color.New(color.FgGreen).SprintFunc()
	brown := color.New(color.FgYellow).SprintFunc() // Using yellow for brown-ish
	coconut := color.New(color.FgWhite).SprintFunc()

	// Randomize leaf shape slightly
	leafTop := []string{".~~.", ".~.~", ".~~~"}
	leafMid := []string{"/~~~\\", "/~ ~\\", "/~~~~"}
	leafBottom := []string{"\\~~~/", "\\~ ~/", "\\~~~~"}
	leafIndex := rand.Intn(3)
	fmt.Println("     " + green(leafTop[leafIndex]))
	fmt.Println("    " + green(leafMid[leafIndex]))
	fmt.Println("   |     |")
	fmt.Println("   " + green(leafBottom[leafIndex]))
	fmt.Println("    `~~'")

	// Trunk height variation
	trunkHeight := rand.Intn(3) + 3 // Trunk length between 3 and 5

	for i := 0; i < trunkHeight; i++ {
		fmt.Println("      " + brown("||"))
	}

	fmt.Println(" --------")
	fmt.Println("| " + coconut("O") + "    " + coconut("O") + " |")
	fmt.Println("|________|")
}
