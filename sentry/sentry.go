package sentry

import (
	"context"
	"fmt"
	"github.com/fabric8-services/fabric8-common/log"
	"github.com/fabric8-services/fabric8-common/sentry"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/token"
	"github.com/fabric8-services/fabric8-tenant/user"
	"github.com/getsentry/raven-go"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
)

// InitializeLogger initializes sentry client
func InitializeLogger(config *configuration.Data, userService user.Service, commit string) (func(), error) {
	log.InitializeLogger(config.IsLogJSON(), config.GetLogLevel())
	sentryDSN := config.GetSentryDSN()

	return sentry.InitializeSentryClient(
		&sentryDSN,
		sentry.WithRelease(commit),
		sentry.WithEnvironment(config.GetEnvironment()),
		sentry.WithUser(extractUserInfo(userService)))
}

func extractUserInfo(userService user.Service) func(ctx context.Context) (*raven.User, error) {
	return func(ctx context.Context) (*raven.User, error) {
		userToken := goajwt.ContextJWT(ctx)
		if userToken == nil {
			return nil, fmt.Errorf("no token found in context")
		}
		ttoken := &token.TenantToken{Token: userToken}
		user, err := userService.GetUser(ctx, ttoken.Subject())
		if err != nil {
			return nil, err
		}
		return &raven.User{
			Username: value(user.Username),
			Email:    value(user.Email),
			ID:       value(user.IdentityID),
		}, nil
	}
}

func value(pointer *string) string {
	if pointer == nil {
		return ""
	}
	return *pointer
}

// LogError logs the given error and reports it to sentry
func LogError(ctx context.Context, fields map[string]interface{}, err error, message string) {
	sentryError := fmt.Errorf("an error occured with a message: \n%s\n with fields: \n%s\n and caused by: \n%s", message, fields, err)
	sentry.Sentry().CaptureError(ctx, sentryError)

	fields["error"] = err
	log.Error(ctx, fields, message)
}
