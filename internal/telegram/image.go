package telegram

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"time"

	"cotizaciones/internal/db"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// GeneratePriceImage creates a PNG with USDT, Official and Referential quotes (Buy & Sell).
func GeneratePriceImage(summary map[string]db.Cotizacion) (string, error) {
	const (
		w = 1200
		h = 1100
	)

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	
	bgColor := color.RGBA{R: 10, G: 15, B: 25, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

	faceData, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return "", err
	}

	titleFace, _ := opentype.NewFace(faceData, &opentype.FaceOptions{Size: 40, DPI: 72, Hinting: font.HintingFull})
	labelFace, _ := opentype.NewFace(faceData, &opentype.FaceOptions{Size: 45, DPI: 72, Hinting: font.HintingFull})
	priceFace, _ := opentype.NewFace(faceData, &opentype.FaceOptions{Size: 90, DPI: 72, Hinting: font.HintingFull})
	smallFace, _ := opentype.NewFace(faceData, &opentype.FaceOptions{Size: 30, DPI: 72, Hinting: font.HintingFull})

	timestamp := time.Now().Format("02/01/2006 15:04:05")
	
	white := image.NewUniform(color.White)
	green := image.NewUniform(color.RGBA{R: 0, G: 200, B: 120, A: 255})
	red := image.NewUniform(color.RGBA{R: 250, G: 60, B: 80, A: 255})
	blue := image.NewUniform(color.RGBA{R: 60, G: 150, B: 250, A: 255})
	muted := image.NewUniform(color.RGBA{R: 130, G: 140, B: 160, A: 255})

	drawer := &font.Drawer{Dst: img, Src: white, Face: titleFace}

	drawQuoteRow := func(y int, title string, c db.Cotizacion, isPrecision bool) {
		// Section Title
		drawer.Face = labelFace
		drawer.Src = blue
		drawer.Dot = fixed.P(60, y)
		drawer.DrawString(title)

		// Venta
		drawer.Face = smallFace
		drawer.Src = red
		drawer.Dot = fixed.P(80, y+60)
		drawer.DrawString("VENTA")

		drawer.Face = priceFace
		drawer.Src = white
		vMsg := fmt.Sprintf("%.2f", c.Cotizacion)
		if isPrecision { vMsg = fmt.Sprintf("%.4f", c.Cotizacion) }
		drawer.Dot = fixed.P(80, y+160)
		drawer.DrawString(vMsg)

		// Compra
		drawer.Face = smallFace
		drawer.Src = green
		drawer.Dot = fixed.P(650, y+60)
		drawer.DrawString("COMPRA")

		drawer.Face = priceFace
		drawer.Src = white
		cMsg := fmt.Sprintf("%.2f", c.Purchase)
		if isPrecision { cMsg = fmt.Sprintf("%.4f", c.Purchase) }
		drawer.Dot = fixed.P(650, y+160)
		drawer.DrawString(cMsg)
		
		// Separator line
		draw.Draw(img, image.Rect(60, y+200, w-60, y+202), &image.Uniform{C: color.RGBA{40, 50, 70, 255}}, image.Point{}, draw.Src)
	}

	// Header
	drawer.Face = smallFace
	drawer.Src = muted
	drawer.Dot = fixed.P(60, 60)
	drawer.DrawString("RESUMEN DE COTIZACIONES")

	// 1. USDT
	drawQuoteRow(150, "USDT - BINANCE P2P", summary["USDT"], true)
	
	// 2. Oficial
	drawQuoteRow(450, "USD OFICIAL - BANCO CENTRAL", summary["usd oficial"], false)

	// 3. Referencial
	drawQuoteRow(750, "USD REFERENCIAL - BANCO CENTRAL", summary["usd referencial"], false)

	// Footer
	drawer.Face = smallFace
	drawer.Src = muted
	drawer.Dot = fixed.P(60, h-40)
	drawer.DrawString("Actualizado: " + timestamp)

	path, err := os.CreateTemp("", "cotizacion-*.png")
	if err != nil {
		return "", err
	}
	defer path.Close()
	if err := png.Encode(path, img); err != nil {
		return "", err
	}
	return path.Name(), nil
}

