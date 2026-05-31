package telegram

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"cotizaciones/internal/db"

	qrcode "github.com/skip2/go-qrcode"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type imageBuilder struct {
	img    *image.RGBA
	drawer *font.Drawer
	w, h   int

	titleFace font.Face
	labelFace font.Face
	priceFace font.Face
	smallFace font.Face
	tinyFace  font.Face

	white *image.Uniform
	green *image.Uniform
	red   *image.Uniform
	blue  *image.Uniform
	muted *image.Uniform
	gold  *image.Uniform
}

func newImageBuilder(w, h int) (*imageBuilder, error) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	bgColor := color.RGBA{R: 10, G: 15, B: 25, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

	faceData, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return nil, err
	}

	titleFace, _ := opentype.NewFace(faceData, &opentype.FaceOptions{Size: 38, DPI: 72, Hinting: font.HintingFull})
	labelFace, _ := opentype.NewFace(faceData, &opentype.FaceOptions{Size: 42, DPI: 72, Hinting: font.HintingFull})
	priceFace, _ := opentype.NewFace(faceData, &opentype.FaceOptions{Size: 84, DPI: 72, Hinting: font.HintingFull})
	smallFace, _ := opentype.NewFace(faceData, &opentype.FaceOptions{Size: 26, DPI: 72, Hinting: font.HintingFull})
	tinyFace, _ := opentype.NewFace(faceData, &opentype.FaceOptions{Size: 22, DPI: 72, Hinting: font.HintingFull})

	drawer := &font.Drawer{Dst: img, Src: image.NewUniform(color.White), Face: titleFace}

	return &imageBuilder{
		img: img, drawer: drawer, w: w, h: h,
		titleFace: titleFace, labelFace: labelFace, priceFace: priceFace,
		smallFace: smallFace, tinyFace: tinyFace,
		white: image.NewUniform(color.White),
		green: image.NewUniform(color.RGBA{R: 0, G: 200, B: 120, A: 255}),
		red:   image.NewUniform(color.RGBA{R: 250, G: 60, B: 80, A: 255}),
		blue:  image.NewUniform(color.RGBA{R: 60, G: 150, B: 250, A: 255}),
		muted: image.NewUniform(color.RGBA{R: 130, G: 140, B: 160, A: 255}),
		gold:  image.NewUniform(color.RGBA{R: 255, G: 200, B: 60, A: 255}),
	}, nil
}

func (b *imageBuilder) drawHeader() {
	b.drawer.Face = b.smallFace
	b.drawer.Src = b.gold
	b.drawer.Dot = fixed.P(60, 38)
	b.drawer.DrawString("COTIZACIONES")
}

func (b *imageBuilder) drawFooter() {
	b.drawer.Face = b.tinyFace
	b.drawer.Src = b.muted
	b.drawer.Dot = fixed.P(60, b.h-18)
	b.drawer.DrawString("Generado: " + time.Now().Format(db.DisplayTimeFmt))
}

func (b *imageBuilder) drawQRs() {
	const qrSize = 230
	const qrMargin = 12
	drawQR := func(url string, xRight, yTop int) {
		pngBytes, err2 := qrcode.Encode(url, qrcode.Medium, qrSize)
		if err2 != nil {
			return
		}
		qrImg, err2 := png.Decode(bytes.NewReader(pngBytes))
		if err2 != nil {
			return
		}
		dstRect := image.Rect(xRight, yTop, xRight+qrSize, yTop+qrSize)
		draw.Draw(b.img, dstRect, qrImg, image.Point{}, draw.Src)
	}
	qrX := b.w - qrSize - qrMargin

	qr1TitleY := qrMargin + 22
	qr1Top := qrMargin + 26
	b.drawer.Face = b.tinyFace
	b.drawer.Src = b.muted
	b.drawer.Dot = fixed.P(qrX, qr1TitleY)
	b.drawer.DrawString("Telegram")
	drawQR("https://t.me/usdtbolivia", qrX, qr1Top)

	qr2TitleY := qr1Top + qrSize + 20 + 22
	qr2Top := qr1Top + qrSize + 20 + 26
	b.drawer.Face = b.tinyFace
	b.drawer.Src = b.muted
	b.drawer.Dot = fixed.P(qrX, qr2TitleY)
	b.drawer.DrawString("Website")
	drawQR("https://dolarbolivia.org", qrX, qr2Top)
}

func (b *imageBuilder) drawQuoteRow(y int, title string, c db.Cotizacion, isPrecision bool) int {
	b.drawer.Face = b.labelFace
	b.drawer.Src = b.blue
	b.drawer.Dot = fixed.P(60, y)
	b.drawer.DrawString(title)

	b.drawer.Face = b.tinyFace
	b.drawer.Src = b.muted
	b.drawer.Dot = fixed.P(62, y+28)
	b.drawer.DrawString("Actualizado: " + formatDatetime(c.Datetime))

	b.drawer.Face = b.smallFace
	b.drawer.Src = b.red
	b.drawer.Dot = fixed.P(80, y+80)
	b.drawer.DrawString("VENTA")

	b.drawer.Face = b.priceFace
	b.drawer.Src = b.white
	vMsg := fmt.Sprintf("%.2f", c.Cotizacion)
	if isPrecision {
		vMsg = fmt.Sprintf("%.4f", c.Cotizacion)
	}
	b.drawer.Dot = fixed.P(80, y+175)
	b.drawer.DrawString(vMsg)

	b.drawer.Face = b.smallFace
	b.drawer.Src = b.green
	b.drawer.Dot = fixed.P(650, y+80)
	b.drawer.DrawString("COMPRA")

	b.drawer.Face = b.priceFace
	b.drawer.Src = b.white
	cMsg := fmt.Sprintf("%.2f", c.Purchase)
	if isPrecision {
		cMsg = fmt.Sprintf("%.4f", c.Purchase)
	}
	b.drawer.Dot = fixed.P(650, y+175)
	b.drawer.DrawString(cMsg)

	draw.Draw(b.img, image.Rect(60, y+205, b.w-60, y+207), &image.Uniform{C: color.RGBA{40, 50, 70, 255}}, image.Point{}, draw.Src)

	return y + 260
}

func (b *imageBuilder) drawSingleRow(y int, title, valueLabel string, value float64, fmtStr string, c db.Cotizacion) int {
	b.drawer.Face = b.labelFace
	b.drawer.Src = b.blue
	b.drawer.Dot = fixed.P(60, y)
	b.drawer.DrawString(title)

	b.drawer.Face = b.tinyFace
	b.drawer.Src = b.muted
	b.drawer.Dot = fixed.P(62, y+28)
	b.drawer.DrawString("Actualizado: " + formatDatetime(c.Datetime))

	b.drawer.Face = b.smallFace
	b.drawer.Src = b.gold
	b.drawer.Dot = fixed.P(80, y+80)
	b.drawer.DrawString(valueLabel)

	b.drawer.Face = b.priceFace
	b.drawer.Src = b.white
	b.drawer.Dot = fixed.P(80, y+175)
	b.drawer.DrawString(fmt.Sprintf(fmtStr, value))

	draw.Draw(b.img, image.Rect(60, y+205, b.w-60, y+207), &image.Uniform{C: color.RGBA{40, 50, 70, 255}}, image.Point{}, draw.Src)

	return y + 260
}

func (b *imageBuilder) saveTo(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, b.img)
}

func formatDatetime(dt string) string {
	layouts := []string{db.TimeFmt, "2006-01-02 15:04", "2006-01-02"}
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

func destSuffix(c db.Cotizacion) string {
	if c.MonedaDest == "" {
		return ""
	}
	return " (" + c.MonedaDest + ")"
}

// GenerateUSDImage creates a PNG with USDT, Official and Referential quotes.
func GenerateUSDImage(summary map[string]db.Cotizacion) (string, error) {
	const outPath = "/opt/osbo/cotiza/usdt.png"
	b, err := newImageBuilder(1200, 1050)
	if err != nil {
		return "", err
	}

	b.drawHeader()
	b.drawQRs()

	y := 100
	y = b.drawQuoteRow(y, "USDT – BINANCE P2P"+destSuffix(summary["USDT"]), summary["USDT"], true)
	y = b.drawQuoteRow(y, "USD OFICIAL – BCB"+destSuffix(summary["usd oficial"]), summary["usd oficial"], false)
	y = b.drawQuoteRow(y, "USD REFERENCIAL – BCB"+destSuffix(summary["usd referencial"]), summary["usd referencial"], false)

	b.drawFooter()
	if err := b.saveTo(outPath); err != nil {
		return "", err
	}
	return outPath, nil
}

// GenerateRestoImage creates a PNG with Euro, Oro, Plata and UFV quotes.
func GenerateRestoImage(summary map[string]db.Cotizacion) (string, error) {
	const outPath = "/opt/osbo/cotiza/resto.png"
	b, err := newImageBuilder(1200, 1350)
	if err != nil {
		return "", err
	}

	b.drawHeader()
	b.drawQRs()

	y := 100
	y = b.drawQuoteRow(y, "EURO – BCB"+destSuffix(summary["eur"]), summary["eur"], false)
	y = b.drawSingleRow(y, "ORO (TROY OZ) – BCB"+destSuffix(summary["oro"]), "PRECIO", summary["oro"].Cotizacion, "%.2f", summary["oro"])
	y = b.drawSingleRow(y, "PLATA (TROY OZ) – BCB"+destSuffix(summary["plata"]), "PRECIO", summary["plata"].Cotizacion, "%.2f", summary["plata"])
	y = b.drawSingleRow(y, "UFV – BCB"+destSuffix(summary["ufv"]), "VALOR", summary["ufv"].Cotizacion, "%.5f", summary["ufv"])

	b.drawFooter()
	if err := b.saveTo(outPath); err != nil {
		return "", err
	}
	return outPath, nil
}
