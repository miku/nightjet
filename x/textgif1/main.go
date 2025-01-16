package main

import (
	"flag"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
)

type Line struct {
	text string
	yPos int
}

func parseHexColor(hex string) color.Color {
	hex = strings.TrimPrefix(hex, "#")
	r, _ := strconv.ParseUint(hex[0:2], 16, 8)
	g, _ := strconv.ParseUint(hex[2:4], 16, 8)
	b, _ := strconv.ParseUint(hex[4:6], 16, 8)
	return color.RGBA{uint8(r), uint8(g), uint8(b), 255}
}

func measureTextWidth(text string, c *freetype.Context) int {
	bounds, _ := c.DrawString(text, freetype.Pt(0, 0))
	return int(bounds.X.Round())
}

func wordWrap(text string, maxWidth int, c *freetype.Context) []string {
	var lines []string
	var currentLine string
	words := strings.Fields(text)

	for _, word := range words {
		testLine := currentLine
		if testLine != "" {
			testLine += " "
		}
		testLine += word

		if measureTextWidth(testLine, c) <= maxWidth {
			currentLine = testLine
		} else {
			if currentLine != "" {
				lines = append(lines, currentLine)
			}
			currentLine = word
		}
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}
	return lines
}

func createFrame(lines []Line, currentText string, font *truetype.Font, width, height int, showCursor bool, yOffset int, textColor, bgColor color.Color) *image.Paletted {
	palette := color.Palette{
		bgColor,
		textColor,
	}

	img := image.NewPaletted(
		image.Rect(0, 0, width, height),
		palette,
	)

	draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(font)
	c.SetFontSize(24)
	c.SetClip(img.Bounds())
	c.SetDst(img)
	c.SetSrc(&image.Uniform{textColor})

	startX := 20

	// Draw existing lines
	for _, line := range lines {
		if line.yPos < height {
			pt := freetype.Pt(startX, line.yPos)
			c.DrawString(line.text, pt)
		}
	}

	// Draw current line with cursor
	currentY := lines[len(lines)-1].yPos + yOffset
	pt := freetype.Pt(startX, currentY)
	c.DrawString(currentText, pt)

	if showCursor {
		textWidth := measureTextWidth(currentText, c)
		cursorX := startX + textWidth
		cursorY := currentY - 20
		cursor := image.Rect(cursorX, cursorY, cursorX+13, cursorY+25)
		draw.Draw(img, cursor, &image.Uniform{textColor}, image.Point{}, draw.Over)
	}

	return img
}

func randomJitter(maxJitter int) int {
	return rand.Intn(maxJitter*2+1) - maxJitter
}

func randomDelay(baseDelay int, jitterMs int) int {
	jitter := rand.Intn(jitterMs*2+1) - jitterMs
	return baseDelay + jitter
}

func main() {
	rand.Seed(time.Now().UnixNano())

	var (
		text          = flag.String("text", "Hello, World!", "Text to animate")
		output        = flag.String("output", "output.gif", "Output file name")
		delay         = flag.Int("delay", 10, "Base delay between frames (100ths of seconds)")
		endDelay      = flag.Int("end-delay", 30, "Delay after last character (100ths of seconds)")
		initialBlinks = flag.Int("blinks", 3, "Number of cursor blinks before animation")
		blinkDelay    = flag.Int("blink-delay", 50, "Delay for cursor blinks (100ths of seconds)")
		jitter        = flag.Int("jitter", 0, "Maximum vertical jitter in pixels")
		delayJitter   = flag.Int("delay-jitter", 3, "Maximum delay jitter in 100ths of seconds")
		textColorHex  = flag.String("text-color", "#000000", "Text color in hex format (e.g. #FF0000)")
		bgColorHex    = flag.String("bg-color", "#FFFFFF", "Background color in hex format (e.g. #FFFFFF)")
		width         = flag.Int("width", 600, "GIF width in pixels")
		height        = flag.Int("height", 200, "GIF height in pixels")
	)
	flag.Parse()

	textColor := parseHexColor(*textColorHex)
	bgColor := parseHexColor(*bgColorHex)

	fontBytes, err := ioutil.ReadFile("fonts/Helvetica.ttf")
	if err != nil {
		log.Fatalf("Error reading font file: %v", err)
	}

	f, err := truetype.Parse(fontBytes)
	if err != nil {
		log.Fatalf("Error parsing font: %v", err)
	}

	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(f)
	c.SetFontSize(24)

	maxWidth := *width - 40
	var completedLines []Line
	lineHeight := 30
	baseY := 40

	completedLines = append(completedLines, Line{text: "", yPos: baseY})

	var images []*image.Paletted
	var delays []int

	// Initial blinking cursor
	for i := 0; i < *initialBlinks*2; i++ {
		img := createFrame(completedLines, "", f, *width, *height, i%2 == 0, 0, textColor, bgColor)
		images = append(images, img)
		delays = append(delays, *blinkDelay)
	}

	// Text animation
	currentText := ""
	for pos, char := range *text {
		currentText += string(char)
		textWidth := measureTextWidth(currentText, c)

		if textWidth > maxWidth || char == '\n' {
			completedLines[len(completedLines)-1].text = strings.TrimSpace(currentText[:len(currentText)-1])
			currentText = string(char)
			if char == '\n' {
				currentText = ""
			}

			// Shift lines up if needed
			if len(completedLines)*lineHeight > *height-lineHeight {
				for i := range completedLines {
					completedLines[i].yPos -= lineHeight
				}
			}

			completedLines = append(completedLines, Line{text: "", yPos: baseY + (len(completedLines) * lineHeight)})
		}

		yOffset := randomJitter(*jitter)
		img := createFrame(completedLines, currentText, f, *width, *height, true, yOffset, textColor, bgColor)
		images = append(images, img)

		if pos == len(*text)-1 {
			delays = append(delays, *endDelay)
		} else {
			delays = append(delays, randomDelay(*delay, *delayJitter))
		}
	}

	// Add final line if needed
	if currentText != "" {
		completedLines[len(completedLines)-1].text = currentText
	}

	outFile, err := os.Create(*output)
	if err != nil {
		log.Fatalf("Error creating output file: %v", err)
	}
	defer outFile.Close()

	if err := gif.EncodeAll(outFile, &gif.GIF{
		Image: images,
		Delay: delays,
	}); err != nil {
		log.Fatalf("Error encoding gif: %v", err)
	}
}
