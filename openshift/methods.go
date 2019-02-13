package openshift

import (
	"github.com/fabric8-services/fabric8-tenant/environment"
	"gopkg.in/yaml.v2"
	"net/http"
)

// MethodDefinition represents defined actions (beforeDoCallbacks, afterDoCallbacks,requestCreator) to be executed when the method is performed for an endpoint.
type MethodDefinition struct {
	action            string
	beforeDoCallbacks BeforeDoCallbacksChain
	afterDoCallbacks  AfterDoCallbacksChain
	requestCreator    RequestCreator
}

func NewMethodDefinition(action string, beforeCallbacks []BeforeDoCallback, afterCallbacks []AfterDoCallback, requestCreator RequestCreator, modifiers ...MethodDefModifier) MethodDefinition {
	methodDefinition := MethodDefinition{
		action:            action,
		beforeDoCallbacks: BeforeDoCallbacksChain(beforeCallbacks),
		afterDoCallbacks:  AfterDoCallbacksChain(afterCallbacks),
		requestCreator:    requestCreator,
	}
	for _, modify := range modifiers {
		modify(&methodDefinition)
	}
	return methodDefinition
}

const EnsureDeletion = "ENSURE_DELETION"

type methodDefCreator func(endpoint string) MethodDefinition
type RequestCreatorModifier func(requestCreator RequestCreator) RequestCreator
type MethodDefModifier func(*MethodDefinition) *MethodDefinition

type BeforeDoCallbackFuncCreator func(previousCallback BeforeDoCallbackFunc) BeforeDoCallbackFunc
type BeforeDoCallbackFunc func(context CallbackContext) (*MethodDefinition, []byte, error)

type AfterDoCallbackFuncCreator func(previousCallback AfterDoCallbackFunc) AfterDoCallbackFunc
type AfterDoCallbackFunc func(context CallbackContext) (*Result, error)
type CallbackContext struct {
	Client       *Client
	Object       environment.Object
	ObjEndpoints *ObjectEndpoints
	Method       *MethodDefinition
}

func NewCallbackContext(client *Client, object environment.Object, objEndpoints *ObjectEndpoints, method *MethodDefinition) CallbackContext {
	return CallbackContext{
		Client:       client,
		Object:       object,
		ObjEndpoints: objEndpoints,
		Method:       method,
	}
}

type BeforeDoCallbacksChain []BeforeDoCallback

var DefaultBeforeDoCallBack = func(context CallbackContext) (*MethodDefinition, []byte, error) {
	reqBody, err := yaml.Marshal(context.Object)
	return context.Method, reqBody, err
}

func (c BeforeDoCallbacksChain) call(context CallbackContext) (*MethodDefinition, []byte, error) {
	callbackFunc := DefaultBeforeDoCallBack
	for _, callback := range c {
		callbackFunc = callback.Create(callbackFunc)
	}
	return callbackFunc(context)
}

func BeforeDo(beforeDoCallback ...BeforeDoCallback) MethodDefModifier {
	return func(methodDefinition *MethodDefinition) *MethodDefinition {
		methodDefinition.beforeDoCallbacks = append(methodDefinition.beforeDoCallbacks, beforeDoCallback...)
		return methodDefinition
	}
}

type AfterDoCallbacksChain []AfterDoCallback

func (c AfterDoCallbacksChain) call(context CallbackContext, result *Result) error {
	callbackFunc := func(context CallbackContext) (*Result, error) {
		return result, result.err
	}
	for _, callback := range c {
		callbackFunc = callback.Create(callbackFunc)
	}
	return CheckHTTPCode(callbackFunc(context))
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
					req.Header.Set("Accept", "application/json")
					req.Header.Set("Content-Type", "application/merge-patch+json")
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

func ENSURE_DELETION(active bool, modifiers ...MethodDefModifier) methodDefCreator {
	return func(urlTemplate string) MethodDefinition {
		return NewMethodDefinition(
			EnsureDeletion,
			[]BeforeDoCallback{WaitUntilIsRemoved(active)},
			[]AfterDoCallback{IgnoreWhenDoesNotExistOrConflicts},
			RequestCreator{
				creator: func(urlCreator urlCreator, body []byte) (*http.Request, error) {
					body = []byte(deleteOptions)
					return newDefaultRequest(http.MethodDelete, urlCreator(urlTemplate), body)
				}},
			modifiers...)
	}
}
