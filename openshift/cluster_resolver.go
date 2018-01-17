package openshift

import "context"

// TokenResolver resolves a Token for a given user/service
type TokenResolver func(ctx context.Context, url, auth string) (string, error)
