package auth

import (
	"context"

	db "filebox/internal/db/gen"
)

// RecomputeUserRoles walks every non-guest user and sets users.role to either
// "admin" or "uploader" based on whether any matching grant has admin=1.
// Called on every grant/group mutation and on sign-in. Cheap at FileBox scale.
//
// Guests (provider='guest') keep whatever role they had — they don't appear
// in grants, and surfacing them in the Users tab with role='guest' is the
// design's intent.
func RecomputeUserRoles(ctx context.Context, queries *db.Queries) error {
	users, err := queries.ListUsers(ctx)
	if err != nil {
		return err
	}
	for _, u := range users {
		if u.Provider == "guest" {
			continue
		}
		if !u.Email.Valid || u.Email.String == "" {
			continue
		}
		role, err := computeRoleFor(ctx, queries, u.Email.String, u.Provider)
		if err != nil {
			return err
		}
		if role != u.Role {
			if err := queries.SetUserRole(ctx, db.SetUserRoleParams{Role: role, ID: u.ID}); err != nil {
				return err
			}
		}
	}
	return nil
}

// RecomputeRoleForUser updates a single user's role on sign-in. This path is
// **raise-only**: if a matching admin grant exists, the user is promoted to
// admin; otherwise their existing role is left alone. Demotion happens only
// when an admin grant is explicitly removed via the admin UI (which calls
// RecomputeUserRoles), so a role set by hand (`UPDATE users SET role='admin'`
// — the documented bootstrap for the very first admin) survives subsequent
// sign-ins. Without this rule, manual seeding would be wiped on next login.
func RecomputeRoleForUser(ctx context.Context, queries *db.Queries, userID int64, email, provider string) (string, error) {
	role, err := computeRoleFor(ctx, queries, email, provider)
	if err != nil {
		return "", err
	}
	if role != "admin" {
		// No admin grant — preserve current role (could be manually set admin).
		u, err := queries.GetUser(ctx, userID)
		if err != nil {
			return "", err
		}
		return u.Role, nil
	}
	if err := queries.SetUserRole(ctx, db.SetUserRoleParams{Role: role, ID: userID}); err != nil {
		return "", err
	}
	return role, nil
}

func computeRoleFor(ctx context.Context, queries *db.Queries, email, provider string) (string, error) {
	// Hard rule: guests are never admins. Even if an admin attaches an admin
	// grant to a guest's email or to the "All guests" group, guest identities
	// stop short of admin here. This is the load-bearing check — the admin
	// API also rejects such grants at write time, but this guard catches any
	// grant that slipped in via direct SQL or a future code path.
	if provider == "guest" {
		return "uploader", nil
	}
	rows, err := queries.GrantsForUser(ctx, db.GrantsForUserParams{
		Email:    email,
		Provider: provider,
	})
	if err != nil {
		return "", err
	}
	for _, g := range rows {
		if g.Admin != 0 {
			return "admin", nil
		}
	}
	return "uploader", nil
}
