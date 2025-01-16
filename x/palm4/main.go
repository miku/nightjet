package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/fatih/color"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// Create some shades of green for the leaves
	greens := []*color.Color{
		color.New(color.FgHiGreen),
		color.New(color.FgGreen),
	}

	// Draw the stem of the palm tree
	for i := 0; i < 2; i++ {
		fmt.Println(" |")
	}

	// Draw the leaves of the palm tree
	for i := 0; i < 5; i++ {
		// Choose a random shade of green for this leaf
		g := greens[rand.Intn(len(greens))]
		// Create the leaf using solid green block characters
		leaf := strings.Repeat("█", i+1)
		// Print the leaf in the chosen color
		g.Println(leaf)
	}
}
