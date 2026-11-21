package exercises

// Catalog lists exercise types in the order of the add-exercise dialog.
// Only "test" is implemented in this stage; others are shown as unavailable.
var Catalog = []CatalogEntry{
	{Kind: "image", Group: "media", Available: false},
	{Kind: "audio_video", Group: "media", Available: false},
	{Kind: "gaps", Group: "words", Available: false},
	{Kind: KindTest, Group: "check", Available: true},
	{Kind: "true_false", Group: "check", Available: false},
	{Kind: "ordering", Group: "words", Available: false},
	{Kind: "matching", Group: "words", Available: false},
	{Kind: "text", Group: "text", Available: false},
	{Kind: "word_set", Group: "words", Available: false},
	{Kind: "note", Group: "text", Available: false},
	{Kind: "link", Group: "other", Available: false},
	{Kind: "divider", Group: "other", Available: false},
	{Kind: "integrations", Group: "other", Available: false},
}

// KindHandler is the extension point for new exercise types.
type KindHandler interface {
	Validate(req SaveRequest) map[string]string
}

var kinds = map[string]KindHandler{KindTest: testKind{}}

func handlerFor(kind string) (KindHandler, bool) {
	h, ok := kinds[kind]
	return h, ok
}
