package main

import "fmt"

func main() {
	// Draw the trunk of the palm tree
	for i := 0; i < 3; i++ {
		fmt.Println("  |")
	}

	// Draw the fronds of the palm tree
	for i := 0; i < 5; i++ {
		fmt.Println(" /\\")
	}
}
