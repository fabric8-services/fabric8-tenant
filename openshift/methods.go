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

func NewMethodDefinition(action string, beforeCallbacks []BeforeDoCallback, afterCallbacks []AfterDoCallback, requestCreator RequestCreator, modifiers ...MethodDefModifier) MethodDefinition {
	methodDefinition := MethodDefinition{
		action:            action,
		beforeDoCallbacks: beforeCallbacks,
		afterDoCallbacks:  afterCallbacks,
		requestCreator:    requestCreator,
	}
	for _, modify := range modifiers {
		modify(&methodDefinition)
	}
	return methodDefinition
}

type methodDefCreator func(endpoint string) MethodDefinition
type RequestCreatorModifier func(requestCreator RequestCreator) RequestCreator

type MethodDefModifier func(*MethodDefinition) *MethodDefinition

const MethodDeleteAll = "DELETEALL"

func BeforeDo(beforeDoCallback ...BeforeDoCallback) MethodDefModifier {
	return func(methodDefinition *MethodDefinition) *MethodDefinition {
		methodDefinition.beforeDoCallbacks = append(methodDefinition.beforeDoCallbacks, beforeDoCallback...)
		return methodDefinition
	}
}
func AfterDo(afterDoCallbacks ...AfterDoCallback) MethodDefModifier {
	return func(methodDefinition *MethodDefinition) *MethodDefinition {
		methodDefinition.afterDoCallbacks = append(methodDefinition.afterDoCallbacks, afterDoCallbacks...)
		return methodDefinition
	}
}

func Require(requestCreatorModifier RequestCreatorModifier) MethodDefModifier {
	return func(methodDefinition *MethodDefinition) *MethodDefinition {
		methodDefinition.requestCreator = requestCreatorModifier(methodDefinition.requestCreator)
		return methodDefinition
	}
}

func MasterToken(requestCreator RequestCreator) RequestCreator {
	requestCreator.needMasterToken = true
	return requestCreator
}

func POST(modifiers ...MethodDefModifier) methodDefCreator {
	return func(urlTemplate string) MethodDefinition {
		return NewMethodDefinition(
			http.MethodPost,
			[]BeforeDoCallback{},
			[]AfterDoCallback{},
			RequestCreator{
				creator: func(urlCreator urlCreator, body []byte) (*http.Request, error) {
					return newDefaultRequest(http.MethodPost, urlCreator(urlTemplate), body)
				}},
			modifiers...)
	}
}
func PATCH(modifiers ...MethodDefModifier) methodDefCreator {
	return func(urlTemplate string) MethodDefinition {
		return NewMethodDefinition(
			http.MethodPatch,
			[]BeforeDoCallback{GetObjectAndMerge},
			[]AfterDoCallback{},
			RequestCreator{
				creator: func(urlCreator urlCreator, body []byte) (*http.Request, error) {
					req, err := newDefaultRequest(http.MethodPatch, urlCreator(urlTemplate), body)
					if err != nil {
						return nil, err
					}
					req.Header.Set("Content-Type", "application/strategic-merge-patch+json")
					return req, err
				}},
			modifiers...)
	}
}
func GET(modifiers ...MethodDefModifier) methodDefCreator {
	return func(urlTemplate string) MethodDefinition {
		return NewMethodDefinition(
			http.MethodGet,
			[]BeforeDoCallback{},
			[]AfterDoCallback{},
			RequestCreator{
				creator: func(urlCreator urlCreator, body []byte) (*http.Request, error) {
					return newDefaultRequest(http.MethodGet, urlCreator(urlTemplate), body)
				}},
			modifiers...)
	}
}

func DELETE(modifiers ...MethodDefModifier) methodDefCreator {
	return func(urlTemplate string) MethodDefinition {
		return NewMethodDefinition(
			http.MethodDelete,
			[]BeforeDoCallback{},
			[]AfterDoCallback{IgnoreWhenDoesNotExistOrConflicts},
			RequestCreator{
				creator: func(urlCreator urlCreator, body []byte) (*http.Request, error) {
					body = []byte(deleteOptions)
					return newDefaultRequest(http.MethodDelete, urlCreator(urlTemplate), body)
				}},
			modifiers...)
	}
}

func DELETEALL(modifiers ...MethodDefModifier) methodDefCreator {
	return func(urlTemplate string) MethodDefinition {
		return NewMethodDefinition(
			MethodDeleteAll,
			[]BeforeDoCallback{},
			[]AfterDoCallback{IgnoreWhenDoesNotExistOrConflicts},
			RequestCreator{
				creator: func(urlCreator urlCreator, body []byte) (*http.Request, error) {
					body = []byte(deleteOptions)
					return newDefaultRequest(http.MethodDelete, urlCreator(urlTemplate), body)
				}},
			modifiers...)
	}
}
