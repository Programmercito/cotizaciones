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

// GeneratePriceImage creates a PNG with the current buy and sell quotes and returns the file path.
func GeneratePriceImage(bid, purchase float64) (string, error) {
	const (
		w = 1400
		h = 800
	)

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	
	// Gradient-like background (dark navy)
	bgColor := color.RGBA{R: 11, G: 18, B: 29, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

	// Load fonts
	faceData, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return "", fmt.Errorf("error parsing font: %w", err)
	}

	labelFace, err := opentype.NewFace(faceData, &opentype.FaceOptions{
		Size:    60,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return "", fmt.Errorf("error creating label face: %w", err)
	}

	priceFace, err := opentype.NewFace(faceData, &opentype.FaceOptions{
		Size:    140,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return "", fmt.Errorf("error creating price face: %w", err)
	}

	smallFace, err := opentype.NewFace(faceData, &opentype.FaceOptions{
		Size:    40,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return "", fmt.Errorf("error creating small face: %w", err)
	}

	timestamp := time.Now().Format("02/01/2006 15:04:05")
	
	// Colors
	white := image.NewUniform(color.White)
	green := image.NewUniform(color.RGBA{R: 0, G: 192, B: 118, A: 255}) // Binance Green
	red := image.NewUniform(color.RGBA{R: 246, G: 70, B: 93, A: 255})   // Binance Red
	muted := image.NewUniform(color.RGBA{R: 132, G: 142, B: 156, A: 255})

	// Helper to draw centered text or left-aligned
	drawer := &font.Drawer{Dst: img, Src: white, Face: labelFace}

	// 1. Title
	drawer.Face = smallFace
	drawer.Src = muted
	drawer.Dot = fixed.P(60, 80)
	drawer.DrawString("BINANCE P2P · USDT / BOB")

	// 2. Venta (Sell) Section
	drawer.Face = labelFace
	drawer.Src = red
	drawer.Dot = fixed.P(100, 220)
	drawer.DrawString("VENTA")

	drawer.Face = priceFace
	drawer.Src = white
	ventaMsg := fmt.Sprintf("%.4f", bid)
	ventaWidth := drawer.MeasureString(ventaMsg).Round()
	drawer.Dot = fixed.P(100, 380)
	drawer.DrawString(ventaMsg)
	
	// Currency label
	drawer.Face = labelFace
	drawer.Src = muted
	drawer.Dot = fixed.P(130 + ventaWidth, 380)
	drawer.DrawString("BOB")

	// 3. Compra (Buy) Section
	drawer.Face = labelFace
	drawer.Src = green
	drawer.Dot = fixed.P(100, 500)
	drawer.DrawString("COMPRA")

	drawer.Face = priceFace
	drawer.Src = white
	compraMsg := fmt.Sprintf("%.4f", purchase)
	compraWidth := drawer.MeasureString(compraMsg).Round()
	drawer.Dot = fixed.P(100, 660)
	drawer.DrawString(compraMsg)

	// Currency label
	drawer.Face = labelFace
	drawer.Src = muted
	drawer.Dot = fixed.P(130 + compraWidth, 660)
	drawer.DrawString("BOB")

	// 4. Footer
	drawer.Face = smallFace
	drawer.Src = muted
	drawer.Dot = fixed.P(60, h-40)
	drawer.DrawString("Actualizado: " + timestamp)

	// Save to temp file
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

