package media

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func sample(t *testing.T, format string) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 4), uint8(y * 8), 120, 255})
		}
	}
	var b bytes.Buffer
	var err error
	if format == "png" {
		err = png.Encode(&b, img)
	} else {
		err = jpeg.Encode(&b, img, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestImageValidationAndReencoding(t *testing.T) {
	for _, format := range []string{"png", "jpeg"} {
		t.Run(format, func(t *testing.T) {
			raw := append(sample(t, format), []byte("private metadata <script>attack</script>")...)
			out, err := sanitize(raw, "image/"+format, "Un sentiero 🌿")
			if err != nil {
				t.Fatal(err)
			}
			if out.width != 64 || out.height != 32 || bytes.Contains(out.data, []byte("private metadata")) {
				t.Fatal("metadata/dimensions not sanitized")
			}
			cfg, kind, err := image.DecodeConfig(bytes.NewReader(out.data))
			if err != nil || kind != format || cfg.Width != 64 {
				t.Fatal("invalid reencoded image", err)
			}
		})
	}
	for _, tc := range []struct {
		name, mime, alt string
		data            []byte
		expected        error
	}{
		{"svg", "image/svg+xml", "alt", []byte(`<svg onload="alert(1)"/>`), ErrUnsupported},
		{"spoofed", "image/jpeg", "alt", sample(t, "png"), ErrUnsupported},
		{"tooLarge", "image/png", "alt", make([]byte, MaxUploadBytes+1), ErrTooLarge},
		{"altRequired", "image/png", "", sample(t, "png"), nil},
		{"altLong", "image/png", strings.Repeat("🌿", 201), sample(t, "png"), nil},
		{"altControl", "image/png", "bad\x00", sample(t, "png"), nil},
		{"broken", "image/png", "alt", sample(t, "png")[:40], nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := sanitize(tc.data, tc.mime, tc.alt)
			if err == nil || (tc.expected != nil && !errors.Is(err, tc.expected)) {
				t.Fatal("unexpected result", err)
			}
		})
	}
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4097, 1)))
	if _, err := sanitize(b.Bytes(), "image/png", "wide"); err == nil {
		t.Fatal("oversized dimensions accepted")
	}
}
func TestStorageConfig(t *testing.T) {
	for _, endpoint := range []string{"http://example.test", "http://user:secret@localhost:8333", "https://example.test/path", "https://example.test?secret=x", "ftp://localhost:8333"} {
		if _, err := NewS3(endpoint, "us-east-1", "radura-test", "test", strings.Repeat("a", 32)); err == nil {
			t.Fatal("unsafe S3 endpoint accepted")
		}
	}
	if _, err := NewS3("http://127.0.0.1:8333", "us-east-1", "radura-test", "test", strings.Repeat("a", 32)); err != nil {
		t.Fatal(err)
	}
}
