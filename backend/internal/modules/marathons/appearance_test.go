package marathons

import (
	"testing"

	"github.com/google/uuid"
)

func TestResolveAppearance(t *testing.T) {
	img := uuid.New()
	m := DefaultAppearance(false)
	m.ReactionsEnabled = true
	m.BackgroundEnabled, m.Background = true, "waves"
	l := DefaultAppearance(true)
	l.Enabled = true
	l.BackgroundEnabled, l.Background, l.BackgroundImageID = true, BackgroundCustom, &img

	// Nothing saved: nothing applies.
	if r := Resolve(false, nil, nil); r.Source != AppearanceMarathon || r.Reactions != nil || r.Background != nil {
		t.Fatalf("empty: %+v", r)
	}
	// Marathon appearance.
	r := Resolve(false, &m, &l)
	if r.Source != AppearanceMarathon || r.Reactions == nil || r.Background == nil || r.Background.Preset != "waves" {
		t.Fatalf("marathon: %+v", r)
	}
	// «Оформление урока» on and the lesson has its own: the lesson wins as a whole (its reactions are off).
	r = Resolve(true, &m, &l)
	if r.Source != AppearanceLesson || r.Reactions != nil || r.Background == nil || *r.Background.ImageID != img {
		t.Fatalf("lesson: %+v", r)
	}
	// «Оформление урока» off: the lesson's own appearance is ignored.
	if r = Resolve(false, &m, &l); r.Source != AppearanceMarathon {
		t.Fatalf("lesson appearance off: %+v", r)
	}
	// The lesson's own appearance switched off: the marathon's applies.
	l.Enabled = false
	if r = Resolve(true, &m, &l); r.Source != AppearanceMarathon {
		t.Fatalf("lesson own off: %+v", r)
	}
	// Reactions on with every part off are no reactions; a disabled icon part sends no icons.
	m.Reactions.CorrectIcon.Enabled, m.Reactions.IncorrectIcon.Enabled = false, false
	m.Reactions.CorrectSound.Enabled, m.Reactions.IncorrectSound.Enabled = false, false
	if r = Resolve(false, &m, nil); r.Reactions != nil {
		t.Fatalf("all parts off: %+v", r.Reactions)
	}
	m.Reactions.CorrectSound.Enabled = true
	if r = Resolve(false, &m, nil); r.Reactions == nil || len(r.Reactions.CorrectIcon.Icons) != 0 || len(r.Reactions.IncorrectIcon.Icons) != 0 {
		t.Fatalf("icons of disabled parts must not be sent: %+v", r.Reactions)
	}
	// Plain background or a custom one without its picture shows nothing.
	m.Background = BackgroundPlain
	if r = Resolve(false, &m, nil); r.Background != nil {
		t.Fatal("plain background")
	}
	m.Background, m.BackgroundImageID = BackgroundCustom, nil
	if r = Resolve(false, &m, nil); r.Background != nil {
		t.Fatal("custom background without a picture")
	}
}

func TestCheckAppearance(t *testing.T) {
	ok := DefaultAppearance(false)
	if err := checkAppearance(&ok, false); err != nil {
		t.Fatalf("defaults: %v", err)
	}
	cases := map[string]func(a *Appearance){
		"unknown icon":         func(a *Appearance) { a.Reactions.CorrectIcon.Icons = []string{"skull"} },
		"incorrect in correct": func(a *Appearance) { a.Reactions.CorrectIcon.Icons = []string{"sad"} },
		"duplicate icon":       func(a *Appearance) { a.Reactions.IncorrectIcon.Icons = []string{"sad", "sad"} },
		"enabled without icon": func(a *Appearance) { a.Reactions.CorrectIcon.Icons = nil },
		"sound 0":              func(a *Appearance) { a.Reactions.CorrectSound.Sound = 0 },
		"sound 6":              func(a *Appearance) { a.Reactions.IncorrectSound.Sound = 6 },
		"unknown background":   func(a *Appearance) { a.Background = "space" },
		"custom without image": func(a *Appearance) { a.Background = BackgroundCustom },
	}
	for name, f := range cases {
		a := DefaultAppearance(false)
		f(&a)
		if err := checkAppearance(&a, false); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// A disabled icon part may keep an empty list; a custom background with a picture is fine.
	a := DefaultAppearance(false)
	a.Reactions.CorrectIcon = IconReaction{Enabled: false}
	a.Background = BackgroundCustom
	if err := checkAppearance(&a, true); err != nil || a.Reactions.CorrectIcon.Icons == nil {
		t.Fatalf("valid: %v", err)
	}
	a.Background = ""
	if err := checkAppearance(&a, false); err != nil || a.Background != BackgroundPlain {
		t.Fatalf("empty background is plain: %v %q", err, a.Background)
	}
}
