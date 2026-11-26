// Package languages is the list of languages the project offers for teaching materials and marathons
// (ISO 639-1 codes). Display names live in the frontend locales ("languages.<code>").
package languages

var Codes = []string{"ar", "cs", "de", "en", "es", "fr", "it", "ja", "kk", "ko", "pl", "pt", "ru", "tr", "uz", "zh"}

// Known reports whether code is on the list.
func Known(code string) bool {
	for _, l := range Codes {
		if l == code {
			return true
		}
	}
	return false
}
