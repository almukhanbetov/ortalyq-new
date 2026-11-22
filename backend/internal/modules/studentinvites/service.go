package studentinvites

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ortalyq/backend/internal/apperr"
	"ortalyq/backend/internal/database"
	"ortalyq/backend/internal/mail"
	"ortalyq/backend/internal/modules/auth"
)

// Mailer is satisfied by mail.Dispatcher: the letter is written to the outbox in the same transaction as the link
// and sent after the commit; the body is removed from the outbox once sent.
type Mailer interface {
	Enqueue(ctx context.Context, tx pgx.Tx, kind string, refID uuid.UUID, locale string, m mail.Message) (uuid.UUID, error)
	Kick()
}

// Accounts is satisfied by users.Repository. q may be a transaction.
type Accounts interface {
	ActiveAccountByEmail(ctx context.Context, q database.Querier, email string) (uuid.UUID, bool, error)
	ActiveProfile(ctx context.Context, q database.Querier, id uuid.UUID) (name, role, email string, ok bool, err error)
	EmailTaken(ctx context.Context, q database.Querier, email string) (bool, error)
	CreateSelfRegistered(ctx context.Context, q database.Querier, email, name, role, hash, color string, termsAt time.Time) (uuid.UUID, error)
	IsLinked(ctx context.Context, q database.Querier, teacherID, studentID uuid.UUID) (bool, error)
	LinkInvitedStudent(ctx context.Context, q database.Querier, teacherID, studentID, invitationID uuid.UUID) error
}

// Sessions is satisfied by auth.Service.
type Sessions interface {
	StartSession(ctx context.Context, userID uuid.UUID, ip, ua string) (string, auth.Principal, error)
}

type Service struct {
	repo      *Repository
	accounts  Accounts
	sessions  Sessions
	mailer    Mailer
	normalize func(string) string
	color     func(string) string
	appURL    string
	opt       Options
}

// normalize checks and lower-cases an address ("" when invalid), as in the sign-up flow; color is the default
// avatar colour of a new account.
func NewService(repo *Repository, accounts Accounts, sessions Sessions, mailer Mailer, normalize, color func(string) string, appURL string, opt Options) *Service {
	return &Service{repo: repo, accounts: accounts, sessions: sessions, mailer: mailer, normalize: normalize, color: color,
		appURL: strings.TrimRight(appURL, "/"), opt: opt.withDefaults()}
}

func tokenHash(t string) []byte {
	h := sha256.Sum256([]byte(t))
	return h[:]
}

func newToken() (string, []byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	t := base64.RawURLEncoding.EncodeToString(b)
	return t, tokenHash(t), nil
}

func validToken(t string) bool {
	return len(t) == 43 && strings.Trim(t, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_") == ""
}

func validLocale(l string) bool { return l == "ru" || l == "kk" || l == "en" }

func tooMany(wait float64) error {
	sec := int(math.Ceil(wait))
	if sec < 1 {
		sec = 1
	}
	e := apperr.TooMany(fmt.Sprintf("Приглашение уже отправлено. Отправить новое можно через %d с.", sec))
	e.Fields = map[string]string{"retry_after": fmt.Sprint(sec)}
	return e
}

// maskEmail shows an address without revealing it fully: "st***@example.kz".
func maskEmail(e string) string {
	i := strings.Index(e, "@")
	if i <= 0 {
		return "***"
	}
	keep := min(2, i)
	return e[:keep] + "***" + e[i:]
}

// Invite is «Пригласить ученика» of a private teacher: a letter with a one-time link to the address. A newer
// letter to the same address supersedes the older link. The letter is the same whether the address has an account
// or not; only a student who is already in the teacher's list is reported (that is the teacher's own data).
func (s *Service) Invite(ctx context.Context, p auth.Principal, req InviteRequest) (*InviteResponse, error) {
	if p.Role != auth.RolePrivateTeacher {
		return nil, apperr.Forbidden()
	}
	email := s.normalize(req.Email)
	if email == "" {
		return nil, apperr.Validation(map[string]string{"email": "Введите корректный email"})
	}
	if strings.EqualFold(email, p.Email) {
		return nil, apperr.Validation(map[string]string{"email": "Нельзя пригласить самого себя"})
	}
	locale := req.Locale
	if !validLocale(locale) {
		locale = p.Locale
	}
	if !validLocale(locale) {
		locale = "ru"
	}
	var out *InviteResponse
	err := database.WithTx(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		if err := s.repo.LockAddress(ctx, tx, p.UserID.String()+":"+email); err != nil {
			return err
		}
		if id, ok, err := s.accounts.ActiveAccountByEmail(ctx, tx, email); err != nil {
			return err
		} else if ok {
			linked, err := s.accounts.IsLinked(ctx, tx, p.UserID, id)
			if err != nil {
				return err
			}
			if linked {
				return apperr.Conflict(CodeAlreadyLinked, "Этот ученик уже есть в вашем списке")
			}
		}
		u, err := s.repo.Usage(ctx, tx, p.UserID, email, limitWindow, s.opt.ResendAfter)
		if err != nil {
			return err
		}
		switch {
		case u.WaitResend > 0:
			return tooMany(u.WaitResend)
		case u.ByTeacher >= s.opt.MaxPerTeacher:
			return tooMany(u.WaitWindow)
		}
		if err := s.repo.Supersede(ctx, tx, p.UserID, email); err != nil {
			return err
		}
		token, hash, err := newToken()
		if err != nil {
			return err
		}
		id, exp, err := s.repo.Insert(ctx, tx, p.UserID, email, locale, hash, s.opt.LinkTTL)
		if err != nil {
			return err
		}
		// The token travels in the fragment: browsers do not send it to servers or in Referer.
		msg := mail.StudentInvitation(locale, mail.StudentInvitationData{To: email, Teacher: p.FullName,
			Link: s.appURL + "/student-invite#" + token, Days: int(math.Round(s.opt.LinkTTL.Hours() / 24))})
		oid, err := s.mailer.Enqueue(ctx, tx, "student_invitation", id, locale, msg)
		if err != nil {
			return err
		}
		out = &InviteResponse{Email: email, ExpiresAt: exp, ResendAfter: int(s.opt.ResendAfter.Seconds())}
		return s.repo.SetOutbox(ctx, tx, id, oid)
	})
	if err != nil {
		return nil, err
	}
	s.mailer.Kick()
	return out, nil
}

func linkError(code string) error {
	switch code {
	case CodeLinkExpired:
		return apperr.New(http.StatusGone, code, "Срок действия приглашения истёк. Попросите преподавателя отправить новое.")
	case CodeLinkUsed:
		return apperr.New(http.StatusGone, code, "Приглашение уже принято.")
	}
	return apperr.New(http.StatusNotFound, CodeLinkInvalid, "Приглашение недействительно. Попросите преподавателя отправить новое.")
}

// state checks a link: nil when it can still be used. A newer letter makes the older link expired.
func state(l *link) error {
	switch {
	case l == nil:
		return linkError(CodeLinkInvalid)
	case l.AcceptedAt != nil:
		return linkError(CodeLinkUsed)
	case l.SupersededAt != nil || l.Expired:
		return linkError(CodeLinkExpired)
	}
	return nil
}

// teacher returns the name of the inviting teacher while they are still an active private teacher.
func (s *Service) teacher(ctx context.Context, q database.Querier, id uuid.UUID) (string, error) {
	name, role, _, ok, err := s.accounts.ActiveProfile(ctx, q, id)
	if err != nil {
		return "", err
	}
	if !ok || role != auth.RolePrivateTeacher {
		return "", linkError(CodeLinkInvalid)
	}
	return name, nil
}

// Verify tells the page whether the link still works, who invites and whether to sign in or create an account.
// It changes nothing.
func (s *Service) Verify(ctx context.Context, token string) (*LinkInfo, error) {
	if !validToken(token) {
		return nil, linkError(CodeLinkInvalid)
	}
	l, err := s.repo.LinkByHash(ctx, tokenHash(token))
	if err != nil {
		return nil, err
	}
	if err := state(l); err != nil {
		return nil, err
	}
	name, err := s.teacher(ctx, s.repo.Pool(), l.TeacherID)
	if err != nil {
		return nil, err
	}
	taken, err := s.accounts.EmailTaken(ctx, s.repo.Pool(), l.Email)
	if err != nil {
		return nil, err
	}
	return &LinkInfo{TeacherName: name, Email: l.Email, ExpiresAt: l.ExpiresAt, HasAccount: taken}, nil
}

// accept links the student and closes the link (inside the transaction, the link row locked).
func (s *Service) accept(ctx context.Context, tx pgx.Tx, l *link, studentID uuid.UUID) error {
	if err := s.accounts.LinkInvitedStudent(ctx, tx, l.TeacherID, studentID, l.ID); err != nil {
		return err
	}
	return s.repo.MarkAccepted(ctx, tx, l, studentID)
}

// Accept is the link opened by a signed-in user. It works only for a student account with the invited address
// (signing in proves the address), so an existing student keeps one account. Accepting again is harmless.
func (s *Service) Accept(ctx context.Context, p auth.Principal, token string) (*Accepted, error) {
	if !validToken(token) {
		return nil, linkError(CodeLinkInvalid)
	}
	var out *Accepted
	err := database.WithTx(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		l, err := s.repo.LinkForUpdate(ctx, tx, tokenHash(token))
		if err != nil {
			return err
		}
		if l != nil && l.AcceptedAt != nil && l.AcceptedBy != nil && *l.AcceptedBy == p.UserID {
			name, _, _, _, err := s.accounts.ActiveProfile(ctx, tx, l.TeacherID)
			out = &Accepted{TeacherName: name}
			return err
		}
		if err := state(l); err != nil {
			return err
		}
		name, err := s.teacher(ctx, tx, l.TeacherID)
		if err != nil {
			return err
		}
		if !strings.EqualFold(p.Email, l.Email) {
			e := apperr.New(http.StatusForbidden, CodeWrongAccount, "Приглашение отправлено на другой адрес. Войдите под учётной записью, на которую пришло письмо.")
			e.Fields = map[string]string{"email": maskEmail(l.Email)}
			return e
		}
		if !p.IsStudent() {
			return apperr.New(http.StatusForbidden, CodeNotStudent, "Принять приглашение может только аккаунт ученика.")
		}
		if err := s.accept(ctx, tx, l, p.UserID); err != nil {
			return err
		}
		out = &Accepted{TeacherName: name}
		return nil
	})
	return out, err
}

// Register creates a student account for an address that has none: the link proves the address. In one
// transaction the account is created, linked to the teacher and the link is closed; then the student signs in.
// An address that already has an account must sign in instead (no duplicate account).
func (s *Service) Register(ctx context.Context, req RegisterRequest, ip, ua string) (string, auth.Principal, error) {
	if !validToken(req.Token) {
		return "", auth.Principal{}, linkError(CodeLinkInvalid)
	}
	hash := tokenHash(req.Token)
	// The link is checked before the form: an old link answers about the link.
	l, err := s.repo.LinkByHash(ctx, hash)
	if err != nil {
		return "", auth.Principal{}, err
	}
	if err := state(l); err != nil {
		return "", auth.Principal{}, err
	}
	name := strings.Join(strings.Fields(req.FullName), " ")
	v := apperr.V{}
	v.Check(name != "" && len([]rune(name)) <= 120, "full_name", "Укажите имя и фамилию")
	if msg := auth.ValidatePassword(req.Password); msg != "" {
		v.Add("password", msg)
	} else if strings.EqualFold(strings.TrimSpace(req.Password), l.Email) {
		v.Add("password", "Пароль не должен совпадать с email")
	}
	v.Check(req.AcceptTerms, "accept_terms", "Нужно принять условия использования и политику конфиденциальности")
	if err := v.Err(); err != nil {
		return "", auth.Principal{}, err
	}
	pwHash, err := auth.HashPassword(req.Password)
	if err != nil {
		return "", auth.Principal{}, err
	}
	exists := apperr.New(http.StatusConflict, CodeAccountExists, "У этого адреса уже есть аккаунт. Войдите, чтобы принять приглашение.")
	var userID uuid.UUID
	err = database.WithTx(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		l, err := s.repo.LinkForUpdate(ctx, tx, hash)
		if err != nil {
			return err
		}
		if err := state(l); err != nil {
			return err
		}
		if _, err := s.teacher(ctx, tx, l.TeacherID); err != nil {
			return err
		}
		taken, err := s.accounts.EmailTaken(ctx, tx, l.Email)
		if err != nil {
			return err
		}
		if taken {
			return exists
		}
		userID, err = s.accounts.CreateSelfRegistered(ctx, tx, l.Email, name, auth.RoleStudent, pwHash, s.color(l.Email), time.Now())
		if database.IsUniqueViolation(err, "") { // an account with the address was created right now
			return exists
		}
		if err != nil {
			return err
		}
		return s.accept(ctx, tx, l, userID)
	})
	if err != nil {
		return "", auth.Principal{}, err
	}
	return s.sessions.StartSession(ctx, userID, ip, ua)
}
