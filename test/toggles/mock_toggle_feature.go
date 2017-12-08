package toggles

// NewMockFeature returns a new MockFeature
func NewMockFeature(name string, enabled bool) *MockFeature {
	f := MockFeature{
		name:    name,
		enabled: enabled,
	}
	return &f
}

// MockFeature a mock feature that can only be enabled or disabled
type MockFeature struct {
	name    string
	enabled bool
}
