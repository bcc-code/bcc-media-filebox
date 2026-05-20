package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"strings"

	db "filebox/internal/db/gen"
)

// BootstrapAdminFromEnv creates an admin grant for `email` when the users
// table is empty. Intended to be called once at startup so a fresh deploy has
// a usable admin without manual SQL.
//
// The grant is `(principal_kind='user', principal_value=<email>, admin=1,
// all_targets=1)`. The matching users row is created on first sign-in; the
// grant takes effect via RecomputeRoleForUser at that point.
//
// Behavior:
//   - email empty           → no-op
//   - users table non-empty → no-op (operators should use the admin UI)
//   - matching grant exists → no-op (idempotent across restarts)
//   - otherwise             → insert grant
//
// Guests can never be admins (see computeRoleFor), so this is only useful
// for emails that will sign in via OIDC.
func BootstrapAdminFromEnv(ctx context.Context, queries *db.Queries, email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil
	}
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return fmt.Errorf("BOOTSTRAP_ADMIN_EMAIL %q is not a valid email: %w", email, err)
	}
	email = strings.ToLower(addr.Address)

	count, err := queries.CountUsers(ctx)
	if err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	if count > 0 {
		log.Printf("BOOTSTRAP_ADMIN_EMAIL ignored: %d user(s) already exist", count)
		return nil
	}

	if _, err := queries.GetGrantByPrincipal(ctx, db.GetGrantByPrincipalParams{
		PrincipalKind:  "user",
		PrincipalValue: email,
	}); err == nil {
		log.Printf("BOOTSTRAP_ADMIN_EMAIL: grant for %s already exists, skipping", email)
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("lookup grant: %w", err)
	}

	if _, err := queries.CreateGrant(ctx, db.CreateGrantParams{
		PrincipalKind:  "user",
		PrincipalValue: email,
		Admin:          1,
		AllTargets:     1,
	}); err != nil {
		return fmt.Errorf("create admin grant: %w", err)
	}
	log.Printf("BOOTSTRAP_ADMIN_EMAIL: seeded admin grant for %s", email)
	return nil
}
