package file

import (
	"fmt"
	"image"
	_ "image/jpeg" // register JPEG decoder
	_ "image/png"  // register PNG decoder
	"os"
)

// LoadImage reads a PNG or JPG image from disk.
func LoadImage(filePath string) (image.Image, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image %s: %w", filePath, err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image %s: %w", filePath, err)
	}

	return img, nil
}

// ExtractSubImage extracts a rectangular region from an image.
func ExtractSubImage(img image.Image, rect image.Rectangle) image.Image {
	if sub, ok := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}); ok {
		return sub.SubImage(rect)
	}

	// Fallback: manual copy
	dst := image.NewRGBA(rect)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			dst.Set(x, y, img.At(x, y))
		}
	}
	return dst
}
