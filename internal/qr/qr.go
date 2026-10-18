// Package qr renders the application's QR codes: every one the hall sees -- the score
// keepers' code, the landing page's, a code per mat, the info sheet's two -- with the
// Porta di Ferro gate in the middle (issue #143).
//
// The gate covers modules, so the code is made at the highest error correction, which
// rebuilds up to 30% of it; the gate and the white ring around it take under 6%. The
// gate is drawn here from web/public/favicon.svg's own geometry rather than carried as an
// image, so it is sharp at any size and there is no second copy of it to keep in step.
package qr

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"

	qrcode "github.com/skip2/go-qrcode"
)

// logoShare is the gate's side, with its ring, as a share of the code's side.
const logoShare = 0.22

// PNG renders payload as a size-by-size PNG with the gate in the middle.
func PNG(payload string, size int) ([]byte, error) {
	q, err := qrcode.New(payload, qrcode.Highest)
	if err != nil {
		return nil, err
	}
	code := q.Image(size)
	img := image.NewRGBA(code.Bounds())
	draw.Draw(img, img.Bounds(), code, code.Bounds().Min, draw.Src)

	side := int(float64(img.Bounds().Dx()) * logoShare)
	ring := max(2, side/14)
	cx, cy := img.Bounds().Dx()/2, img.Bounds().Dy()/2
	box := image.Rect(cx-side/2, cy-side/2, cx-side/2+side, cy-side/2+side)
	// A white ring, so the gate's dark ground does not run into the modules around it.
	draw.Draw(img, box, image.NewUniform(color.White), image.Point{}, draw.Src)
	gate(img, box.Inset(ring))

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

var (
	ground = color.RGBA{0x12, 0x14, 0x1a, 0xff}
	amber  = color.RGBA{0xf2, 0xb1, 0x34, 0xff}
)

// gate draws favicon.svg into r: a rounded dark square with the amber gate on it. Each
// pixel is sampled four by four, which is all the smoothing a printed code needs.
func gate(img *image.RGBA, r image.Rectangle) {
	const n = 4
	scale := 64 / float64(r.Dx())
	for py := r.Min.Y; py < r.Max.Y; py++ {
		for px := r.Min.X; px < r.Max.X; px++ {
			var inGround, inGate int
			for sy := 0; sy < n; sy++ {
				for sx := 0; sx < n; sx++ {
					x := (float64(px-r.Min.X) + (float64(sx)+0.5)/n) * scale
					y := (float64(py-r.Min.Y) + (float64(sy)+0.5)/n) * scale
					if !roundedSquare(x, y) {
						continue
					}
					inGround++
					if bars(x, y) {
						inGate++
					}
				}
			}
			if inGround == 0 {
				continue
			}
			under := img.RGBAAt(px, py)
			c := mix(under, ground, float64(inGround)/(n*n))
			c = mix(c, amber, float64(inGate)/float64(inGround))
			img.SetRGBA(px, py, c)
		}
	}
}

// roundedSquare is <rect width="64" height="64" rx="12">.
func roundedSquare(x, y float64) bool {
	const rx = 12
	dx := math.Max(math.Max(rx-x, x-(64-rx)), 0)
	dy := math.Max(math.Max(rx-y, y-(64-rx)), 0)
	return x >= 0 && x <= 64 && y >= 0 && y <= 64 && dx*dx+dy*dy <= rx*rx
}

// bars is the gate's three paths, with their stroke widths.
func bars(x, y float64) bool {
	// The arch: M14 54V26 a18 18 0 0 1 36 0v28, stroke 5.
	if y <= 26 {
		if d := math.Hypot(x-32, y-26); math.Abs(d-18) <= 2.5 {
			return true
		}
	} else if y <= 54 && (math.Abs(x-14) <= 2.5 || math.Abs(x-50) <= 2.5) {
		return true
	}
	// The bars: M24 54V18 M32 54V12 M40 54V18 M14 34h36 M14 44h36, stroke 3.5.
	const w = 1.75
	for _, b := range []struct{ x, top float64 }{{24, 18}, {32, 12}, {40, 18}} {
		if math.Abs(x-b.x) <= w && y >= b.top && y <= 54 {
			return true
		}
	}
	for _, rail := range []float64{34, 44} {
		if math.Abs(y-rail) <= w && x >= 14 && x <= 50 {
			return true
		}
	}
	// The feet: M22 54h4 M30 54h4 M38 54h4, stroke 3.5 with round caps.
	for _, left := range []float64{22, 30, 38} {
		cx := math.Max(left, math.Min(x, left+4))
		if math.Hypot(x-cx, y-54) <= w {
			return true
		}
	}
	return false
}

// mix is a over b by share a.
func mix(b, a color.RGBA, share float64) color.RGBA {
	m := func(p, q uint8) uint8 { return uint8(math.Round(float64(p)*(1-share) + float64(q)*share)) }
	return color.RGBA{m(b.R, a.R), m(b.G, a.G), m(b.B, a.B), 0xff}
}
