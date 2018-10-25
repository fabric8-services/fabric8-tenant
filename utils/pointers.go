package utils

import (
	"github.com/fabric8-services/fabric8-common/errors"
	"github.com/satori/go.uuid"
)

func String(value string) *string {
	return &value
}

func UuidValue(pointer *uuid.UUID) uuid.UUID {
	if pointer == nil {
		return uuid.UUID{}
	}
	return *pointer
}

func IsEmpty(s *string) bool {
	return s == nil || *s == ""
}

func UuidFromString(value *string) (uuid.UUID, error) {
	if value == nil {
		return uuid.UUID{}, errors.NewBadParameterError("identityID", nil)
	}
	return uuid.FromString(*value)
}
