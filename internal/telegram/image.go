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

// GeneratePriceImage creates a PNG with USDT, Official, Referential, Euro, Oro, Plata and UFV quotes.
func GeneratePriceImage(summary map[string]db.Cotizacion) (string, error) {
	const (
		w = 1200
		h = 2050
	)

	img := image.NewRGBA(image.Rect(0, 0, w, h))

	bgColor := color.RGBA{R: 10, G: 15, B: 25, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

	faceData, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return "", err
	}

	titleFace, _ := opentype.NewFace(faceData, &opentype.FaceOptions{Size: 38, DPI: 72, Hinting: font.HintingFull})
	labelFace, _ := opentype.NewFace(faceData, &opentype.FaceOptions{Size: 42, DPI: 72, Hinting: font.HintingFull})
	priceFace, _ := opentype.NewFace(faceData, &opentype.FaceOptions{Size: 84, DPI: 72, Hinting: font.HintingFull})
	smallFace, _ := opentype.NewFace(faceData, &opentype.FaceOptions{Size: 26, DPI: 72, Hinting: font.HintingFull})
	tinyFace, _ := opentype.NewFace(faceData, &opentype.FaceOptions{Size: 22, DPI: 72, Hinting: font.HintingFull})

	white := image.NewUniform(color.White)
	green := image.NewUniform(color.RGBA{R: 0, G: 200, B: 120, A: 255})
	red := image.NewUniform(color.RGBA{R: 250, G: 60, B: 80, A: 255})
	blue := image.NewUniform(color.RGBA{R: 60, G: 150, B: 250, A: 255})
	muted := image.NewUniform(color.RGBA{R: 130, G: 140, B: 160, A: 255})
	gold := image.NewUniform(color.RGBA{R: 255, G: 200, B: 60, A: 255})

	drawer := &font.Drawer{Dst: img, Src: white, Face: titleFace}

	// formatDatetime: usa las constantes del paquete db, sin strings hardcodeados
	// Si el valor incluye hora, muestra segundos; si es solo fecha, muestra solo la fecha.
	formatDatetime := func(dt string) string {
		layouts := []string{
			db.TimeFmt,
			"2006-01-02 15:04",
			"2006-01-02",
		}
		for _, layout := range layouts {
			t, err := time.Parse(layout, dt)
			if err != nil {
				continue
			}
			if layout == db.TimeFmt || layout == "2006-01-02 15:04" {
				return t.Format(db.DisplayTimeFmt)
			}
			return t.Format(db.DisplayDateFmt)
		}
		return dt
	}

	drawQuoteRow := func(y int, title string, c db.Cotizacion, isPrecision bool) {
		// Section title
		drawer.Face = labelFace
		drawer.Src = blue
		drawer.Dot = fixed.P(60, y)
		drawer.DrawString(title)

		// Actualizado label (hora de la DB para esta moneda)
		drawer.Face = tinyFace
		drawer.Src = muted
		drawer.Dot = fixed.P(62, y+28)
		drawer.DrawString("Actualizado: " + formatDatetime(c.Datetime))

		// VENTA label + price
		drawer.Face = smallFace
		drawer.Src = red
		drawer.Dot = fixed.P(80, y+80)
		drawer.DrawString("VENTA")

		drawer.Face = priceFace
		drawer.Src = white
		vMsg := fmt.Sprintf("%.2f", c.Cotizacion)
		if isPrecision {
			vMsg = fmt.Sprintf("%.4f", c.Cotizacion)
		}
		drawer.Dot = fixed.P(80, y+175)
		drawer.DrawString(vMsg)

		// COMPRA label + price
		drawer.Face = smallFace
		drawer.Src = green
		drawer.Dot = fixed.P(650, y+80)
		drawer.DrawString("COMPRA")

		drawer.Face = priceFace
		drawer.Src = white
		cMsg := fmt.Sprintf("%.2f", c.Purchase)
		if isPrecision {
			cMsg = fmt.Sprintf("%.4f", c.Purchase)
		}
		drawer.Dot = fixed.P(650, y+175)
		drawer.DrawString(cMsg)

		// Separator line
		draw.Draw(img, image.Rect(60, y+205, w-60, y+207), &image.Uniform{C: color.RGBA{40, 50, 70, 255}}, image.Point{}, draw.Src)
	}

	// drawSingleRow draws a row with a single value (no buy/sell pair)
	drawSingleRow := func(y int, title, valueLabel string, value float64, fmtStr string, c db.Cotizacion) {
		drawer.Face = labelFace
		drawer.Src = blue
		drawer.Dot = fixed.P(60, y)
		drawer.DrawString(title)

		drawer.Face = tinyFace
		drawer.Src = muted
		drawer.Dot = fixed.P(62, y+28)
		drawer.DrawString("Actualizado: " + formatDatetime(c.Datetime))

		drawer.Face = smallFace
		drawer.Src = gold
		drawer.Dot = fixed.P(80, y+80)
		drawer.DrawString(valueLabel)

		drawer.Face = priceFace
		drawer.Src = white
		drawer.Dot = fixed.P(80, y+175)
		drawer.DrawString(fmt.Sprintf(fmtStr, value))

		draw.Draw(img, image.Rect(60, y+205, w-60, y+207), &image.Uniform{C: color.RGBA{40, 50, 70, 255}}, image.Point{}, draw.Src)
	}

	// Header
	drawer.Face = smallFace
	drawer.Src = gold
	drawer.Dot = fixed.P(60, 38)
	drawer.DrawString("COTIZACIONES · BOB")

	// 1. USDT         (y=100)
	drawQuoteRow(100, "USDT – BINANCE P2P", summary["USDT"], true)

	// 2. Oficial      (y=400)
	drawQuoteRow(400, "USD OFICIAL – BCB", summary["usd oficial"], false)

	// 3. Referencial  (y=700)
	drawQuoteRow(700, "USD REFERENCIAL – BCB", summary["usd referencial"], false)

	// 4. Euro         (y=1000)
	drawQuoteRow(1000, "EURO – BCB", summary["euro"], false)

	// 5. Oro          (y=1300)
	drawSingleRow(1300, "ORO (TROY OZ) – BCB", "PRECIO", summary["oro"].Cotizacion, "%.2f", summary["oro"])

	// 6. Plata        (y=1540)
	drawSingleRow(1540, "PLATA (TROY OZ) – BCB", "PRECIO", summary["plata"].Cotizacion, "%.2f", summary["plata"])

	// 7. UFV          (y=1780)
	drawSingleRow(1780, "UFV – BCB", "VALOR", summary["ufv"].Cotizacion, "%.5f", summary["ufv"])

	// Footer global (hora de generación de la imagen)
	drawer.Face = tinyFace
	drawer.Src = muted
	drawer.Dot = fixed.P(60, h-18)
	drawer.DrawString("Generado: " + time.Now().Format(db.DisplayTimeFmt))

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
