package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"ortalyq/backend/internal/apperr"
	"ortalyq/backend/internal/config"
	"ortalyq/backend/internal/database"
	"ortalyq/backend/internal/modules/auth"
	"ortalyq/backend/internal/modules/users"
)

// createAdmin creates an administrator account (the first one on a fresh production database,
// where seed is refused). The password is read from the first line of stdin, never from arguments
// or the environment, so it does not end up in shell history or `docker inspect`. An existing
// email is an error: the command never changes the password of an existing account.
func createAdmin(cfg config.Config, args []string, stdin io.Reader) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: api create-admin <email> <full name>  (password on stdin)")
	}
	email, name := args[0], strings.Join(args[1:], " ")
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("read password: %w", err)
	}
	password := strings.TrimRight(line, "\r\n")

	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	svc := users.NewService(users.NewRepository(pool), auth.NewRepository(pool))
	u, err := svc.Create(ctx, users.CreateUserRequest{Email: email, FullName: name, Role: auth.RoleAdmin, Password: password})
	if err != nil {
		var ae *apperr.Error
		if errors.As(err, &ae) && len(ae.Fields) > 0 {
			var parts []string
			for f, m := range ae.Fields {
				parts = append(parts, f+": "+m)
			}
			return fmt.Errorf("%s", strings.Join(parts, "; "))
		}
		return err
	}
	fmt.Printf("administrator created: id=%s\n", u.ID)
	return nil
}
