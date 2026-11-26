package uploads

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"ortalyq/backend/internal/apperr"
)

// PDFMaxBytes is the largest PDF of a lesson block («PDF-документ»).
const PDFMaxBytes = 20 << 20

// ErrPDFType / ErrPDFTooLarge are the user-facing refusals for lesson PDFs.
func ErrPDFType() *apperr.Error {
	return apperr.New(http.StatusUnprocessableEntity, "pdf_type", "Загрузите файл в формате PDF")
}

func ErrPDFTooLarge() *apperr.Error {
	return apperr.New(http.StatusRequestEntityTooLarge, "file_too_large", "PDF больше 20 МБ")
}

// StoredFile is a checked file written to the upload directory.
type StoredFile struct {
	StoredName string
	MimeType   string
	SizeBytes  int64
}

// pdfTail is how much of the end of a file is searched for the end-of-file marker. Writers may append a little
// after %%EOF (line breaks, padding), the specification asks readers to look in the last 1024 bytes.
const pdfTail = 1024

// isPDF checks the real content, not the name or the declared type: the header "%PDF-1.x" / "%PDF-2.x" at the start
// and the end-of-file marker near the end. A renamed picture or text, an HTML page or a cut-off download fails.
func isPDF(head []byte, path string, size int64) bool {
	if !bytes.HasPrefix(head, []byte("%PDF-")) || len(head) < 8 || (head[5] != '1' && head[5] != '2') || head[6] != '.' {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	n := int64(pdfTail)
	if size < n {
		n = size
	}
	tail := make([]byte, n)
	if _, err := f.ReadAt(tail, size-n); err != nil && !errors.Is(err, io.EOF) {
		return false
	}
	return bytes.Contains(tail, []byte("%%EOF"))
}

// StorePDF writes a PDF to the upload directory after checking its content (see isPDF) and its size (at most
// PDFMaxBytes). The caller records it and removes the file with RemoveStored if recording fails.
func (s *Service) StorePDF(size int64, r io.Reader) (*StoredFile, error) {
	if size <= 0 {
		return nil, apperr.Validation(map[string]string{"file": "Файл пустой"})
	}
	if size > PDFMaxBytes {
		return nil, ErrPDFTooLarge()
	}
	head := make([]byte, 16)
	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, err
	}
	head = head[:n]
	if !bytes.HasPrefix(head, []byte("%PDF-")) {
		return nil, ErrPDFType()
	}
	rnd := make([]byte, 16)
	if _, err := rand.Read(rnd); err != nil {
		return nil, err
	}
	stored := hex.EncodeToString(rnd) + ".pdf"
	path := filepath.Join(s.dir, stored)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return nil, err
	}
	written, err := io.Copy(f, io.LimitReader(io.MultiReader(bytes.NewReader(head), r), PDFMaxBytes+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && written > PDFMaxBytes {
		err = ErrPDFTooLarge()
	}
	if err == nil && !isPDF(head, path, written) {
		err = ErrPDFType()
	}
	if err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return &StoredFile{StoredName: stored, MimeType: "application/pdf", SizeBytes: written}, nil
}

// SanitizeName cleans an uploaded file name for storage and the download header.
func SanitizeName(name string) string { return sanitizeName(name) }

// ServePDF writes a stored PDF for viewing in the browser: inline (the browser's own viewer, also inside a frame of
// the web app) with Range support for large files. The caller has checked access. frameAncestors are the web app
// origins allowed to embed it; every other site is refused by the browser (X-Frame-Options of the API is replaced
// by the CSP frame-ancestors list). No "sandbox": it switches Chrome's PDF viewer off.
func ServePDF(c *gin.Context, f *os.File, name string, modified time.Time, frameAncestors []string) {
	defer f.Close()
	w := c.Writer.Header()
	w.Del("X-Frame-Options")
	w.Set("Content-Type", "application/pdf")
	w.Set("Content-Disposition", fmt.Sprintf(`inline; filename="document.pdf"; filename*=UTF-8''%s`, url.PathEscape(name)))
	w.Set("X-Content-Type-Options", "nosniff")
	w.Set("Content-Security-Policy", "frame-ancestors 'self' "+strings.Join(frameAncestors, " "))
	w.Set("Cache-Control", "private, max-age=0, must-revalidate")
	w.Set("Cross-Origin-Resource-Policy", "same-site")
	http.ServeContent(c.Writer, c.Request, "", modified, f)
}
