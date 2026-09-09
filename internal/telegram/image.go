package telegram

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cotizaciones/internal/db"

	qrcode "github.com/skip2/go-qrcode"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
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
	trendFace font.Face
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
	drawVerticalGradient(img, color.RGBA{R: 20, G: 31, B: 48, A: 255}, color.RGBA{R: 6, G: 10, B: 17, A: 255})

	faceData, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return nil, err
	}

	titleFace, _ := opentype.NewFace(faceData, &opentype.FaceOptions{Size: 38, DPI: 72, Hinting: font.HintingFull})
	labelFace, _ := opentype.NewFace(faceData, &opentype.FaceOptions{Size: 42, DPI: 72, Hinting: font.HintingFull})
	priceFace, _ := opentype.NewFace(faceData, &opentype.FaceOptions{Size: 168, DPI: 72, Hinting: font.HintingFull})
	trendFace, _ := opentype.NewFace(faceData, &opentype.FaceOptions{Size: 52, DPI: 72, Hinting: font.HintingFull})
	smallFace, _ := opentype.NewFace(faceData, &opentype.FaceOptions{Size: 26, DPI: 72, Hinting: font.HintingFull})
	tinyFace, _ := opentype.NewFace(faceData, &opentype.FaceOptions{Size: 22, DPI: 72, Hinting: font.HintingFull})

	drawer := &font.Drawer{Dst: img, Src: image.NewUniform(color.White), Face: titleFace}

	return &imageBuilder{
		img: img, drawer: drawer, w: w, h: h,
		titleFace: titleFace, labelFace: labelFace, priceFace: priceFace, trendFace: trendFace,
		smallFace: smallFace, tinyFace: tinyFace,
		white: image.NewUniform(color.White),
		green: image.NewUniform(color.RGBA{R: 0, G: 200, B: 120, A: 255}),
		red:   image.NewUniform(color.RGBA{R: 250, G: 60, B: 80, A: 255}),
		blue:  image.NewUniform(color.RGBA{R: 60, G: 150, B: 250, A: 255}),
		muted: image.NewUniform(color.RGBA{R: 130, G: 140, B: 160, A: 255}),
		gold:  image.NewUniform(color.RGBA{R: 255, G: 200, B: 60, A: 255}),
	}, nil
}

func drawVerticalGradient(img *image.RGBA, top, bottom color.RGBA) {
	height := img.Bounds().Dy() - 1
	for y := 0; y <= height; y++ {
		progress := float64(y) / float64(height)
		background := color.RGBA{
			R: uint8(float64(top.R) + (float64(bottom.R)-float64(top.R))*progress),
			G: uint8(float64(top.G) + (float64(bottom.G)-float64(top.G))*progress),
			B: uint8(float64(top.B) + (float64(bottom.B)-float64(top.B))*progress),
			A: 255,
		}
		draw.Draw(img, image.Rect(0, y, img.Bounds().Dx(), y+1), &image.Uniform{C: background}, image.Point{}, draw.Src)
	}
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

const (
	quoteRowHeight      = 330
	trendQuoteRowHeight = 440
)

// trendInfo describes a previous-day baseline comparison for a quote row.
// A zero value (exists == false) means no trend badge is rendered.
type trendInfo struct {
	exists bool
	up     bool
	equal  bool
	diff   float64
}

// buildTrend computes trend info comparing current against a baseline that
// may or may not exist.
func buildTrend(current float64, baseline db.Baseline) trendInfo {
	if !baseline.Exists {
		return trendInfo{}
	}
	diff := current - baseline.Value
	return trendInfo{
		exists: true,
		up:     diff > 0,
		equal:  math.Abs(diff) < 0.005,
		diff:   diff,
	}
}

// drawTrendArrow renders a smooth SVG trend icon directly into the PNG.
func (b *imageBuilder) drawTrendArrow(x, y int, up bool, stroke string) {
	transform := ""
	if !up {
		transform = ` transform="translate(0 100) scale(1 -1)"`
	}
	svg := fmt.Sprintf(`<svg viewBox="0 0 120 100" xmlns="http://www.w3.org/2000/svg"><g%s fill="none" stroke="%s" stroke-linecap="round" stroke-linejoin="round" stroke-width="12"><path d="M14 78 L42 50 L60 68 L104 24"/><path d="M77 24 H104 V51"/></g><g%s fill="%s"><circle cx="14" cy="78" r="7"/><circle cx="42" cy="50" r="7"/><circle cx="60" cy="68" r="7"/></g></svg>`, transform, stroke, transform, stroke)
	icon, err := oksvg.ReadIconStream(strings.NewReader(svg), oksvg.StrictErrorMode)
	if err != nil {
		return
	}
	icon.SetTarget(float64(x), float64(y), 96, 80)
	scanner := rasterx.NewScannerGV(b.w, b.h, b.img, b.img.Bounds())
	icon.Draw(rasterx.NewDasher(b.w, b.h, scanner), 1)
}

// drawTrendBadge renders a prominent arrow and amount-only delta centered
// between sale and purchase prices when trend data exists.
func (b *imageBuilder) drawTrendBadge(y int, t trendInfo) {
	if !t.exists {
		return
	}

	const arrowX = 442
	const amountX = 564
	col := b.muted
	if !t.equal {
		stroke := "#FA3C50"
		if t.up {
			col = b.green
			stroke = "#00C878"
		} else {
			col = b.red
		}
		b.drawTrendArrow(arrowX, y+236, t.up, stroke)
	} else {
		draw.Draw(b.img, image.Rect(arrowX, y+270, arrowX+72, y+277), col, image.Point{}, draw.Src)
	}

	b.drawer.Face = b.trendFace
	b.drawer.Src = col
	change := "0.00"
	if t.equal {
		change = "0.00"
	} else {
		change = fmt.Sprintf("%+.2f", t.diff)
	}
	b.drawer.Dot = fixed.P(amountX, y+294)
	b.drawer.DrawString(change)
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
	drawQR("https://t.me/usdbolivia", qrX, qr1Top)

	qr2TitleY := qr1Top + qrSize + 20 + 22
	qr2Top := qr1Top + qrSize + 20 + 26
	b.drawer.Face = b.tinyFace
	b.drawer.Src = b.muted
	b.drawer.Dot = fixed.P(qrX, qr2TitleY)
	b.drawer.DrawString("Website")
	drawQR("https://dolarbolivia.org", qrX, qr2Top)
}

func (b *imageBuilder) drawQuoteRow(y int, title string, c db.Cotizacion, trend trendInfo) int {
	b.drawer.Face = b.labelFace
	b.drawer.Src = b.blue
	b.drawer.Dot = fixed.P(60, y)
	b.drawer.DrawString(title)

	b.drawer.Face = b.tinyFace
	b.drawer.Src = b.muted
	b.drawer.Dot = fixed.P(62, y+32)
	b.drawer.DrawString("Actualizado: " + formatDatetime(c.Datetime))

	b.drawer.Face = b.smallFace
	b.drawer.Src = b.red
	b.drawer.Dot = fixed.P(80, y+70)
	b.drawer.DrawString("VENTA")

	priceY := y + 212

	b.drawer.Face = b.priceFace
	b.drawer.Src = b.white
	vMsg := fmt.Sprintf("%.2f", c.Cotizacion)
	b.drawer.Dot = fixed.P(80, priceY)
	b.drawer.DrawString(vMsg)

	b.drawer.Face = b.smallFace
	b.drawer.Src = b.green
	b.drawer.Dot = fixed.P(535, y+70)
	b.drawer.DrawString("COMPRA")

	b.drawer.Face = b.priceFace
	b.drawer.Src = b.white
	cMsg := fmt.Sprintf("%.2f", c.Purchase)
	b.drawer.Dot = fixed.P(535, priceY)
	b.drawer.DrawString(cMsg)

	b.drawTrendBadge(y, trend)
	if trend.exists {
		return y + trendQuoteRowHeight
	}

	return y + quoteRowHeight
}

func (b *imageBuilder) drawSingleRow(y int, title, valueLabel string, value float64, fmtStr string, c db.Cotizacion) int {
	b.drawer.Face = b.labelFace
	b.drawer.Src = b.blue
	b.drawer.Dot = fixed.P(60, y)
	b.drawer.DrawString(title)

	b.drawer.Face = b.tinyFace
	b.drawer.Src = b.muted
	b.drawer.Dot = fixed.P(62, y+32)
	b.drawer.DrawString("Actualizado: " + formatDatetime(c.Datetime))

	b.drawer.Face = b.smallFace
	b.drawer.Src = b.gold
	b.drawer.Dot = fixed.P(80, y+70)
	b.drawer.DrawString(valueLabel)

	b.drawer.Face = b.priceFace
	b.drawer.Src = b.white
	b.drawer.Dot = fixed.P(80, y+212)
	b.drawer.DrawString(fmt.Sprintf(fmtStr, value))

	return y + 330
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

// GenerateUSDImage creates the production PNG with USDT, Official and optional
// Referential quotes.
func GenerateUSDImage(summary map[string]db.Cotizacion, showReferencial bool, baselines db.USDBaselines) (string, error) {
	return GenerateUSDImageAt("/opt/osbo/cotiza/usdt.png", summary, showReferencial, baselines)
}

// GenerateUSDImageAt creates a USD image at outPath. It is useful for local
// previews while keeping the production output path unchanged.
func GenerateUSDImageAt(outPath string, summary map[string]db.Cotizacion, showReferencial bool, baselines db.USDBaselines) (string, error) {
	const topMargin = 100
	const bottomMargin = 120
	usdtTrend := buildTrend(summary["USDT"].Cotizacion, baselines.USDT)
	oficialTrend := buildTrend(summary["usd oficial"].Cotizacion, baselines.UsdOficial)
	h := topMargin + bottomMargin
	if usdtTrend.exists {
		h += trendQuoteRowHeight
	} else {
		h += quoteRowHeight
	}
	if oficialTrend.exists {
		h += trendQuoteRowHeight
	} else {
		h += quoteRowHeight
	}
	if showReferencial {
		h += quoteRowHeight
	}
	b, err := newImageBuilder(1200, h)
	if err != nil {
		return "", err
	}

	b.drawHeader()
	b.drawQRs()

	y := topMargin
	y = b.drawQuoteRow(y, "USDT – BINANCE P2P"+destSuffix(summary["USDT"]), summary["USDT"], usdtTrend)
	y = b.drawQuoteRow(y, "USD OFICIAL – BCB"+destSuffix(summary["usd oficial"]), summary["usd oficial"], oficialTrend)
	if showReferencial {
		y = b.drawQuoteRow(y, "USD REFERENCIAL – BCB"+destSuffix(summary["usd referencial"]), summary["usd referencial"], trendInfo{})
	}

	b.drawFooter()
	if err := b.saveTo(outPath); err != nil {
		return "", err
	}
	return outPath, nil
}

// GenerateRestoImage creates a PNG with Euro, Oro, Plata and UFV quotes.
func GenerateRestoImage(summary map[string]db.Cotizacion) (string, error) {
	const outPath = "/opt/osbo/cotiza/resto.png"
	b, err := newImageBuilder(1200, 1500)
	if err != nil {
		return "", err
	}

	b.drawHeader()
	b.drawQRs()

	y := 100
	y = b.drawQuoteRow(y, "EURO – BCB"+destSuffix(summary["eur"]), summary["eur"], trendInfo{})
	y = b.drawSingleRow(y, "ORO (TROY OZ) – BCB"+destSuffix(summary["oro"]), "PRECIO", summary["oro"].Cotizacion, "%.2f", summary["oro"])
	y = b.drawSingleRow(y, "PLATA (TROY OZ) – BCB"+destSuffix(summary["plata"]), "PRECIO", summary["plata"].Cotizacion, "%.2f", summary["plata"])
	y = b.drawSingleRow(y, "UFV – BCB"+destSuffix(summary["ufv"]), "VALOR", summary["ufv"].Cotizacion, "%.2f", summary["ufv"])

	b.drawFooter()
	if err := b.saveTo(outPath); err != nil {
		return "", err
	}
	return outPath, nil
}
