package token

import (
	"context"

	"github.com/fabric8-services/fabric8-tenant/auth"
)

// Resolve a func to resolve a token for a given user/service
type Resolve func(ctx context.Context, target, token *string, decode Decode) (username, accessToken *string, err error)

// NewResolve creates a Resolver that rely on the Auth service to retrieve tokens
func NewResolve(config auth.ClientConfig) Resolve {
	c := tokenService{config: config}
	return c.ResolveUserToken
}
