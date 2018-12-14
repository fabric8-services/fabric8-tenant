package environment

import (
	"database/sql/driver"
	"errors"
)

// Type describes which type of namespace this is
type Type string

// Represents the namespace type
const (
	TypeChe     Type = "che"
	TypeJenkins Type = "jenkins"
	TypeTest    Type = "test"
	TypeStage   Type = "stage"
	TypeRun     Type = "run"
	TypeUser    Type = "user"
	TypeCustom  Type = "custom"
)

// Value - Implementation of valuer for database/sql
func (ns *Type) Value() (driver.Value, error) {
	return string(*ns), nil
}

// Scan - Implement the database/sql scanner interface
func (ns *Type) Scan(value interface{}) error {
	if value == nil {
		*ns = Type("")
		return nil
	}
	if bv, err := driver.String.ConvertValue(value); err == nil {
		// if this is a bool type
		if v, ok := bv.(string); ok {
			// set the value of the pointer yne to YesNoEnum(v)
			*ns = Type(v)
			return nil
		}
	}
	// otherwise, return an error
	return errors.New("failed to scan Type")
}

func (t Type) String() string {
	return string(t)
}
