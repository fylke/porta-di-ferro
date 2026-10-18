package qr

import (
	"bytes"
	"image/color"
	"image/png"
	"testing"
)

func TestPNGPutsTheGateInTheMiddle(t *testing.T) {
	b, err := PNG("http://192.168.1.20:8080/score", 512)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if got := img.Bounds().Dx(); got != 512 {
		t.Fatalf("the code should be 512 wide, got %d", got)
	}
	rgba := func(x, y int) color.RGBA {
		r, g, b, _ := img.At(x, y).RGBA()
		return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 0xff}
	}
	// The middle bar of the gate runs down the centre, amber.
	if c := rgba(256, 256); c != amber {
		t.Errorf("the centre should be the gate's amber, got %v", c)
	}
	// The gate's square, as PNG lays it out, and a point in it by favicon.svg's numbers.
	width := img.Bounds().Dx()
	side := int(float64(width) * logoShare)
	ring := max(2, side/14)
	left := width/2 - side/2
	at := func(x, y float64) (int, int) {
		inner := float64(side - 2*ring)
		return left + ring + int(x/64*inner), left + ring + int(y/64*inner)
	}
	// Beside the arch, in a corner of the square: its dark ground.
	if c := rgba(at(6, 6)); c != ground {
		t.Errorf("the gate's ground should be dark, got %v", c)
	}
	// The ring around the gate is white, and so is the quiet zone round the code.
	if c := rgba(left+ring/2, 256); c != (color.RGBA{0xff, 0xff, 0xff, 0xff}) {
		t.Errorf("the ring should be white, got %v", c)
	}
	if c := rgba(2, 2); c != (color.RGBA{0xff, 0xff, 0xff, 0xff}) {
		t.Errorf("the quiet zone should be white, got %v", c)
	}
}

func TestPNGRefusesWhatNoCodeHolds(t *testing.T) {
	if _, err := PNG(string(make([]byte, 4000)), 512); err == nil {
		t.Error("a payload too long for any code should be refused")
	}
}
