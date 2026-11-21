package uploads

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"

	"ortalyq/backend/internal/apperr"
)

func pngOf(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

func jpegOf(w, h int) []byte {
	var b bytes.Buffer
	_ = jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h)), nil)
	return b.Bytes()
}

// webpVP8L builds a lossless WebP header of the given size (the body is not needed for the check).
func webpVP8L(w, h int) []byte {
	b := []byte("RIFF\x00\x00\x00\x00WEBPVP8L\x00\x00\x00\x00\x2f")
	v := uint32(w-1) | uint32(h-1)<<14
	b = append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
	return append(b, make([]byte, 40)...)
}

func TestWebpSize(t *testing.T) {
	if w, h, ok := webpSize(webpVP8L(640, 480)); !ok || w != 640 || h != 480 {
		t.Fatalf("VP8L: %d %d %v", w, h, ok)
	}
	x := []byte("RIFF\x00\x00\x00\x00WEBPVP8X\x00\x00\x00\x00\x00\x00\x00\x00")
	x = append(x, 0x7f, 0x07, 0x00, 0x37, 0x04, 0x00) // 1920 x 1080 minus one
	if w, h, ok := webpSize(x); !ok || w != 1920 || h != 1080 {
		t.Fatalf("VP8X: %d %d %v", w, h, ok)
	}
	if _, _, ok := webpSize([]byte("RIFF\x00\x00\x00\x00WAVEfmt ")); ok {
		t.Fatal("not a webp")
	}
}

func TestSaveImage(t *testing.T) {
	dir := t.TempDir()
	s := &Service{dir: dir, maxBytes: 1 << 20}
	ctx := context.Background()
	cid, uid := uuid.New(), uuid.New()
	code := func(err error) string {
		if e, ok := err.(*apperr.Error); ok {
			return e.Code
		}
		if err == nil {
			return ""
		}
		return "other"
	}
	bad := map[string]struct {
		data []byte
		want string
	}{
		"svg":        {[]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), "image_type"},
		"pdf":        {[]byte("%PDF-1.4\n" + strings.Repeat("x", 100)), "image_type"},
		"gif":        {[]byte("GIF89a" + strings.Repeat("\x00", 40)), "image_type"},
		"broken png": {append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 50)...), "image_type"},
		"tiny":       {pngOf(8, 8), "image_size"},
		"huge":       {pngOf(ImageMaxSide+1, 16), "image_size"},
		"webp tiny":  {webpVP8L(10, 10), "image_size"},
		"too large":  {append(pngOf(20, 20), make([]byte, ImageMaxBytes)...), "file_too_large"},
		"empty":      {nil, "validation_failed"},
	}
	for name, c := range bad {
		_, err := s.SaveImage(ctx, cid, uid, name+".png", int64(len(c.data)), bytes.NewReader(c.data))
		if got := code(err); got != c.want {
			t.Errorf("%s: want %s got %s (%v)", name, c.want, got, err)
		}
	}
	// A refused file leaves nothing on disk.
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Fatalf("files left after refusals: %d", len(left))
	}
	// Valid pictures are stored by the integration test (TestMarathonAppearance), which has a database.
}
