package exercises

// Catalog lists exercise types in the order of the add-exercise dialog.
// Kinds without a handler are shown as unavailable.
var Catalog = []CatalogEntry{
	{Kind: "image", Group: "media", Available: false},
	{Kind: "audio_video", Group: "media", Available: false},
	{Kind: KindPDF, Group: "media", Available: true},
	{Kind: KindGaps, Group: "words", Available: true},
	{Kind: KindTest, Group: "check", Available: true},
	{Kind: "true_false", Group: "check", Available: false},
	{Kind: "ordering", Group: "words", Available: false},
	{Kind: "matching", Group: "words", Available: false},
	{Kind: KindText, Group: "text", Available: true},
	{Kind: "word_set", Group: "words", Available: false},
	{Kind: "note", Group: "text", Available: false},
	{Kind: "link", Group: "other", Available: false},
	{Kind: "divider", Group: "other", Available: false},
	{Kind: "integrations", Group: "other", Available: false},
}

// KindHandler is the extension point for new exercise types.
// A kind validates what the teacher saves, grades what the student answers and hides its key from the student.
type KindHandler interface {
	Validate(req SaveRequest) map[string]string
	Grade(q Question, r Response) AnswerResult
	Sanitize(q Question) Question
}

var kinds = map[string]KindHandler{KindTest: testKind{}, KindGaps: gapsKind{}, KindText: textKind{}, KindPDF: pdfKind{}}

func handlerFor(kind string) (KindHandler, bool) {
	h, ok := kinds[kind]
	return h, ok
}
