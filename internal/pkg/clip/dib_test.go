package clip

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"testing"
)

func TestDIBToPNG_32bpp_TopDown(t *testing.T) {
	dib := make([]byte, 40+2*1*4)
	binary.LittleEndian.PutUint32(dib[0:4], 40)
	binary.LittleEndian.PutUint32(dib[4:8], uint32(int32(2)))
	var h int32 = -1
	binary.LittleEndian.PutUint32(dib[8:12], uint32(h))
	binary.LittleEndian.PutUint16(dib[12:14], 1)
	binary.LittleEndian.PutUint16(dib[14:16], 32)
	binary.LittleEndian.PutUint32(dib[16:20], 0)

	pix := dib[40:]
	pix[0] = 0
	pix[1] = 0
	pix[2] = 255
	pix[3] = 0
	pix[4] = 0
	pix[5] = 255
	pix[6] = 0
	pix[7] = 0

	pngBytes, err := dibToPNG(dib)
	if err != nil {
		t.Fatalf("dibToPNG error: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("png decode error: %v", err)
	}
	if img.Bounds().Dx() != 2 || img.Bounds().Dy() != 1 {
		t.Fatalf("unexpected bounds: %v", img.Bounds())
	}
	r0, g0, b0, _ := img.At(0, 0).RGBA()
	if r0 < 0xff00 || g0 > 0x0100 || b0 > 0x0100 {
		t.Fatalf("expected red pixel at (0,0), got r=%x g=%x b=%x", r0, g0, b0)
	}
	r1, g1, b1, _ := img.At(1, 0).RGBA()
	if g1 < 0xff00 || r1 > 0x0100 || b1 > 0x0100 {
		t.Fatalf("expected green pixel at (1,0), got r=%x g=%x b=%x", r1, g1, b1)
	}
}
