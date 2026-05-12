// Small helpers shared across the translator files. Kept in its own file
// so every other file can import it without circular reference.

package openai

import (
	"encoding/json/v2"
	"net/http"
)

// ValidationError carries a validation message + the offending OpenAI
// request parameter name. Returned by TranslateRequest on bad input and
// mapped to HTTP 400 + invalid_request_error by the handler layer.
type ValidationError struct {
	Message string
	Param   string
}

// Error implements the error interface.
func (e *ValidationError) Error() string { return e.Message }

func newValidationError(msg, param string) *ValidationError {
	return &ValidationError{Message: msg, Param: param}
}

// AsValidationError returns the underlying *ValidationError if err is one,
// together with a boolean ok. Callers use it to turn translator errors into
// OpenAI-shape 400 responses.
func AsValidationError(err error) (*ValidationError, bool) {
	ve, ok := err.(*ValidationError)
	return ve, ok
}

// unmarshalJSON wraps encoding/json/v2.Unmarshal so files that do not import
// the package directly can still parse a tool_call arguments blob.
func unmarshalJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// WriteValidationError writes a 400 response in OpenAI shape from a
// ValidationError. Convenience for handlers.
func WriteValidationError(w http.ResponseWriter, err error) bool {
	ve, ok := AsValidationError(err)
	if !ok {
		return false
	}
	WriteError(w, http.StatusBadRequest, ErrTypeInvalidRequest, ve.Message, ve.Param, "")
	return true
}
