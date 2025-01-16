package main

import (
	"flag"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"io/ioutil"
	"log"
	"os"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
)

func measureTextWidth(text string, c *freetype.Context) int {
	bounds, _ := c.DrawString(text, freetype.Pt(0, 0))
	return int(bounds.X.Round())
}

func createFrame(text string, font *truetype.Font, width, height int, showCursor bool) *image.Paletted {
	img := image.NewPaletted(
		image.Rect(0, 0, width, height),
		color.Palette{
			color.White,
			color.Black,
		},
	)

	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(font)
	c.SetFontSize(24)
	c.SetClip(img.Bounds())
	c.SetDst(img)
	c.SetSrc(image.Black)

	startX := 20
	pt := freetype.Pt(startX, 40)

	if text != "" {
		c.DrawString(text, pt)
	}

	if showCursor {
		textWidth := measureTextWidth(text, c)
		cursorX := startX + textWidth
		cursor := image.Rect(cursorX, 20, cursorX+13, 45)
		draw.Draw(img, cursor, &image.Uniform{color.Black}, image.Point{}, draw.Over)
	}

	return img
}

func main() {
	var (
		text          = flag.String("text", "Hello, World!", "Text to animate")
		output        = flag.String("output", "output.gif", "Output file name")
		delay         = flag.Int("delay", 10, "Delay between frames (100ths of seconds)")
		endDelay      = flag.Int("end-delay", 30, "Delay after last character (100ths of seconds)")
		initialBlinks = flag.Int("blinks", 3, "Number of cursor blinks before animation")
		blinkDelay    = flag.Int("blink-delay", 50, "Delay for cursor blinks (100ths of seconds)")
	)
	flag.Parse()

	fontBytes, err := ioutil.ReadFile("fonts/Helvetica.ttf")
	if err != nil {
		log.Fatalf("Error reading font file: %v", err)
	}

	f, err := truetype.Parse(fontBytes)
	if err != nil {
		log.Fatalf("Error parsing font: %v", err)
	}

	width := 600 // Fixed width to accommodate various text lengths
	height := 60

	var images []*image.Paletted
	var delays []int

	for i := 0; i < *initialBlinks*2; i++ {
		img := createFrame("", f, width, height, i%2 == 0)
		images = append(images, img)
		delays = append(delays, *blinkDelay)
	}

	for i := 0; i <= len(*text); i++ {
		img := createFrame((*text)[:i], f, width, height, true)
		images = append(images, img)
		if i == len(*text) {
			delays = append(delays, *endDelay)
		} else {
			delays = append(delays, *delay)
		}
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
