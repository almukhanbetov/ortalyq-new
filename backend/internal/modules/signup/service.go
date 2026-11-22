package signup

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math"
	"net/http"
	netmail "net/mail"
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
// and sent after the commit.
type Mailer interface {
	Enqueue(ctx context.Context, tx pgx.Tx, kind string, refID uuid.UUID, locale string, m mail.Message) (uuid.UUID, error)
	Kick()
}

// Accounts is satisfied by users.Repository.
type Accounts interface {
	EmailTaken(ctx context.Context, q database.Querier, email string) (bool, error)
	CreateSelfRegistered(ctx context.Context, q database.Querier, email, name, role, hash, color string, termsAt time.Time) (uuid.UUID, error)
}

// Sessions is satisfied by auth.Service.
type Sessions interface {
	StartSession(ctx context.Context, userID uuid.UUID, ip, ua string) (string, auth.Principal, error)
}

type Service struct {
	repo     *Repository
	accounts Accounts
	sessions Sessions
	mailer   Mailer
	color    func(email string) string
	appURL   string
	opt      Options
}

func NewService(repo *Repository, accounts Accounts, sessions Sessions, mailer Mailer, color func(string) string, appURL string, opt Options) *Service {
	return &Service{repo: repo, accounts: accounts, sessions: sessions, mailer: mailer, color: color,
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

// NormalizeEmail trims and lower-cases the address and checks it is a bare address (no display name), at most
// 254 characters, with a dot in the domain. "" means invalid.
func NormalizeEmail(raw string) string {
	e := strings.ToLower(strings.TrimSpace(raw))
	if e == "" || len(e) > 254 || strings.ContainsAny(e, " \t\r\n<>,;\"") {
		return ""
	}
	a, err := netmail.ParseAddress(e)
	if err != nil || a.Address != e || a.Name != "" {
		return ""
	}
	at := strings.LastIndex(e, "@")
	local, domain := e[:at], e[at+1:]
	if local == "" || len(local) > 64 || !strings.Contains(domain, ".") || strings.HasPrefix(domain, ".") ||
		strings.HasSuffix(domain, ".") || strings.Contains(domain, "..") {
		return ""
	}
	return e
}

func validLocale(l string) bool { return l == "ru" || l == "kk" || l == "en" }

func tooMany(wait float64) error {
	sec := int(math.Ceil(wait))
	if sec < 1 {
		sec = 1
	}
	e := apperr.TooMany(fmt.Sprintf("Письмо уже отправлено. Запросить новое можно через %d с.", sec))
	e.Fields = map[string]string{"retry_after": fmt.Sprint(sec)}
	return e
}

// RequestEmail is the first step. A new address gets a one-time link; an address that already has an account gets
// a letter that only suggests signing in, and the account is not touched. The answer is the same in both cases.
// Limits (per address and per IP) apply to both alike, so they do not reveal whether an account exists either.
func (s *Service) RequestEmail(ctx context.Context, req RequestEmail, ip string) (*RequestResponse, error) {
	email := NormalizeEmail(req.Email)
	v := apperr.V{}
	v.Check(email != "", "email", "Введите корректный email")
	v.Check(req.AcceptTerms, "accept_terms", "Примите условия использования и политику конфиденциальности")
	if err := v.Err(); err != nil {
		return nil, err
	}
	locale := req.Locale
	if !validLocale(locale) {
		locale = "ru"
	}
	ip = truncate(ip, 64)
	err := database.WithTx(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		if err := s.repo.LockEmail(ctx, tx, email); err != nil {
			return err
		}
		u, err := s.repo.Usage(ctx, tx, email, ip, limitWindow, s.opt.ResendAfter)
		if err != nil {
			return err
		}
		switch {
		case u.WaitResend > 0:
			return tooMany(u.WaitResend)
		case u.ByEmail >= s.opt.MaxPerEmail:
			return tooMany(u.WaitEmailWindow)
		case u.ByIP >= s.opt.MaxPerIP:
			return tooMany(u.WaitIPWindow)
		}
		taken, err := s.accounts.EmailTaken(ctx, tx, email)
		if err != nil {
			return err
		}
		if err := s.repo.Supersede(ctx, tx, email); err != nil {
			return err
		}
		var (
			hash []byte
			msg  mail.Message
			kind = "signup_existing"
		)
		token := ""
		if !taken {
			if token, hash, err = newToken(); err != nil {
				return err
			}
			kind = "signup_confirm"
		}
		id, _, err := s.repo.Insert(ctx, tx, email, locale, hash, ip, s.opt.LinkTTL)
		if err != nil {
			return err
		}
		if taken {
			msg = mail.SignupExisting(locale, mail.SignupData{To: email, Link: s.appURL + "/login"})
		} else {
			// The token travels in the fragment: browsers do not send it to servers or in Referer.
			msg = mail.SignupConfirm(locale, mail.SignupData{To: email, Link: s.appURL + "/signup/confirm#" + token,
				Hours: int(s.opt.LinkTTL.Hours())})
		}
		oid, err := s.mailer.Enqueue(ctx, tx, kind, id, locale, msg)
		if err != nil {
			return err
		}
		return s.repo.SetOutbox(ctx, tx, id, oid)
	})
	if err != nil {
		return nil, err
	}
	s.mailer.Kick()
	return &RequestResponse{ResendAfter: int(s.opt.ResendAfter.Seconds())}, nil
}

func linkError(code string) error {
	switch code {
	case CodeLinkExpired:
		return apperr.New(http.StatusGone, code, "Срок действия ссылки истёк. Запросите новое письмо.")
	case CodeLinkUsed:
		return apperr.New(http.StatusGone, code, "Ссылка уже использована. Войдите в аккаунт.")
	}
	return apperr.New(http.StatusNotFound, CodeLinkInvalid, "Ссылка недействительна. Запросите новое письмо.")
}

// state checks a link: nil when it can still be used. A newer letter to the same address makes the link expired.
func state(l *link) error {
	switch {
	case l == nil:
		return linkError(CodeLinkInvalid)
	case l.UsedAt != nil:
		return linkError(CodeLinkUsed)
	case l.SupersededAt != nil || l.Expired:
		return linkError(CodeLinkExpired)
	}
	return nil
}

func validToken(t string) bool {
	return len(t) == 43 && strings.Trim(t, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_") == ""
}

// Verify is the confirmation link opened: the address is confirmed and the password form may be shown. Opening it
// again before the password is set is harmless; the link stays one-time for creating the account.
func (s *Service) Verify(ctx context.Context, token string) (*LinkInfo, error) {
	if !validToken(token) {
		return nil, linkError(CodeLinkInvalid)
	}
	l, err := s.repo.LinkByHash(ctx, s.repo.Pool(), tokenHash(token))
	if err != nil {
		return nil, err
	}
	if err := state(l); err != nil {
		return nil, err
	}
	// The address may have got an account in the meantime (e.g. created by an administrator).
	if taken, err := s.accounts.EmailTaken(ctx, s.repo.Pool(), l.Email); err != nil {
		return nil, err
	} else if taken {
		return nil, linkError(CodeLinkUsed)
	}
	if err := s.repo.MarkVerified(ctx, l.ID); err != nil {
		return nil, err
	}
	return &LinkInfo{Email: l.Email, ExpiresAt: l.ExpiresAt}, nil
}

// defaultName is the display name until the profile step exists: the part of the address before "@".
func defaultName(email string) string {
	n := email
	if i := strings.Index(n, "@"); i > 0 {
		n = n[:i]
	}
	if r := []rune(n); len(r) > 120 {
		n = string(r[:120])
	}
	return n
}

// Complete sets the password and creates the account with the role chosen by the server (private teacher), then
// signs in. The link is closed in the same transaction, so a second request with it creates nothing. An address
// that already has an account is never changed: the link is closed and the answer is "already used".
func (s *Service) Complete(ctx context.Context, req CompleteRequest, ip, ua string) (string, auth.Principal, error) {
	if !validToken(req.Token) {
		return "", auth.Principal{}, linkError(CodeLinkInvalid)
	}
	hash := tokenHash(req.Token)
	// The link is checked before the password: an old link answers about the link, not about the form.
	l, err := s.repo.LinkByHash(ctx, s.repo.Pool(), hash)
	if err != nil {
		return "", auth.Principal{}, err
	}
	if err := state(l); err != nil {
		return "", auth.Principal{}, err
	}
	if msg := auth.ValidatePassword(req.Password); msg != "" {
		return "", auth.Principal{}, apperr.Validation(map[string]string{"password": msg})
	}
	if strings.EqualFold(strings.TrimSpace(req.Password), l.Email) {
		return "", auth.Principal{}, apperr.Validation(map[string]string{"password": "Пароль не должен совпадать с email"})
	}
	pwHash, err := auth.HashPassword(req.Password)
	if err != nil {
		return "", auth.Principal{}, err
	}

	var userID uuid.UUID
	closed := false // the address already had an account: the link is closed, nothing else changes
	err = database.WithTx(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		l, err := s.repo.LinkForUpdate(ctx, tx, hash)
		if err != nil {
			return err
		}
		if err := state(l); err != nil {
			return err
		}
		taken, err := s.accounts.EmailTaken(ctx, tx, l.Email)
		if err != nil {
			return err
		}
		if taken {
			closed = true
			return s.repo.MarkUsed(ctx, tx, l.ID, nil)
		}
		// The unique index on lower(email) still guards a race with another account being created right now: the
		// statement fails inside its own savepoint so the transaction can close the link.
		sp, err := tx.Begin(ctx)
		if err != nil {
			return err
		}
		userID, err = s.accounts.CreateSelfRegistered(ctx, sp, l.Email, defaultName(l.Email), auth.RolePrivateTeacher, pwHash,
			s.color(l.Email), l.TermsAt)
		if database.IsUniqueViolation(err, "") {
			_ = sp.Rollback(ctx)
			closed = true
			return s.repo.MarkUsed(ctx, tx, l.ID, nil)
		}
		if err != nil {
			_ = sp.Rollback(ctx)
			return err
		}
		if err := sp.Commit(ctx); err != nil {
			return err
		}
		return s.repo.MarkUsed(ctx, tx, l.ID, &userID)
	})
	if err != nil {
		return "", auth.Principal{}, err
	}
	if closed {
		return "", auth.Principal{}, linkError(CodeLinkUsed)
	}
	return s.sessions.StartSession(ctx, userID, ip, ua)
}

func (s *Service) Cleanup(ctx context.Context) error { return s.repo.DeleteOld(ctx, keepRows) }

func truncate(v string, n int) string {
	if len(v) > n {
		return v[:n]
	}
	return v
}
