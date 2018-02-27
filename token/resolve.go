package token

import (
	"context"

	"github.com/fabric8-services/fabric8-tenant/auth"
)

// Resolve a func to resolve a token for a given user/service
type Resolve func(ctx context.Context, target, token string, forcePull bool, decode Decode) (username, accessToken string, err error)

// NewResolve creates a Resolver that rely on the Auth service to retrieve tokens
func NewResolve(authURL string, options ...auth.ClientOption) Resolve {
	s := tokenService{
		authURL:       authURL,
		clientOptions: options,
	}
	return s.ResolveTargetToken
}
