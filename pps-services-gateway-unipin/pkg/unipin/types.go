package unipin

// BusinessError represents a business-level error from Unipin API.
type BusinessError struct {
	Status int
	Reason string
}

// Error returns a readable business error message.
func (e *BusinessError) Error() string {
	return "unipin business error: " + e.Reason
}

// TechnicalError represents technical failures such as timeout, network and 5xx response.
type TechnicalError struct {
	StatusCode int
	Cause      error
}

// Error returns a readable technical error message.
func (e *TechnicalError) Error() string {
	if e.StatusCode > 0 {
		if e.Cause != nil {
			return "unipin technical error: status=" + itoa(e.StatusCode) + " cause=" + e.Cause.Error()
		}
		return "unipin technical error: status=" + itoa(e.StatusCode)
	}
	if e.Cause != nil {
		return "unipin technical error: " + e.Cause.Error()
	}
	return "unipin technical error"
}

// Unwrap returns wrapped error cause for TechnicalError.
func (e *TechnicalError) Unwrap() error {
	return e.Cause
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
