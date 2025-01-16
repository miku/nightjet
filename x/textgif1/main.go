package main

import (
	"flag"
	"image"
	"image/color"
	"image/gif"
	"log"
	"os"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font/gofont/goregular"
)

func createFrame(text string, font *truetype.Font, width, height int) *image.Paletted {
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

	pt := freetype.Pt(20, 40)
	c.DrawString(text, pt)

	// Draw cursor if not at the end
	if len(text) > 0 {
		cursorX := 20 + (len(text) * 13) // Approximate character width
		c.DrawString("|", freetype.Pt(cursorX, 40))
	}

	return img
}

func main() {
	var (
		text   = flag.String("text", "Hello, World!", "Text to animate")
		output = flag.String("output", "output.gif", "Output file name")
		delay  = flag.Int("delay", 10, "Delay between frames (100ths of seconds)")
	)
	flag.Parse()

	// Load font
	f, err := truetype.Parse(goregular.TTF)
	if err != nil {
		log.Fatalf("Error loading font: %v", err)
	}

	// Calculate image dimensions
	width := 20 + (len(*text) * 15) // Add padding and approximate char width
	height := 60

	var images []*image.Paletted
	var delays []int

	// Create frames
	for i := 0; i <= len(*text); i++ {
		img := createFrame((*text)[:i], f, width, height)
		images = append(images, img)
		delays = append(delays, *delay)
	}

	// Create output file
	outFile, err := os.Create(*output)
	if err != nil {
		log.Fatalf("Error creating output file: %v", err)
	}
	defer outFile.Close()

	// Encode GIF
	if err := gif.EncodeAll(outFile, &gif.GIF{
		Image: images,
		Delay: delays,
	}); err != nil {
		log.Fatalf("Error encoding gif: %v", err)
	}
}
