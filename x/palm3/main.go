package main

import "fmt"

func main() {
	palmTree := []string{
		"    __ __",
		"   /  V  \\",
		"  /   |   \\",
		"  \\   |   /",
		"   \\__|__/",
		"      |",
		"      |",
		"      |",
		"      |",
		"      |",
		"     / \\",
		"    /   \\",
		"   /     \\",
	}

	for _, line := range palmTree {
		fmt.Println(line)
	}
}
