package tenant

import "context"

// Resolve a func to resolve tenant tokens based on tenants auth
type Resolve func(ctx context.Context, target, token string) (username, accessToken string, err error)
