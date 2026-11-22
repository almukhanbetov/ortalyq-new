package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"ortalyq/backend/internal/apperr"
	"ortalyq/backend/internal/database"
)

const bcryptCost = 12

// dummyHash makes login timing similar whether or not the user exists.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("timing-equaliser"), bcryptCost)

func HashPassword(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcryptCost)
	return string(h), err
}

// ValidatePassword enforces the minimal password policy.
func ValidatePassword(pw string) string {
	if len(pw) < 8 {
		return "Пароль должен содержать не менее 8 символов"
	}
	if len(pw) > 72 {
		return "Пароль слишком длинный"
	}
	return ""
}

type Options struct {
	SessionTTL    time.Duration
	MaxPerEmail   int
	MaxPerIP      int
	FailureWindow time.Duration
}

type Service struct {
	repo *Repository
	opt  Options
	now  func() time.Time
}

func NewService(repo *Repository, opt Options) *Service {
	return &Service{repo: repo, opt: opt, now: time.Now}
}

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// Login verifies credentials and returns a new opaque session token.
func (s *Service) Login(ctx context.Context, email, password, ip, ua string) (string, Principal, error) {
	email = strings.TrimSpace(email)
	v := apperr.V{}
	v.Check(email != "", "email", "Введите email")
	v.Check(password != "", "password", "Введите пароль")
	if err := v.Err(); err != nil {
		return "", Principal{}, err
	}
	byEmail, byIP, err := s.repo.CountFailures(ctx, email, ip, s.now().Add(-s.opt.FailureWindow))
	if err != nil {
		return "", Principal{}, err
	}
	if byEmail >= s.opt.MaxPerEmail || byIP >= s.opt.MaxPerIP {
		return "", Principal{}, apperr.TooMany("Слишком много попыток входа. Попробуйте позже.")
	}
	u, err := s.repo.UserByEmail(ctx, email)
	if err != nil && !database.IsNoRows(err) {
		return "", Principal{}, err
	}
	hash := dummyHash
	if u != nil {
		hash = []byte(u.PasswordHash)
	}
	pwErr := bcrypt.CompareHashAndPassword(hash, []byte(password))
	if u == nil || pwErr != nil || u.DisabledAt != nil {
		_ = s.repo.RecordAttempt(ctx, email, ip, false)
		return "", Principal{}, apperr.New(401, "invalid_credentials", "Неверный email или пароль")
	}
	_ = s.repo.ClearFailures(ctx, email)
	_ = s.repo.RecordAttempt(ctx, email, ip, true)

	token, err := s.newSession(ctx, u.UserID, ip, ua)
	if err != nil {
		return "", Principal{}, err
	}
	return token, u.Principal, nil
}

func (s *Service) newSession(ctx context.Context, userID uuid.UUID, ip, ua string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	if err := s.repo.CreateSession(ctx, hashToken(token), userID, s.now().Add(s.opt.SessionTTL), truncate(ua, 300), ip); err != nil {
		return "", err
	}
	return token, nil
}

// StartSession signs in a user that has just proved control of the address in another way (the sign-up link
// followed by a new password). Disabled accounts get no session.
func (s *Service) StartSession(ctx context.Context, userID uuid.UUID, ip, ua string) (string, Principal, error) {
	token, err := s.newSession(ctx, userID, ip, ua)
	if err != nil {
		return "", Principal{}, err
	}
	p, err := s.Authenticate(ctx, token)
	if err != nil {
		_ = s.repo.DeleteSession(ctx, hashToken(token))
		return "", Principal{}, err
	}
	return token, p, nil
}

// Authenticate resolves a session token into a principal (sliding expiration).
func (s *Service) Authenticate(ctx context.Context, token string) (Principal, error) {
	if token == "" || len(token) > 128 {
		return Principal{}, apperr.Unauthorized()
	}
	h := hashToken(token)
	row, err := s.repo.SessionPrincipal(ctx, h)
	if err != nil {
		if database.IsNoRows(err) {
			return Principal{}, apperr.Unauthorized()
		}
		return Principal{}, err
	}
	if s.now().Sub(row.LastSeenAt) > time.Hour {
		_ = s.repo.TouchSession(ctx, h, s.now().Add(s.opt.SessionTTL))
	}
	return row.Principal, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.repo.DeleteSession(ctx, hashToken(token))
}

func (s *Service) UpdatePrefs(ctx context.Context, p Principal, req UpdateMeRequest) error {
	v := apperr.V{}
	if req.Locale != nil {
		v.Check(*req.Locale == "ru" || *req.Locale == "kk" || *req.Locale == "en", "locale", "Неподдерживаемый язык")
	}
	if req.Timezone != nil {
		_, err := time.LoadLocation(*req.Timezone)
		v.Check(err == nil && *req.Timezone != "", "timezone", "Неизвестный часовой пояс")
	}
	if err := v.Err(); err != nil {
		return err
	}
	return s.repo.UpdatePrefs(ctx, p.UserID, req.Locale, req.Timezone)
}

func (s *Service) Cleanup(ctx context.Context) error { return s.repo.DeleteExpired(ctx) }

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
