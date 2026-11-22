package mail

import (
	"strconv"
	"strings"
)

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

// SignupData fills the letters of the teacher sign-up (bilimway.kz).
type SignupData struct {
	To    string
	Link  string // confirmation link (SignupConfirm) or the sign-in page (SignupExisting)
	Hours int    // how long the confirmation link works
}

var signupConfirmTexts = map[string][2]string{
	"ru": {"Подтвердите email для регистрации на bilimway.kz",
		"Здравствуйте!\n\nВы начали регистрацию преподавателя на bilimway.kz с адресом {email}.\n\n" +
			"Чтобы подтвердить адрес и создать пароль, откройте ссылку:\n{link}\n\n" +
			"Ссылка действует {hours} ч и работает один раз. Если вы не регистрировались, просто проигнорируйте это письмо."},
	"kk": {"bilimway.kz сайтында тіркелу үшін email-ді растаңыз",
		"Сәлеметсіз бе!\n\nСіз bilimway.kz сайтында {email} мекенжайымен оқытушы ретінде тіркеле бастадыңыз.\n\n" +
			"Мекенжайды растап, құпиясөз жасау үшін сілтемені ашыңыз:\n{link}\n\n" +
			"Сілтеме {hours} сағат жарамды және бір рет қана жұмыс істейді. Егер сіз тіркелмеген болсаңыз, бұл хатты елемеңіз."},
	"en": {"Confirm your email to sign up on bilimway.kz",
		"Hello!\n\nYou started signing up as a teacher on bilimway.kz with the address {email}.\n\n" +
			"To confirm the address and create a password, open the link:\n{link}\n\n" +
			"The link is valid for {hours} h and works once. If you did not sign up, just ignore this letter."},
}

var signupExistingTexts = map[string][2]string{
	"ru": {"Вход на bilimway.kz",
		"Здравствуйте!\n\nКто-то указал адрес {email} при регистрации на bilimway.kz. Аккаунт с этим адресом уже есть, " +
			"поэтому новый не создан и ничего в вашем аккаунте не изменилось.\n\nВойти можно здесь:\n{link}\n\n" +
			"Если вы не регистрировались, просто проигнорируйте это письмо."},
	"kk": {"bilimway.kz сайтына кіру",
		"Сәлеметсіз бе!\n\nbilimway.kz сайтында тіркелу кезінде {email} мекенжайы көрсетілді. Бұл мекенжаймен есептік жазба бар, " +
			"сондықтан жаңасы жасалмады және есептік жазбаңызда ештеңе өзгерген жоқ.\n\nМұнда кіруге болады:\n{link}\n\n" +
			"Егер сіз тіркелмеген болсаңыз, бұл хатты елемеңіз."},
	"en": {"Sign in to bilimway.kz",
		"Hello!\n\nSomeone entered {email} when signing up on bilimway.kz. An account with this address already exists, " +
			"so no new one was created and nothing in your account has changed.\n\nYou can sign in here:\n{link}\n\n" +
			"If you did not sign up, just ignore this letter."},
}

func signupLetter(texts map[string][2]string, locale string, d SignupData) Message {
	t, ok := texts[locale]
	if !ok {
		t = texts["ru"]
	}
	r := strings.NewReplacer("{email}", d.To, "{link}", d.Link, "{hours}", strconv.Itoa(d.Hours))
	return Message{To: d.To, Subject: r.Replace(t[0]), Text: r.Replace(t[1])}
}

// SignupConfirm is the letter with the one-time confirmation link of a new address.
func SignupConfirm(locale string, d SignupData) Message {
	return signupLetter(signupConfirmTexts, locale, d)
}

// SignupExisting goes to an address that already has an account: no link that could change it, only sign-in.
func SignupExisting(locale string, d SignupData) Message {
	return signupLetter(signupExistingTexts, locale, d)
}
