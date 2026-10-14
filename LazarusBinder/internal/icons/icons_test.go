package icons

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func smallICO() []byte {
	im := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	im.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var p bytes.Buffer
	png.Encode(&p, im)
	data := make([]byte, 22)
	binary.LittleEndian.PutUint16(data[2:], 1)
	binary.LittleEndian.PutUint16(data[4:], 1)
	data[6] = 16
	data[7] = 16
	binary.LittleEndian.PutUint16(data[10:], 1)
	binary.LittleEndian.PutUint16(data[12:], 32)
	binary.LittleEndian.PutUint32(data[14:], uint32(p.Len()))
	binary.LittleEndian.PutUint32(data[18:], 22)
	return append(data, p.Bytes()...)
}

func TestParseICOAndRejectInvalidBounds(t *testing.T) {
	data := smallICO()
	images, e := ParseICO(data)
	if e != nil || len(images) != 1 {
		t.Fatal(e)
	}
	bad := append([]byte(nil), data...)
	binary.LittleEndian.PutUint32(bad[18:], 0)
	if _, e := ParseICO(bad); e == nil {
		t.Fatal("accepted image inside directory")
	}
	bad = append([]byte(nil), data...)
	binary.LittleEndian.PutUint32(bad[14:], ^uint32(0))
	if _, e := ParseICO(bad); e == nil {
		t.Fatal("accepted overflowing image size")
	}
	if _, e := ParseICO(data[:20]); e == nil {
		t.Fatal("accepted truncated header")
	}
}

func FuzzParseICO(f *testing.F) {
	f.Add(smallICO())
	f.Add([]byte{0, 0, 1, 0, 1, 0})
	f.Fuzz(func(t *testing.T, data []byte) { ParseICO(data) })
}
