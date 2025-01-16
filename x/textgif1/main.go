package main

import (
	"flag"
	"image"
	"image/color"
	"image/gif"
	"io/ioutil"
	"log"
	"os"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
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

	if len(text) > 0 {
		cursorX := 20 + (len(text) * 13)
		c.DrawString("|", freetype.Pt(cursorX, 40))
	}

	return img
}

func main() {
	var (
		text     = flag.String("text", "Hello, World!", "Text to animate")
		output   = flag.String("output", "output.gif", "Output file name")
		delay    = flag.Int("delay", 10, "Delay between frames (100ths of seconds)")
		endDelay = flag.Int("end-delay", 30, "Delay after last character (100ths of seconds)")
		fontPath = flag.String("font", "fonts/NotoSans-Regular.ttf", "Path to font file") // https://github.com/golang/freetype/issues/8
	)
	flag.Parse()

	fontBytes, err := ioutil.ReadFile(*fontPath)
	if err != nil {
		log.Fatalf("Error reading font file: %v", err)
	}

	f, err := truetype.Parse(fontBytes)
	if err != nil {
		log.Fatalf("Error parsing font: %v", err)
	}

	width := 20 + (len(*text) * 15)
	height := 60

	var images []*image.Paletted
	var delays []int

	for i := 0; i <= len(*text); i++ {
		img := createFrame((*text)[:i], f, width, height)
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
