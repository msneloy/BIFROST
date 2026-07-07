package gui

import (
	"bytes"
	"image"
	_ "image/jpeg"
)

func decodeJPEG(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}
