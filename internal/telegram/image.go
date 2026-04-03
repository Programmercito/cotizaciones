package telegram

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// GeneratePriceImage creates a PNG with the current cotización en texto y devuelve la ruta del archivo.
func GeneratePriceImage(bid float64) (string, error) {
	const (
		n = 1024
		h = 512
	)

	img := image.NewRGBA(image.Rect(0, 0, n, h))
	bgColor := color.RGBA{R: 18, G: 28, B: 40, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

	faceData, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return "", fmt.Errorf("error parsing font: %w", err)
	}

	bigFace, err := opentype.NewFace(faceData, &opentype.FaceOptions{Size: 96, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return "", fmt.Errorf("error creating big face: %w", err)
	}
	smallFace, err := opentype.NewFace(faceData, &opentype.FaceOptions{Size: 36, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return "", fmt.Errorf("error creating small face: %w", err)
	}

	message := fmt.Sprintf("1 USDT = %.4f BOB", bid)
	timestamp := time.Now().Format("02/01/2006 15:04:05")

	drawer := &font.Drawer{Dst: img, Src: image.NewUniform(color.White), Face: bigFace}
	width := drawer.MeasureString(message).Round()
	drawer.Dot = fixed.P((n-width)/2, h/2-10)
	drawer.DrawString(message)

	drawer.Face = smallFace
	drawer.Dot = fixed.P(30, h-46)
	drawer.DrawString("Actualizado: " + timestamp)

	path, err := os.CreateTemp("", "cotizacion-*.png")
	if err != nil {
		return "", fmt.Errorf("error creando archivo temporal: %w", err)
	}
	defer path.Close()

	if err := png.Encode(path, img); err != nil {
		return "", fmt.Errorf("error codificando PNG: %w", err)
	}

	return path.Name(), nil
}
