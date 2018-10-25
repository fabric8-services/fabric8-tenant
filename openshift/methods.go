package openshift

import (
	"net/http"
)

// MethodDefinition represents defined actions (beforeDoCallbacks, afterDoCallbacks,requestCreator) to be executed when the method is performed for an endpoint.
type MethodDefinition struct {
	action            string
	beforeDoCallbacks []BeforeDoCallback
	afterDoCallbacks  []AfterDoCallback
	requestCreator    RequestCreator
}

func NewMethodDefinition(action string, beforeCallbacks []BeforeDoCallback, afterCallbacks []AfterDoCallback, requestCreator RequestCreator) *MethodDefinition {
	return &MethodDefinition{
		action:            action,
		beforeDoCallbacks: beforeCallbacks,
		afterDoCallbacks:  afterCallbacks,
		requestCreator:    requestCreator,
	}
}

type methodDefCreator func(endpoint string) *MethodDefinition
type RequestCreatorModifier func(requestCreator RequestCreator) RequestCreator

func (creator methodDefCreator) WithModifier(requestCreatorModifier RequestCreatorModifier) methodDefCreator {
	return func(urlTemplate string) *MethodDefinition {
		methodDefinition := creator(urlTemplate)
		methodDefinition.requestCreator = requestCreatorModifier(methodDefinition.requestCreator)
		return methodDefinition
	}
}

func NeedMasterToken(requestCreator RequestCreator) RequestCreator {
	requestCreator.needMasterToken = true
	return requestCreator
}

func POST(afterCallbacks ...AfterDoCallback) methodDefCreator {
	return func(urlTemplate string) *MethodDefinition {
		return NewMethodDefinition(
			http.MethodPost,
			[]BeforeDoCallback{},
			append(afterCallbacks),
			RequestCreator{
				creator: func(urlCreator urlCreator, body []byte) (*http.Request, error) {
					return newDefaultRequest(http.MethodPost, urlCreator(urlTemplate), body)
				}})
	}
}

func PUT(afterCallbacks ...AfterDoCallback) methodDefCreator {
	return func(urlTemplate string) *MethodDefinition {
		return NewMethodDefinition(
			http.MethodPut,
			[]BeforeDoCallback{},
			afterCallbacks,
			RequestCreator{
				creator: func(urlCreator urlCreator, body []byte) (*http.Request, error) {
					return newDefaultRequest(http.MethodPut, urlCreator(urlTemplate), body)
				}})
	}
}
func PATCH(afterCallbacks ...AfterDoCallback) methodDefCreator {
	return func(urlTemplate string) *MethodDefinition {
		return NewMethodDefinition(
			http.MethodPatch,
			[]BeforeDoCallback{GetObjectAndMerge},
			append(afterCallbacks),
			RequestCreator{
				creator: func(urlCreator urlCreator, body []byte) (*http.Request, error) {
					req, err := newDefaultRequest(http.MethodPatch, urlCreator(urlTemplate), body)
					if err != nil {
						return nil, err
					}
					req.Header.Set("Content-Type", "application/strategic-merge-patch+json")
					return req, err
				}})
	}
}
func GET(afterCallbacks ...AfterDoCallback) methodDefCreator {
	return func(urlTemplate string) *MethodDefinition {
		return NewMethodDefinition(
			http.MethodGet,
			[]BeforeDoCallback{},
			afterCallbacks,
			RequestCreator{
				creator: func(urlCreator urlCreator, body []byte) (*http.Request, error) {
					return newDefaultRequest(http.MethodGet, urlCreator(urlTemplate), body)
				}})
	}
}

func DELETE(afterCallbacks ...AfterDoCallback) methodDefCreator {
	return func(urlTemplate string) *MethodDefinition {
		return NewMethodDefinition(
			http.MethodDelete,
			[]BeforeDoCallback{},
			append(afterCallbacks),
			RequestCreator{
				creator: func(urlCreator urlCreator, body []byte) (*http.Request, error) {
					body = []byte(deleteOptions)
					return newDefaultRequest(http.MethodDelete, urlCreator(urlTemplate), body)
				}})
	}
}
