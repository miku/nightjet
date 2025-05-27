 ```go
package main

import (
	"fmt"
	"math/rand"
	"time"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	printPalmTree()
}

func printPalmTree() {
	height := 15 + rand.Intn(10)
	leaves := 3 + rand.Intn(3)

	for i := 0; i < height; i++ {
		spaces := height - i - 1
		for j := 0; j < spaces; j++ {
			fmt.Print(" ")
		}
		for j := 0; j < leaves; j++ {
			fmt.Print("\033[32m*\033[0m")
		}
		fmt.Println()
		if i%2 == 0 && leaves < 10 {
			leaves += 2
		}
	}

	trunkHeight := 3 + rand.Intn(3)
	for i := 0; i < trunkHeight; i++ {
		spaces := height - 1
		for j := 0; j < spaces; j++ {
			fmt.Print(" ")
		}
		fmt.Println("\033[33m|\033[0m")
	}
}
```