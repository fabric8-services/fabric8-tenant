package sentry

import (
	"context"
	"fmt"
	"github.com/fabric8-services/fabric8-common/log"
	"github.com/fabric8-services/fabric8-common/sentry"
	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/getsentry/raven-go"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
)

// InitializeLogger initializes sentry client
func InitializeLogger(config *configuration.Data, commit string) (func(), error) {
	log.InitializeLogger(config.IsLogJSON(), config.GetLogLevel())
	sentryDSN := config.GetSentryDSN()

	return sentry.InitializeSentryClient(
		&sentryDSN,
		sentry.WithRelease(commit),
		sentry.WithEnvironment(config.GetEnvironment()),
		sentry.WithUser(extractUserInfo()))
}

func extractUserInfo() func(ctx context.Context) (*raven.User, error) {
	return func(ctx context.Context) (*raven.User, error) {
		userToken := goajwt.ContextJWT(ctx)
		if userToken == nil {
			return nil, fmt.Errorf("no token found in context")
		}
		ttoken := &auth.TenantToken{Token: userToken}
		return &raven.User{
			Username: ttoken.Username(),
			Email:    ttoken.Email(),
			ID:       ttoken.Subject().String(),
		}, nil
	}
}

// LogError logs the given error and reports it to sentry
func LogError(ctx context.Context, fields map[string]interface{}, err error, message string) {
	sentryError := fmt.Errorf("an error occured with a message: \n%s\n with fields: \n%s\n and caused by: \n%s", message, fields, err)
	sentry.Sentry().CaptureError(ctx, sentryError)

	fields["err"] = err
	log.Error(ctx, fields, message)
}
