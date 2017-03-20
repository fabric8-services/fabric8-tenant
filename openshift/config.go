package openshift

import "fmt"

type Config struct {
	MasterURL string
	Token     string
}

type multiError struct {
	Message string
	Errors  []error
}

func (m multiError) Error() string {
	s := m.Message + "\n"
	for _, err := range m.Errors {
		s += fmt.Sprintf("%v\n", err)
	}
	return s
}

func (m *multiError) String() string {
	return m.Error()
}
