package main

import (
	"fmt"
	"math/rand"
	"time"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	height := rand.Intn(10) + 15
	width := rand.Intn(5) + 7

	colors := []string{"\033[32m", "\033[33m", "\033[36m"} // Green, Yellow, Cyan

	for i := 0; i < height; i++ {
		leaves := ""
		if i > height-3 {
			for j := 0; j < width; j++ {
				leaves += "🌿"
			}
		} else {
			numLeaves := rand.Intn(width) + 1
			for j := 0; j < numLeaves; j++ {
				leaves += "🌴"
			}
		}

		color := colors[rand.Intn(len(colors))]
		fmt.Println(color + leaves + "\033[0m")
	}

	trunkHeight := height - 10
	if trunkHeight < 3 {
		trunkHeight = 3
	}

	for i := 0; i < trunkHeight; i++ {
		padding := (width - 1) / 2
		trunk := ""
		for j := 0; j < padding; j++ {
			trunk += " "
		}
		trunk += "||"
		fmt.Println("\033[34m" + trunk + "\033[0m")
	}
}
