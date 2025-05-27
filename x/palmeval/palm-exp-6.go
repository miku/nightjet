package main

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
)

const (
	cvsH = 25 // Canvas Height
	cvsW = 50 // Canvas Width

	tc = "┃" // Trunk Character

	brown = "\033[33m" // ANSI Yellow, often appears Brownish
	green = "\033[32m" // ANSI Green
	reset = "\033[0m"  // ANSI Reset
)

var leafChars = []string{"*", "~", "#", "@"} // Characters used for leaves

func main() {
	rand.Seed(time.Now().UnixNano())

	cvs := make([][]string, cvsH) // cvs is the canvas for drawing
	for i := range cvs {
		cvs[i] = make([]string, cvsW)
		for j := range cvs[i] {
			cvs[i][j] = " " // Initialize with spaces
		}
	}

	// --- Trunk Generation ---
	tH := 10 + rand.Intn(8) // Trunk Height: 10 to 17 characters
	if tH > cvsH-5 {        // Ensure trunk isn't too tall, leaving space for fronds
		tH = cvsH - 5
	}
	if tH < 5 { // Ensure minimum trunk height
		tH = 5
	}

	tBaseC := cvsW/2 - 1     // Trunk Base Column (left character of a 2-char wide trunk)
	tCols := make([]int, tH) // Stores the left column index for each trunk segment

	currLean := 0                      // Current lean offset from tBaseC
	maxLean := 2                       // Maximum absolute lean offset
	leanChangeFreq := 2 + rand.Intn(2) // How often lean might change (every 2-3 segments)

	for r := 0; r < tH; r++ { // r is segment index from bottom (0) to top (tH-1)
		if r > 0 && r%leanChangeFreq == 0 {
			leanDelta := rand.Intn(3) - 1 // -1, 0, or 1
			currLean += leanDelta
			if currLean > maxLean {
				currLean = maxLean
			}
			if currLean < -maxLean {
				currLean = -maxLean
			}
		}
		tCols[r] = tBaseC + currLean
	}

	topTY := cvsH - tH // Y-coordinate (row index) of the highest trunk part

	for r := 0; r < tH; r++ {
		y := cvsH - 1 - r // Map segment r to canvas row y
		actualC := tCols[r]

		if y >= 0 && y < cvsH { // Check bounds for y
			// Draw left part of trunk
			if actualC >= 0 && actualC < cvsW {
				cvs[y][actualC] = brown + tc + reset
			}
			// Draw right part of trunk
			if actualC+1 >= 0 && actualC+1 < cvsW {
				cvs[y][actualC+1] = brown + tc + reset
			}
		}
	}

	fOrgX := tBaseC + 1 // Default Frond Origin X (center of base trunk)
	if tH > 0 {
		fOrgX = tCols[tH-1] + 1 // Frond origin X based on the top-most, leaned trunk segment (right char)
	}
	fOrgY := topTY // Frond Origin Y

	// --- Frond Generation ---
	nFronds := 5 + rand.Intn(4) // Number of fronds: 5 to 8

	minAngle := math.Pi * 0.10 // Approx 18 degrees from horizontal right
	maxAngle := math.Pi * 0.90 // Approx 162 degrees
	angleRange := maxAngle - minAngle

	for i := 0; i < nFronds; i++ {
		fLen := 5 + rand.Intn(6) // Frond Length: 5 to 10 units

		var angle float64
		if nFronds == 1 { // Should not happen with current nFronds range, but good practice
			angle = minAngle + angleRange/2.0
		} else {
			angle = minAngle + (float64(i)/float64(nFronds-1))*angleRange
		}
		// Add random jitter to angle for variation
		angle += (rand.Float64() - 0.5) * (math.Pi / float64(nFronds+4))

		curveF := 0.04 + rand.Float64()*0.08          // Curvature factor (0.04 to 0.12)
		lChar := leafChars[rand.Intn(len(leafChars))] // Random leaf character for this frond

		for l := 0; l < fLen; l++ { // l is segment along the frond
			// Base projection of frond segment
			dx := float64(l) * math.Cos(angle)
			dy := float64(l) * math.Sin(angle) // dy is positive for upward angles (0 to Pi)

			// Apply curve (droop effect)
			// yOffC (y-offset due to curve) makes dy effectively smaller, then negative, causing downward curve
			yOffC := curveF * float64(l*l) * 0.25 // Quadratic curve, scaled

			// Calculate final coordinates for the frond segment
			fx := fOrgX + int(math.Round(dx))
			// dy*0.65 flattens fronds initially; yOffC adds the droop.
			// Canvas Y decreases upwards, so subtract for "up", add for "down".
			fy := fOrgY - int(math.Round(dy*0.65-yOffC))

			if fx >= 0 && fx < cvsW && fy >= 0 && fy < cvsH { // Check canvas bounds
				// Avoid frond overwriting the main trunk heavily, except near origin
				isTrunkPixel := false
				if fy > fOrgY && fy < cvsH && tH > 0 { // Only check if below frond origin and trunk exists
					trunkSegIdx := cvsH - 1 - fy
					if trunkSegIdx >= 0 && trunkSegIdx < tH {
						trunkColForY := tCols[trunkSegIdx]
						if fx == trunkColForY || fx == trunkColForY+1 {
							isTrunkPixel = true
						}
					}
				}

				if !isTrunkPixel || l < fLen/3 { // Allow fronds to start "on" the trunk
					cvs[fy][fx] = green + lChar + reset
				}
			}

			// Add random thickness to fronds
			if rand.Intn(3) == 0 && l > fLen/4 && l < fLen-1 { // For some segments, not too close to origin or tip
				sideFx := fx + (rand.Intn(3) - 1) // -1, 0, or 1 offset horizontally
				sideFy := fy + (rand.Intn(3) - 1) // -1, 0, or 1 offset vertically

				if sideFx >= 0 && sideFx < cvsW && sideFy >= 0 && sideFy < cvsH {
					// Check again to avoid drawing thickness on trunk
					isThickOnTrunk := false
					if sideFy > fOrgY && sideFy < cvsH && tH > 0 {
						trunkSegIdx := cvsH - 1 - sideFy
						if trunkSegIdx >= 0 && trunkSegIdx < tH {
							trunkColForY := tCols[trunkSegIdx]
							if sideFx == trunkColForY || sideFx == trunkColForY+1 {
								isThickOnTrunk = true
							}
						}
					}
					if !isThickOnTrunk {
						cvs[sideFy][sideFx] = green + lChar + reset
					}
				}
			}
		}
	}

	// --- Print Canvas ---
	for r := 0; r < cvsH; r++ {
		fmt.Println(strings.Join(cvs[r], ""))
	}
}
