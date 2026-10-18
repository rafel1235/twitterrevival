package media

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"radura.local/api/internal/auth"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxUploadBytes = 5 * 1024 * 1024
const MaxStoredBytes = 8 * 1024 * 1024

type sanitized struct {
	data          []byte
	mime          string
	width, height int
}

func invalid(message string) error {
	return &auth.ValidationError{Fields: map[string]string{"image": message}}
}
func sanitize(data []byte, claimed, alt string) (sanitized, error) {
	if len(data) == 0 {
		return sanitized{}, invalid("Il file è vuoto.")
	}
	if len(data) > MaxUploadBytes {
		return sanitized{}, ErrTooLarge
	}
	if !utf8.ValidString(alt) || utf8.RuneCountInString(alt) < 1 || utf8.RuneCountInString(alt) > 200 || strings.ContainsFunc(alt, unicode.IsControl) {
		return sanitized{}, &auth.ValidationError{Fields: map[string]string{"alt_text": "Descrivi l'immagine in 1–200 caratteri, senza controlli."}}
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png") {
		return sanitized{}, ErrUnsupported
	}
	mime := "image/" + format
	if claimed != mime {
		return sanitized{}, ErrUnsupported
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 4096 || cfg.Height > 4096 || int64(cfg.Width)*int64(cfg.Height) > 8000000 {
		return sanitized{}, invalid("Immagine troppo grande: massimo 4096 px per lato e 8 megapixel.")
	}
	decoded, decodedFormat, err := image.Decode(bytes.NewReader(data))
	if err != nil || decodedFormat != format {
		return sanitized{}, invalid("Immagine danneggiata o non valida.")
	}
	var output bytes.Buffer
	if format == "jpeg" {
		err = jpeg.Encode(&output, decoded, &jpeg.Options{Quality: 85})
	} else {
		err = png.Encode(&output, decoded)
	}
	if err != nil {
		return sanitized{}, invalid("Immagine non elaborabile.")
	}
	if output.Len() > MaxStoredBytes {
		return sanitized{}, ErrTooLarge
	}
	return sanitized{output.Bytes(), mime, cfg.Width, cfg.Height}, nil
}
