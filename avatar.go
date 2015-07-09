package main

import (
	"image"
	"bytes"	
	"image/gif"
	"image/jpeg"
	"image/png"
	"log"
	"io"
	"github.com/nfnt/resize"
	"github.com/oliamb/cutter"
)

func min(x, y int) int {
	if x <= y {
		return x
	}
	return y
}

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

// alterings the avatar instance!
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
func scale(avatar *Avatar, size int) error {
	img, format, err := avatar2Image(avatar)
	if err != nil {
		return err
	}
	actualSize := img.Bounds().Dx() // assume square
	if size == actualSize {
		return nil
	}
	log.Printf("Resizing img from %vx%v to %vx%v", actualSize, actualSize, size, size)
	resized := resize.Resize(uint(size), uint(size), img, resize.Bicubic)
	image2Avatar(avatar, resized, format)
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
  			Mode: cutter.Centered})
		if err != nil {
			return err
		}
	}
	if size <= maxSize {
		return nil
	}
	log.Printf("Resizing img from %vx%v to %vx%v", size, size, maxSize, maxSize)
	resized := resize.Resize(uint(maxSize), uint(maxSize), img, resize.Bicubic)
	image2Avatar(avatar, resized, format)
	return nil
}

func strictReadImage(reader io.Reader) (*Avatar, error) {
	b := new(bytes.Buffer)
	if _, e := io.Copy(b, reader); e != nil {
		return nil, e
	}
	return &Avatar{size: -1, data: b.Bytes()}, nil
}

func readImage(reader io.Reader) *Avatar {
	avatar, err := strictReadImage(reader)
	if err != nil {
		log.Printf("Could not read image", err)
		return nil
	}
	return avatar
}
