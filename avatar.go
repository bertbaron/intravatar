package main

import (
	"bytes"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"log"

	"github.com/nfnt/resize"
	"github.com/oliamb/cutter"
)

func min(x, y int) int {
	if x <= y {
		return x
	}
	return y
}

func max(x, y int) int {
	if x >= y {
		return x
	}
	return y
}

// Avatar image with some metadata
type Avatar struct {
	size int
	data []byte
	// below are used in header fields
	cacheControl string
	lastModified string
}

func avatar2Image(avatar *Avatar) (img image.Image, format string, err error) {
	return image.Decode(bytes.NewBuffer(avatar.data))
}

// alters the avatar instance!
func image2Avatar(avatar *Avatar, img image.Image, format string) {
	b := new(bytes.Buffer)
	switch format {
	case "jpeg":
		jpeg.Encode(b, img, nil)
	case "gif":
		gif.Encode(b, img, nil)
	case "png":
		png.Encode(b, img)
	}
	avatar.data = b.Bytes()
}

// scales the avatar (altering it!)
func scale(avatar *Avatar, size int, requestFormat string) error {
	img, format, err := avatar2Image(avatar)
	if err != nil {
		return err
	}

	targetFormat := format
	if requestFormat != "" {
		targetFormat = requestFormat
	}
	actualSize := img.Bounds().Dx() // assume square
	log.Printf("Resizing img from %s %vx%v to %s %vx%v", format, actualSize, actualSize, targetFormat, size, size)
	resized := resize.Resize(uint(size), uint(size), img, resize.Bicubic)
	image2Avatar(avatar, resized, targetFormat)
	return nil
}

func cropAndScale(avatar *Avatar) error {
	img, format, err := avatar2Image(avatar)
	if err != nil {
		return err
	}
	x := img.Bounds().Dx()
	y := img.Bounds().Dy()
	size := min(x, y)
	if x != y {
		log.Printf("Cropping img from %vx%v to %vx%v", x, y, size, size)
		img, err = cutter.Crop(img, cutter.Config{
			Width:  size,
			Height: size,
			Mode:   cutter.Centered})
		if err != nil {
			return err
		}
	}
	if size > maxSize {
		log.Printf("Resizing img from %vx%v to %vx%v", size, size, maxSize, maxSize)
		img = resize.Resize(uint(maxSize), uint(maxSize), img, resize.Bicubic)
	}
	image2Avatar(avatar, img, format)
	return nil
}

func readImageFromBuffer(data *bytes.Buffer) *Avatar {
	return &Avatar{size: -1, data: data.Bytes()}
}

func readImageFromReader(reader io.Reader) *Avatar {
	b := new(bytes.Buffer)
	if _, e := io.Copy(b, reader); e != nil {
		log.Print("Could not read image", e)
		return nil
	}
	return readImageFromBuffer(b)
}

func strictReadImage(reader io.Reader) (*Avatar, error) {
	b := new(bytes.Buffer)
	if _, e := io.Copy(b, reader); e != nil {
		return nil, e
	}
	return readImageFromBuffer(b), nil
}
