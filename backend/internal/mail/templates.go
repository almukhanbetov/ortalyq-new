package mail

import "strings"

// InvitationData fills the marathon invitation letter.
type InvitationData struct {
	To       string
	Student  string // full name of the invited student
	Teacher  string
	Marathon string
	Link     string
	Expires  string // already formatted in the marathon time zone
}

// Locales of letters ("Язык писем").
var Locales = []string{"ru", "kk", "en"}

var invitationTexts = map[string][2]string{
	"ru": {"Приглашение в марафон «{marathon}»",
		"Здравствуйте, {student}!\n\n{teacher} приглашает вас в марафон «{marathon}» на ORTALYQ.\n\n" +
			"Чтобы присоединиться, откройте ссылку и войдите под своей учётной записью ({email}):\n{link}\n\n" +
			"Ссылка действует до {expires} и работает один раз. Если вы не ждали это письмо, просто проигнорируйте его."},
	"kk": {"«{marathon}» марафонына шақыру",
		"Сәлеметсіз бе, {student}!\n\n{teacher} сізді ORTALYQ платформасындағы «{marathon}» марафонына шақырады.\n\n" +
			"Қосылу үшін сілтемені ашып, өз есептік жазбаңызбен кіріңіз ({email}):\n{link}\n\n" +
			"Сілтеме {expires} дейін жарамды және бір рет қана жұмыс істейді. Егер бұл хатты күтпесеңіз, оны елемеңіз."},
	"en": {"Invitation to the marathon “{marathon}”",
		"Hello, {student}!\n\n{teacher} invites you to the marathon “{marathon}” on ORTALYQ.\n\n" +
			"To join, open the link and sign in with your account ({email}):\n{link}\n\n" +
			"The link is valid until {expires} and works once. If you did not expect this letter, just ignore it."},
}

// Invitation renders the invitation letter in the chosen language (ru when unknown).
func Invitation(locale string, d InvitationData) Message {
	t, ok := invitationTexts[locale]
	if !ok {
		t = invitationTexts["ru"]
	}
	r := strings.NewReplacer("{marathon}", d.Marathon, "{student}", d.Student, "{teacher}", d.Teacher, "{email}", d.To,
		"{link}", d.Link, "{expires}", d.Expires)
	return Message{To: d.To, Subject: r.Replace(t[0]), Text: r.Replace(t[1])}
}
