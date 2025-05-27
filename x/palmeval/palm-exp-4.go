package main

import (
	"fmt"
	"math/rand"
	"time"
)

var colors = []string{"\033[32m", "\033[33m", "\033[38;5;28m"}

func main() {
	rand.Seed(time.Now().UnixNano())
	trunkHeight := rand.Intn(5) + 3
	leavesHeight := rand.Intn(10) + 10

	for i := 0; i < trunkHeight; i++ {
		fmt.Println("   " + colors[0] + "|")
	}

	for i := 0; i < leavesHeight; i++ {
		fmt.Print(colors[rand.Intn(3)])
		for j := 0; j < leavesHeight-i; j++ {
			fmt.Print(" ")
		}
		for k := 0; k < 2*i+1; k++ {
			fmt.Print("*")
		}
		fmt.Println()
	}

	fmt.Println("\033[0m")
}
