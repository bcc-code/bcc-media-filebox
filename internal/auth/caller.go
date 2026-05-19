package auth

import "context"

// Caller represents an authenticated user attached to a request.
// A nil *Caller (returned by CallerFrom) means the request is a guest.
type Caller struct {
	UserID   int64
	Provider string
	Subject  string
	Email    string
	Name     string
	Role     string
}

// CanonicalUserID is the value written to uploads.user_id for this caller.
// Format: "<provider>:<subject>" — namespaced to avoid collision with guest
// IDs ("guest:<ulid>") and legacy raw-ULID rows.
func (c *Caller) CanonicalUserID() string {
	return c.Provider + ":" + c.Subject
}

type callerKey struct{}

func WithCaller(ctx context.Context, c *Caller) context.Context {
	return context.WithValue(ctx, callerKey{}, c)
}

// CallerFrom returns the caller attached to ctx, or nil for guest requests.
func CallerFrom(ctx context.Context) *Caller {
	c, _ := ctx.Value(callerKey{}).(*Caller)
	return c
}
