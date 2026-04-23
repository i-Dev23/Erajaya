package testmocks

type MockLogger struct {
	ErrorFunc func(msg string, keysAndValues ...interface{})
	WarnFunc  func(msg string, keysAndValues ...interface{})
	InfoFunc  func(msg string, keysAndValues ...interface{})
	DebugFunc func(msg string, keysAndValues ...interface{})
}

func (m *MockLogger) Error(msg string, keysAndValues ...interface{}) {
	if m.ErrorFunc != nil {
		m.ErrorFunc(msg, keysAndValues...)
	}
}
func (m *MockLogger) Warn(msg string, keysAndValues ...interface{}) {
	if m.WarnFunc != nil {
		m.WarnFunc(msg, keysAndValues...)
	}
}
func (m *MockLogger) Info(msg string, keysAndValues ...interface{}) {
	if m.InfoFunc != nil {
		m.InfoFunc(msg, keysAndValues...)
	}
}
func (m *MockLogger) Debug(msg string, keysAndValues ...interface{}) {
	if m.DebugFunc != nil {
		m.DebugFunc(msg, keysAndValues...)
	}
}

// Add methods as needed for tests
// Example:
// func (m *MockLogger) SomeMethod(...) {...}
