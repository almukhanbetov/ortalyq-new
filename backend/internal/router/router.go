// Package router wires all modules into the Gin engine.
package router

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"ortalyq/backend/internal/config"
	"ortalyq/backend/internal/mail"
	"ortalyq/backend/internal/middleware"
	"ortalyq/backend/internal/modules/attempts"
	"ortalyq/backend/internal/modules/auth"
	"ortalyq/backend/internal/modules/boards"
	"ortalyq/backend/internal/modules/chat"
	"ortalyq/backend/internal/modules/classrooms"
	"ortalyq/backend/internal/modules/exercises"
	"ortalyq/backend/internal/modules/grading"
	"ortalyq/backend/internal/modules/homework"
	"ortalyq/backend/internal/modules/library"
	"ortalyq/backend/internal/modules/marathons"
	"ortalyq/backend/internal/modules/materials"
	"ortalyq/backend/internal/modules/passwordreset"
	"ortalyq/backend/internal/modules/signup"
	"ortalyq/backend/internal/modules/studentinvites"
	"ortalyq/backend/internal/modules/uploads"
	"ortalyq/backend/internal/modules/users"
	"ortalyq/backend/internal/realtime"
)

type App struct {
	Engine *gin.Engine
	Auth   *auth.Service
	// Mail sends the outbox after commits (Run in serve; tests call DispatchOnce).
	Mail *mail.Dispatcher
	// Signup is the private teacher sign-up (Cleanup in serve).
	Signup *signup.Service
	// PasswordReset restores access by e-mail (Cleanup in serve).
	PasswordReset *passwordreset.Service
}

// New constructs every module with explicit dependencies.
func New(cfg config.Config, pool *pgxpool.Pool) (*App, error) {
	if !cfg.IsLocal() {
		gin.SetMode(gin.ReleaseMode)
	}
	hub := realtime.NewHub(cfg.AllowedOrigins)

	authRepo := auth.NewRepository(pool)
	authSvc := auth.NewService(authRepo, auth.Options{
		SessionTTL: cfg.SessionTTL, MaxPerEmail: cfg.LoginMaxPerEmail, MaxPerIP: cfg.LoginMaxPerIP, FailureWindow: cfg.LoginWindow,
	})
	authH := auth.NewHandler(authSvc, auth.CookieConfig{
		Name: cfg.CookieName, Secure: cfg.CookieSecure, Domain: cfg.CookieDomain, MaxAge: int(cfg.SessionTTL.Seconds()),
	})

	usersSvc := users.NewService(users.NewRepository(pool), authRepo)
	classSvc := classrooms.NewService(classrooms.NewRepository(pool), usersSvc, hub)
	matSvc := materials.NewService(materials.NewRepository(pool), classSvc)
	classSvc.SetLessonOwnership(matSvc)

	exSvc := exercises.NewService(exercises.NewRepository(pool), matSvc, classSvc)
	hwRepo := homework.NewRepository(pool)
	marRepo := marathons.NewRepository(pool)
	// marRepo implements attempts.LessonReviewState: answers of a marathon lesson on the teacher's review are locked.
	attSvc := attempts.NewService(attempts.NewRepository(pool), exSvc, classSvc, hwRepo, marRepo, matSvc, hub)
	hwSvc := homework.NewService(hwRepo, classSvc, attSvc, hub)
	gradeSvc := grading.NewService(grading.NewRepository(pool), classSvc, matSvc, attSvc, hub)

	upSvc, err := uploads.NewService(uploads.NewRepository(pool), classSvc, cfg.UploadDir, cfg.UploadMaxBytes)
	if err != nil {
		return nil, err
	}
	chatSvc := chat.NewService(chat.NewRepository(pool), classSvc, upSvc, chat.NewGiphy(cfg.GiphyAPIKey, cfg.GiphyAPIBase), hub)

	r := gin.New()
	// Behind a reverse proxy the client IP (login rate limits) comes from X-Forwarded-For of trusted proxies only.
	if len(cfg.TrustedProxies) > 0 {
		if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
			return nil, fmt.Errorf("TRUSTED_PROXIES: %w", err)
		}
	}
	r.Use(middleware.Recovery(), middleware.Logger(), middleware.SecurityHeaders(),
		middleware.CORS(cfg.AllowedOrigins), middleware.OriginGuard(cfg.AllowedOrigins))
	r.MaxMultipartMemory = 8 << 20

	r.GET("/healthz", func(c *gin.Context) {
		if err := pool.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "db_unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api", authH.LoadSession())
	authed := api.Group("", auth.RequireAuth())
	authH.Register(api, authed)
	users.NewHandler(usersSvc).Register(authed)
	classrooms.NewHandler(classSvc, hub).Register(authed)
	materials.NewHandler(matSvc).Register(authed)
	library.NewHandler(library.NewService(library.NewRepository(pool))).Register(authed)
	boards.NewHandler(boards.NewService(boards.NewRepository(pool), upSvc, classSvc, hub)).Register(authed)
	exercises.NewHandler(exSvc).Register(authed)
	attempts.NewHandler(attSvc).Register(authed)
	homework.NewHandler(hwSvc).Register(authed)
	grading.NewHandler(gradeSvc).Register(authed)
	uploads.NewHandler(upSvc).Register(authed)
	chat.NewHandler(chatSvc).Register(authed)
	marSvc := marathons.NewService(marRepo, classSvc, matSvc, matSvc, attSvc, usersSvc, hub)
	classSvc.SetMarathonGate(marSvc)
	sender, err := mail.NewSender(mail.Config{Driver: cfg.MailDriver, SMTPAddr: cfg.MailSMTPAddr, From: cfg.MailFrom, Username: cfg.MailUser, Password: cfg.MailPassword})
	if err != nil {
		return nil, err
	}
	mailer := mail.NewDispatcher(pool, sender)
	marSvc.SetInvitations(mailer, usersSvc, cfg.AppURL)
	marSvc.SetImages(upSvc)
	marH := marathons.NewHandler(marSvc)
	marH.Register(authed)
	marH.RegisterPublic(api)

	// Self sign-up of a private teacher: letters through the same outbox, the role is set by the server.
	usersRepo := users.NewRepository(pool)
	signupSvc := signup.NewService(signup.NewRepository(pool), usersRepo, authSvc, mailer, users.AvatarColor, cfg.AppURL, signup.Options{
		LinkTTL: cfg.SignupLinkTTL, ResendAfter: cfg.SignupResendAfter, MaxPerEmail: cfg.SignupMaxPerEmail, MaxPerIP: cfg.SignupMaxPerIP,
	})
	signup.NewHandler(signupSvc, authH.SetSessionCookie).Register(api)

	// Password reset by e-mail: letters through the same outbox; a new password ends every session of the account.
	resetSvc := passwordreset.NewService(passwordreset.NewRepository(pool), usersRepo, authSvc, mailer, signup.NormalizeEmail, cfg.AppURL,
		passwordreset.Options{LinkTTL: cfg.ResetLinkTTL, ResendAfter: cfg.ResetResendAfter, MaxPerEmail: cfg.ResetMaxPerEmail, MaxPerIP: cfg.ResetMaxPerIP})
	passwordreset.NewHandler(resetSvc).Register(api)

	// A private teacher invites students by e-mail; an accepted invitation links the student (teacher_students).
	inviteSvc := studentinvites.NewService(studentinvites.NewRepository(pool), usersRepo, authSvc, mailer, signup.NormalizeEmail,
		users.AvatarColor, cfg.AppURL, studentinvites.Options{})
	studentinvites.NewHandler(inviteSvc, authH.SetSessionCookie).Register(api, authed)

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "Маршрут не найден"}})
	})
	return &App{Engine: r, Auth: authSvc, Mail: mailer, Signup: signupSvc, PasswordReset: resetSvc}, nil
}
