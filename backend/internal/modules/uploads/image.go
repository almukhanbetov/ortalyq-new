package uploads

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"image"
	_ "image/jpeg" // DecodeConfig for JPEG
	_ "image/png"  // DecodeConfig for PNG
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"ortalyq/backend/internal/apperr"
)

// Picture limits for appearance images (covers and backgrounds, stage 10 of marathons). Only raster formats that
// browsers show safely: SVG may carry scripts, GIF animations are not offered.
const (
	ImageMaxBytes = 5 << 20
	ImageMaxSide  = 6000
	ImageMinSide  = 16
)

var imageTypes = map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp"}

// ErrImageType / ErrImageTooLarge / ErrImageSize are the user-facing refusals for appearance pictures.
func ErrImageType() *apperr.Error {
	return apperr.New(http.StatusUnprocessableEntity, "image_type", "Загрузите картинку PNG, JPEG или WebP")
}

func ErrImageTooLarge() *apperr.Error {
	return apperr.New(http.StatusRequestEntityTooLarge, "file_too_large", "Картинка больше 5 МБ")
}

func ErrImageSize() *apperr.Error {
	return apperr.New(http.StatusUnprocessableEntity, "image_size", "Размер картинки — от 16 до 6000 пикселей по каждой стороне")
}

// webpSize reads the canvas size from a WebP header (VP8, VP8L and VP8X). The standard library has no WebP
// decoder; the header is enough to check the type and the size.
func webpSize(h []byte) (int, int, bool) {
	if len(h) < 30 || string(h[0:4]) != "RIFF" || string(h[8:12]) != "WEBP" {
		return 0, 0, false
	}
	switch string(h[12:16]) {
	case "VP8 ":
		if h[23] != 0x9d || h[24] != 0x01 || h[25] != 0x2a {
			return 0, 0, false
		}
		return int(binary.LittleEndian.Uint16(h[26:28]) & 0x3fff), int(binary.LittleEndian.Uint16(h[28:30]) & 0x3fff), true
	case "VP8L":
		if h[20] != 0x2f {
			return 0, 0, false
		}
		b := binary.LittleEndian.Uint32(h[21:25])
		return int(b&0x3fff) + 1, int((b>>14)&0x3fff) + 1, true
	case "VP8X":
		w := int(h[24]) | int(h[25])<<8 | int(h[26])<<16
		hh := int(h[27]) | int(h[28])<<8 | int(h[29])<<16
		return w + 1, hh + 1, true
	}
	return 0, 0, false
}

// imageInfo checks the real content (not the name or the declared type) and returns the type and the size.
func imageInfo(path string, head []byte) (string, int, int, error) {
	ct := http.DetectContentType(head)
	if _, ok := imageTypes[ct]; !ok {
		return "", 0, 0, ErrImageType()
	}
	if ct == "image/webp" {
		w, h, ok := webpSize(head)
		if !ok {
			return "", 0, 0, ErrImageType()
		}
		return ct, w, h, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return "", 0, 0, err
	}
	defer f.Close()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil || (format != "png" && format != "jpeg") {
		return "", 0, 0, ErrImageType()
	}
	return ct, cfg.Width, cfg.Height, nil
}

// SaveImage stores an appearance picture of a class. The caller has already checked that the principal may change
// the class appearance. The file is checked by its content: PNG, JPEG or WebP, at most ImageMaxBytes, each side
// between ImageMinSide and ImageMaxSide pixels.
func (s *Service) SaveImage(ctx context.Context, classroomID, uploaderID uuid.UUID, original string, size int64, r io.Reader) (*Attachment, error) {
	if size <= 0 {
		return nil, apperr.Validation(map[string]string{"file": "Файл пустой"})
	}
	if size > ImageMaxBytes {
		return nil, ErrImageTooLarge()
	}
	head := make([]byte, 512)
	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, err
	}
	head = head[:n]
	ext, ok := imageTypes[http.DetectContentType(head)]
	if !ok {
		return nil, ErrImageType()
	}
	rnd := make([]byte, 16)
	if _, err := rand.Read(rnd); err != nil {
		return nil, err
	}
	stored := hex.EncodeToString(rnd) + ext
	path := filepath.Join(s.dir, stored)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return nil, err
	}
	written, err := io.Copy(f, io.LimitReader(io.MultiReader(bytes.NewReader(head), r), ImageMaxBytes+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && written > ImageMaxBytes {
		err = ErrImageTooLarge()
	}
	var mime string
	if err == nil {
		var w, h int
		mime, w, h, err = imageInfo(path, head)
		if err == nil && (w < ImageMinSide || h < ImageMinSide || w > ImageMaxSide || h > ImageMaxSide) {
			err = ErrImageSize()
		}
	}
	if err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	uid := uploaderID
	a := &Attachment{ClassroomID: classroomID, UploaderID: &uid, OriginalName: sanitizeName(original), StoredName: stored, MimeType: mime, SizeBytes: written, Purpose: PurposeAppearance}
	if err := s.repo.Insert(ctx, a); err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return a, nil
}
