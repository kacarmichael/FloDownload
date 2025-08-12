package httpClient

import (
	"errors"
	"fmt"
)

// HTTPError represents an HTTP error with status code and message
type HTTPError struct {
	StatusCode int
	Message    string
}

// Error returns the string representation of the HTTP error
func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// Is implements error comparison for errors.Is
func (e *HTTPError) Is(target error) bool {
	var httpErr *HTTPError
	if errors.As(target, &httpErr) {
		return e.StatusCode == httpErr.StatusCode
	}
	return false
}

// NewHTTPError creates a new HTTP error
func NewHTTPError(statusCode int, message string) error {
	return &HTTPError{
		StatusCode: statusCode,
		Message:    message,
	}
}

// IsHTTPError checks if an error is an HTTP error
func IsHTTPError(err error) bool {
	var httpErr *HTTPError
	return errors.As(err, &httpErr)
}

// GetHTTPStatusCode extracts the status code from an HTTP error
func GetHTTPStatusCode(err error) int {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode
	}
	return 0
}

// Legacy support for existing code
type HttpError struct {
	Code int
}

func (e *HttpError) Error() string { return fmt.Sprintf("httpClient %d", e.Code) }

func IsHTTPStatus(err error, code int) bool {
	var he *HttpError
	if errors.As(err, &he) {
		return he.Code == code
	}
	return false
}
