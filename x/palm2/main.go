package main

import (
	"fmt"
	"strings"
)

func main() {
	// Draw the palm tree leaves
	leaves := []string{
		"          ,",
		"     /\\  ||  /\\",
		"    //\\\\ || //\\\\",
		"   //  \\\\||//  \\\\",
		"  //    \\||/    \\\\",
		" //      \\/      \\\\",
		"//                \\\\",
		"\\\\      /\\      //",
		" \\\\    /  \\    //",
		"  \\\\  /    \\  //",
		"   \\\\/ |==| \\/",
	}

	// Draw the trunk
	trunk := []string{
		"      |==|",
		"      |==|",
		"      |==|",
		"      |==|",
		"      |==|",
		"     /|==|\\",
		"    //|==|\\\\",
	}

	// Draw the base
	base := []string{
		"   ///|==|\\\\\\",
		"  ////|==|\\\\\\\\",
		" /////|==|\\\\\\\\\\",
		"~~~~~~~~~~~~~~~~~~~~~~",
	}

	// Print the entire palm tree
	for _, line := range leaves {
		fmt.Println(line)
	}
	for _, line := range trunk {
		fmt.Println(line)
	}
	for _, line := range base {
		fmt.Println(line)
	}
}

// Function to center text (can be used for modifications)
func centerText(text string, width int) string {
	padding := (width - len(text)) / 2
	return strings.Repeat(" ", padding) + text
}
