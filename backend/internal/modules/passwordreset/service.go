package passwordreset

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math"
	"net/http"
	"strings"

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
	// ActiveAccountByEmail returns the id of the active (not disabled) account with the address; ok is false when
	// there is none.
	ActiveAccountByEmail(ctx context.Context, q database.Querier, email string) (id uuid.UUID, ok bool, err error)
	// ActiveAccountByID returns the address of the active account; ok is false when it is gone or disabled.
	ActiveAccountByID(ctx context.Context, q database.Querier, id uuid.UUID) (email string, ok bool, err error)
	// SetPassword replaces only the password hash: id, role and rights stay as they are.
	SetPassword(ctx context.Context, q database.Querier, id uuid.UUID, hash string) error
}

// Sessions is satisfied by auth.Service.
type Sessions interface {
	// RevokeAfterPasswordChange ends every session of the user and clears failed sign-in attempts of the address.
	RevokeAfterPasswordChange(ctx context.Context, q database.Querier, userID uuid.UUID, email string) error
}

type Service struct {
	repo      *Repository
	accounts  Accounts
	sessions  Sessions
	mailer    Mailer
	normalize func(string) string
	appURL    string
	opt       Options
}

// normalize checks and lower-cases an address ("" when invalid); the sign-up flow uses the same rule.
func NewService(repo *Repository, accounts Accounts, sessions Sessions, mailer Mailer, normalize func(string) string, appURL string, opt Options) *Service {
	return &Service{repo: repo, accounts: accounts, sessions: sessions, mailer: mailer, normalize: normalize,
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
	e := apperr.TooMany(fmt.Sprintf("Письмо уже отправлено. Запросить новое можно через %d с.", sec))
	e.Fields = map[string]string{"retry_after": fmt.Sprint(sec)}
	return e
}

// Request is the first step. An active account gets a letter with a one-time link; an unknown or disabled address
// gets nothing. The answer, and the limits (per address and per IP), are the same in both cases, so the screen
// never reveals whether an account exists.
func (s *Service) Request(ctx context.Context, req RequestReset, ip string) (*RequestResponse, error) {
	email := s.normalize(req.Email)
	if email == "" {
		return nil, apperr.Validation(map[string]string{"email": "Введите корректный email"})
	}
	locale := req.Locale
	if !validLocale(locale) {
		locale = "ru"
	}
	ip = truncate(ip, 64)
	sent := false
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
		userID, ok, err := s.accounts.ActiveAccountByEmail(ctx, tx, email)
		if err != nil {
			return err
		}
		if !ok {
			_, err := s.repo.Insert(ctx, tx, email, locale, nil, nil, ip, s.opt.LinkTTL)
			return err
		}
		if err := s.repo.Supersede(ctx, tx, userID); err != nil {
			return err
		}
		token, hash, err := newToken()
		if err != nil {
			return err
		}
		id, err := s.repo.Insert(ctx, tx, email, locale, &userID, hash, ip, s.opt.LinkTTL)
		if err != nil {
			return err
		}
		// The token travels in the fragment: browsers do not send it to servers or in Referer.
		msg := mail.PasswordReset(locale, mail.PasswordResetData{To: email, Link: s.appURL + "/reset-password#" + token,
			Minutes: int(s.opt.LinkTTL.Minutes())})
		oid, err := s.mailer.Enqueue(ctx, tx, "password_reset", id, locale, msg)
		if err != nil {
			return err
		}
		sent = true
		return s.repo.SetOutbox(ctx, tx, id, oid)
	})
	if err != nil {
		return nil, err
	}
	if sent {
		s.mailer.Kick()
	}
	return &RequestResponse{ResendAfter: int(s.opt.ResendAfter.Seconds())}, nil
}

func linkError(code string) error {
	switch code {
	case CodeLinkExpired:
		return apperr.New(http.StatusGone, code, "Срок действия ссылки истёк. Запросите новое письмо.")
	case CodeLinkUsed:
		return apperr.New(http.StatusGone, code, "Ссылка уже использована. Войдите с новым паролем.")
	}
	return apperr.New(http.StatusNotFound, CodeLinkInvalid, "Ссылка недействительна. Запросите новое письмо.")
}

// state checks a link: nil when it can still be used. A newer letter makes the older link expired.
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

// Verify tells the form whether the link still works and for which address. It changes nothing.
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
	if _, ok, err := s.accounts.ActiveAccountByID(ctx, s.repo.Pool(), l.UserID); err != nil {
		return nil, err
	} else if !ok {
		return nil, linkError(CodeLinkInvalid)
	}
	return &LinkInfo{Email: l.Email, ExpiresAt: l.ExpiresAt}, nil
}

// Complete sets the new password. In one transaction: the link is locked and checked, the password hash is
// replaced (the account keeps its id, role and rights), the link and the other open links of the account are closed
// and every session of the account ends. The user then signs in with the new password.
func (s *Service) Complete(ctx context.Context, req CompleteRequest) error {
	if !validToken(req.Token) {
		return linkError(CodeLinkInvalid)
	}
	hash := tokenHash(req.Token)
	// The link is checked before the password: an old link answers about the link, not about the form.
	l, err := s.repo.LinkByHash(ctx, hash)
	if err != nil {
		return err
	}
	if err := state(l); err != nil {
		return err
	}
	if msg := auth.ValidatePassword(req.Password); msg != "" {
		return apperr.Validation(map[string]string{"password": msg})
	}
	if strings.EqualFold(strings.TrimSpace(req.Password), l.Email) {
		return apperr.Validation(map[string]string{"password": "Пароль не должен совпадать с email"})
	}
	pwHash, err := auth.HashPassword(req.Password)
	if err != nil {
		return err
	}
	return database.WithTx(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		l, err := s.repo.LinkForUpdate(ctx, tx, hash)
		if err != nil {
			return err
		}
		if err := state(l); err != nil {
			return err
		}
		email, ok, err := s.accounts.ActiveAccountByID(ctx, tx, l.UserID)
		if err != nil {
			return err
		}
		if !ok {
			return linkError(CodeLinkInvalid)
		}
		if err := s.accounts.SetPassword(ctx, tx, l.UserID, pwHash); err != nil {
			return err
		}
		if err := s.repo.MarkUsed(ctx, tx, l.ID, l.UserID); err != nil {
			return err
		}
		return s.sessions.RevokeAfterPasswordChange(ctx, tx, l.UserID, email)
	})
}

func (s *Service) Cleanup(ctx context.Context) error { return s.repo.DeleteOld(ctx, keepRows) }

func truncate(v string, n int) string {
	if len(v) > n {
		return v[:n]
	}
	return v
}
