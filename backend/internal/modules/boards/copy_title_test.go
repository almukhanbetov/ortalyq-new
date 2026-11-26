package boards

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestCopyTitle(t *testing.T) {
	day := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	if got := CopyTitle("English A1 — Hello!", "Английский — Данияр", day); got != "English A1 — Hello! — Английский — Данияр — 26.11.2026" {
		t.Fatalf("title: %q", got)
	}
	long := CopyTitle(strings.Repeat("Б", 300), "Класс", day)
	if utf8.RuneCountInString(long) != 200 || !strings.HasSuffix(long, "… — Класс — 26.11.2026") {
		t.Fatalf("long: %d %q", utf8.RuneCountInString(long), long[len(long)-40:])
	}
}
