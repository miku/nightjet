package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/fatih/color"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// Create some colors for the fronds
	colors := []*color.Color{
		color.New(color.FgGreen),
		color.New(color.FgYellow),
		color.New(color.FgMagenta),
		color.New(color.FgCyan),
	}

	// Draw the trunk of the palm tree
	for i := 0; i < 3; i++ {
		fmt.Println(" |")
	}

	// Draw the fronds of the palm tree
	for i := 0; i < 5; i++ {
		// Choose a random color for this frond
		c := colors[rand.Intn(len(colors))]
		// Print the frond in the chosen color
		c.Println(" /\\")
	}
}
