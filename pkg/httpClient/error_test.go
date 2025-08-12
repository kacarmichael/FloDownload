package httpClient

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestHTTPError_Error(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		message    string
		want       string
	}{
		{
			name:       "basic http error",
			statusCode: 404,
			message:    "Not Found",
			want:       "HTTP 404: Not Found",
		},
		{
			name:       "server error",
			statusCode: 500,
			message:    "Internal Server Error",
			want:       "HTTP 500: Internal Server Error",
		},
		{
			name:       "unauthorized error",
			statusCode: 401,
			message:    "Unauthorized",
			want:       "HTTP 401: Unauthorized",
		},
		{
			name:       "empty message",
			statusCode: 400,
			message:    "",
			want:       "HTTP 400: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &HTTPError{
				StatusCode: tt.statusCode,
				Message:    tt.message,
			}

			got := err.Error()
			if got != tt.want {
				t.Errorf("HTTPError.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHTTPError_Is(t *testing.T) {
	err404 := &HTTPError{StatusCode: 404, Message: "Not Found"}
	err500 := &HTTPError{StatusCode: 500, Message: "Server Error"}
	otherErr404 := &HTTPError{StatusCode: 404, Message: "Different message"}
	regularError := fmt.Errorf("regular error")

	tests := []struct {
		name   string
		err    error
		target error
		want   bool
	}{
		{
			name:   "same error instance",
			err:    err404,
			target: err404,
			want:   true,
		},
		{
			name:   "different HTTP errors with same status",
			err:    err404,
			target: otherErr404,
			want:   true,
		},
		{
			name:   "different HTTP errors with different status",
			err:    err404,
			target: err500,
			want:   false,
		},
		{
			name:   "HTTP error vs regular error",
			err:    err404,
			target: regularError,
			want:   false,
		},
		{
			name:   "regular error vs HTTP error",
			err:    regularError,
			target: err404,
			want:   false,
		},
		{
			name:   "nil target",
			err:    err404,
			target: nil,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got bool
			if httpErr, ok := tt.err.(*HTTPError); ok {
				got = httpErr.Is(tt.target)
			} else {
				got = false // Non-HTTP errors return false
			}
			if got != tt.want {
				t.Errorf("HTTPError.Is() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsHTTPError(t *testing.T) {
	httpErr := &HTTPError{StatusCode: 404, Message: "Not Found"}
	regularErr := fmt.Errorf("regular error")

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "http error",
			err:  httpErr,
			want: true,
		},
		{
			name: "regular error",
			err:  regularErr,
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "wrapped http error",
			err:  fmt.Errorf("wrapped: %w", httpErr),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsHTTPError(tt.err)
			if got != tt.want {
				t.Errorf("IsHTTPError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetHTTPStatusCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "http error 404",
			err:  &HTTPError{StatusCode: 404, Message: "Not Found"},
			want: 404,
		},
		{
			name: "http error 500",
			err:  &HTTPError{StatusCode: 500, Message: "Server Error"},
			want: 500,
		},
		{
			name: "wrapped http error",
			err:  fmt.Errorf("wrapped: %w", &HTTPError{StatusCode: 403, Message: "Forbidden"}),
			want: 403,
		},
		{
			name: "regular error",
			err:  fmt.Errorf("regular error"),
			want: 0,
		},
		{
			name: "nil error",
			err:  nil,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetHTTPStatusCode(tt.err)
			if got != tt.want {
				t.Errorf("GetHTTPStatusCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewHTTPError(t *testing.T) {
	statusCode := 404
	message := "Page not found"

	err := NewHTTPError(statusCode, message)

	// Check type
	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("NewHTTPError should return *HTTPError, got %T", err)
	}

	// Check fields
	if httpErr.StatusCode != statusCode {
		t.Errorf("Expected StatusCode=%d, got %d", statusCode, httpErr.StatusCode)
	}
	if httpErr.Message != message {
		t.Errorf("Expected Message=%q, got %q", message, httpErr.Message)
	}

	// Check error string
	expectedErrorString := fmt.Sprintf("HTTP %d: %s", statusCode, message)
	if httpErr.Error() != expectedErrorString {
		t.Errorf("Expected error string=%q, got %q", expectedErrorString, httpErr.Error())
	}
}

func TestHTTPError_StatusCodeChecks(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		isClient   bool
		isServer   bool
	}{
		{"200 OK", 200, false, false},
		{"400 Bad Request", 400, true, false},
		{"401 Unauthorized", 401, true, false},
		{"404 Not Found", 404, true, false},
		{"499 Client Error", 499, true, false},
		{"500 Server Error", 500, false, true},
		{"502 Bad Gateway", 502, false, true},
		{"599 Server Error", 599, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &HTTPError{StatusCode: tt.statusCode, Message: "test"}

			isClient := err.StatusCode >= 400 && err.StatusCode < 500
			isServer := err.StatusCode >= 500 && err.StatusCode < 600

			if isClient != tt.isClient {
				t.Errorf("Status %d: expected isClient=%v, got %v", tt.statusCode, tt.isClient, isClient)
			}
			if isServer != tt.isServer {
				t.Errorf("Status %d: expected isServer=%v, got %v", tt.statusCode, tt.isServer, isServer)
			}
		})
	}
}

func TestHTTPError_Integration(t *testing.T) {
	// Test that HTTPError integrates well with standard error handling
	err := NewHTTPError(http.StatusNotFound, "Resource not found")

	// Should be able to use with errors.Is
	target := &HTTPError{StatusCode: http.StatusNotFound}
	if !err.(*HTTPError).Is(target) {
		t.Error("HTTPError should match target with same status code")
	}

	// Should be detectable as HTTPError
	if !IsHTTPError(err) {
		t.Error("Should be detectable as HTTPError")
	}

	// Should return correct status code
	if GetHTTPStatusCode(err) != http.StatusNotFound {
		t.Error("Should return correct status code")
	}

	// Should have meaningful string representation
	errorString := err.Error()
	if !strings.Contains(errorString, "404") {
		t.Error("Error string should contain status code")
	}
	if !strings.Contains(errorString, "Resource not found") {
		t.Error("Error string should contain message")
	}
}

func TestHTTPError_EdgeCases(t *testing.T) {
	// Test with zero status code
	err := NewHTTPError(0, "Zero status")
	if err.Error() != "HTTP 0: Zero status" {
		t.Errorf("Unexpected error string for zero status: %s", err.Error())
	}

	// Test with very long message
	longMessage := strings.Repeat("a", 1000)
	err = NewHTTPError(500, longMessage)
	if !strings.Contains(err.Error(), longMessage) {
		t.Error("Long message should be preserved")
	}

	// Test status code boundaries
	for _, code := range []int{399, 400, 499, 500, 599, 600} {
		err := &HTTPError{StatusCode: code}
		// Should not panic
		_ = err.Error()
	}
}
