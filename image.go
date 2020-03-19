package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	imagelib "image"
	"image/jpeg"
	"image/png"

	imgresize "github.com/nfnt/resize"
)

type Img struct {
	img    imagelib.Image
	format string
}

func inlineImage(o interface{}) template.URL {
	var (
		data     []byte
		mimetype string
	)
	switch img := o.(type) {
	case *Img:
		buf := bytes.NewBuffer(nil)
		switch img.format {
		case "jpeg":
			mimetype = "image/jpeg"

			// Calculate size on which we base quality.
			// We limit size to between 50 and 1600, and quality between 95 and 65.
			width := img.img.Bounds().Dx()
			height := img.img.Bounds().Dy()
			size := width
			if height < size {
				size = height
			}
			if size > 1600 {
				size = 1600
			}
			if size < 50 {
				size = 50
			}

			quality := 65 + (95-65)*(1600-50-size)/(1600-50)
			options := &jpeg.Options{Quality: quality}
			err := jpeg.Encode(buf, img.img, options)
			httpCheck(err)
		case "png":
			mimetype = "image/png"
			err := png.Encode(buf, img.img)
			httpCheck(err)
		default:
			abortUserError("Unsupported image format for inlining.")
		}
		data = buf.Bytes()
	case *image:
		var err error
		data, err = img.Data()
		httpCheck(err)
		mimetype = img.Mimetype
	default:
		abortUserError(fmt.Sprintf("Unexpected input %T to inlineImage.", o))
	}

	s := base64.StdEncoding.EncodeToString(data)
	return template.URL(fmt.Sprintf("data:%s;base64,", mimetype) + s)
}

func thumbnail(width, height uint, img *Img) *Img {
	return &Img{
		img:    imgresize.Thumbnail(width, height, img.img, imgresize.Lanczos3),
		format: img.format,
	}
}

func resize(width, height uint, img *Img) *Img {
	return &Img{
		img:    imgresize.Resize(width, height, img.img, imgresize.Lanczos3),
		format: img.format,
	}
}
