package marathons

import (
	"github.com/google/uuid"

	"ortalyq/backend/internal/apperr"
)

// ---- Appearance (stage 10): covers, reactions to answers, class background ----

// Ready reactions. Codes are stored, the emoji themselves live in the frontend, so a code never changes meaning.
var (
	CorrectIcons   = []string{"party_face", "party_popper", "muscle", "fire", "brain", "check"}
	IncorrectIcons = []string{"sad", "grimace", "see_no_evil", "thinking", "confused", "cross"}
	// Backgrounds: ready variants; custom is the teacher's own picture.
	Backgrounds = []string{"plain", "sunrise", "waves", "petals", "leaves", "autumn", BackgroundCustom}
)

const (
	SoundCount       = 5 // «Звук 1» … «Звук 5» for each of the correct and incorrect answers
	BackgroundPlain  = "plain"
	BackgroundCustom = "custom"
	// Image slots of the marathon and of a lesson in it.
	SlotCover      = "cover"
	SlotBackground = "background"

	AppearanceMarathon = "marathon"
	AppearanceLesson   = "lesson"
)

type IconReaction struct {
	Enabled bool     `json:"enabled"`
	Icons   []string `json:"icons"`
}

type SoundReaction struct {
	Enabled bool `json:"enabled"`
	Sound   int  `json:"sound"`
}

// Reactions: «Иконка правильного / неправильного ответа», «Звук правильного / неправильного ответа».
type Reactions struct {
	CorrectIcon    IconReaction  `json:"correct_icon"`
	IncorrectIcon  IconReaction  `json:"incorrect_icon"`
	CorrectSound   SoundReaction `json:"correct_sound"`
	IncorrectSound SoundReaction `json:"incorrect_sound"`
}

// DefaultReactions are the values shown when «Реакции на ответы» is switched on for the first time
// (as in the original: two icons of each kind and «Звук 1»).
func DefaultReactions() Reactions {
	return Reactions{
		CorrectIcon:    IconReaction{Enabled: true, Icons: []string{"party_face", "party_popper"}},
		IncorrectIcon:  IconReaction{Enabled: true, Icons: []string{"sad", "grimace"}},
		CorrectSound:   SoundReaction{Enabled: true, Sound: 1},
		IncorrectSound: SoundReaction{Enabled: true, Sound: 1},
	}
}

// Appearance of the marathon (lesson_id NULL) or the own appearance of a lesson in it. For a lesson Enabled is
// «Своё оформление урока»; the marathon's row is always enabled.
type Appearance struct {
	Enabled           bool       `json:"enabled"`
	ReactionsEnabled  bool       `json:"reactions_enabled"`
	Reactions         Reactions  `json:"reactions"`
	BackgroundEnabled bool       `json:"background_enabled"`
	Background        string     `json:"background"`
	BackgroundImageID *uuid.UUID `json:"background_image_id"`
}

// DefaultAppearance: everything off; a lesson has no own appearance until the teacher switches it on.
func DefaultAppearance(lesson bool) Appearance {
	return Appearance{Enabled: !lesson, Reactions: DefaultReactions(), Background: BackgroundPlain}
}

// EffectiveAppearance is what the lesson player applies. Reactions and Background are null when switched off.
type EffectiveAppearance struct {
	Source     string               `json:"source"`
	Reactions  *Reactions           `json:"reactions"`
	Background *EffectiveBackground `json:"background"`
}

type EffectiveBackground struct {
	Preset  string     `json:"preset"`
	ImageID *uuid.UUID `json:"image_id"`
}

// Resolve picks the appearance of a lesson: its own one when «Оформление урока» is on in the marathon and the lesson
// has an own appearance switched on; otherwise the marathon's. The lesson appearance replaces the marathon's as a
// whole (reactions and background together), so a lesson can also switch reactions off.
func Resolve(lessonAppearance bool, marathon, lesson *Appearance) EffectiveAppearance {
	a, src := marathon, AppearanceMarathon
	if lessonAppearance && lesson != nil && lesson.Enabled {
		a, src = lesson, AppearanceLesson
	}
	out := EffectiveAppearance{Source: src}
	if a == nil {
		return out
	}
	r := a.Reactions
	if a.ReactionsEnabled && (r.CorrectIcon.Enabled || r.IncorrectIcon.Enabled || r.CorrectSound.Enabled || r.IncorrectSound.Enabled) {
		if !r.CorrectIcon.Enabled {
			r.CorrectIcon.Icons = []string{}
		}
		if !r.IncorrectIcon.Enabled {
			r.IncorrectIcon.Icons = []string{}
		}
		out.Reactions = &r
	}
	if a.BackgroundEnabled && a.Background != BackgroundPlain {
		if a.Background != BackgroundCustom {
			out.Background = &EffectiveBackground{Preset: a.Background}
		} else if a.BackgroundImageID != nil {
			out.Background = &EffectiveBackground{Preset: BackgroundCustom, ImageID: a.BackgroundImageID}
		}
	}
	return out
}

func inList(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func checkIcons(v *apperr.V, field string, r *IconReaction, allowed []string) {
	if r.Icons == nil {
		r.Icons = []string{}
	}
	seen := map[string]bool{}
	ok := true
	for _, c := range r.Icons {
		if !inList(allowed, c) || seen[c] {
			ok = false
		}
		seen[c] = true
	}
	v.Check(ok, field, "Выберите иконки из списка")
	v.Check(!r.Enabled || len(r.Icons) > 0, field, "Выберите хотя бы одну иконку")
}

// checkAppearance validates the request. hasImage tells whether a custom background picture is uploaded.
func checkAppearance(a *Appearance, hasImage bool) error {
	v := apperr.V{}
	checkIcons(&v, "reactions.correct_icon", &a.Reactions.CorrectIcon, CorrectIcons)
	checkIcons(&v, "reactions.incorrect_icon", &a.Reactions.IncorrectIcon, IncorrectIcons)
	v.Check(a.Reactions.CorrectSound.Sound >= 1 && a.Reactions.CorrectSound.Sound <= SoundCount, "reactions.correct_sound", "Выберите звук от 1 до 5")
	v.Check(a.Reactions.IncorrectSound.Sound >= 1 && a.Reactions.IncorrectSound.Sound <= SoundCount, "reactions.incorrect_sound", "Выберите звук от 1 до 5")
	if a.Background == "" {
		a.Background = BackgroundPlain
	}
	v.Check(inList(Backgrounds, a.Background), "background", "Неизвестный фон")
	v.Check(a.Background != BackgroundCustom || hasImage, "background", "Сначала загрузите свою картинку фона")
	return v.Err()
}

func knownSlot(s string) bool { return s == SlotCover || s == SlotBackground }
