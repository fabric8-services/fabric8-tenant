package sentry

import (
	"context"

	"github.com/fabric8-services/fabric8-common/log"
)

// InitializeLogger initializes sentry client
//func InitializeLogger(config *configuration.Data, commit string) (func(), error) {
//	log.InitializeLogger(config.IsLogJSON(), config.GetLogLevel())
//	sentryDSN := config.GetSentryDSN()
//
//	return sentry.InitializeSentryClient(
//		&sentryDSN,
//		sentry.WithRelease(commit),
//		sentry.WithEnvironment(config.GetEnvironment()),
//		sentry.WithUser(extractUserInfo()))
//}
//
//func extractUserInfo() func(ctx context.Context) (*raven.User, error) {
//	return func(ctx context.Context) (*raven.User, error) {
//		if ctx == nil {
//			return unknownUser, nil
//		}
//		userToken := goajwt.ContextJWT(ctx)
//		if userToken == nil {
//			return unknownUser, nil
//		}
//		ttoken := &auth.TenantToken{Token: userToken}
//		return &raven.User{
//			Username: ttoken.Username(),
//			Email:    ttoken.Email(),
//			ID:       ttoken.Subject().String(),
//		}, nil
//	}
//}
//
//var unknownUser = &raven.User{
//	Username: "unknown/update",
//	Email:    "unknown/update",
//	ID:       uuid.UUID{}.String(),
//}

// LogError logs the given error and reports it to sentry
func LogError(ctx context.Context, fields map[string]interface{}, err error, message string) {
	//sentryError := errors.Wrapf(err, "an error occurred with a message: \n%s\n and with fields: \n%s\n", message, fields)
	//sentry.Sentry().CaptureError(ctx, sentryError)

	fields["err"] = err
	log.Error(ctx, fields, message)
}
