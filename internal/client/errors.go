package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// APIError is a non-2xx response from the Silo API. It preserves the body
// (commonly a problem-details document) and headers so callers can surface
// precise diagnostics.
type APIError struct {
	Method string
	Path   string
	// Response is never nil on an APIError.
	Response *Response
	// Detail is the problem body's message when one could be parsed.
	Detail string
}

// Error implements error. Status-code semantics live in the classifiers
// below so call sites branch on meaning, not numbers.
func (e *APIError) Error() string {
	base := fmt.Sprintf("API error %s %s: %d", e.Method, e.Path, e.Response.StatusCode)
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s", base, e.Detail)
	}
	if len(e.Response.Body) > 0 {
		return fmt.Sprintf("%s: %s", base, string(e.Response.Body))
	}
	if e.Response.RequestID != "" {
		return fmt.Sprintf("%s (request id %s)", base, e.Response.RequestID)
	}
	return base
}

// parseDetail best-effort extracts a human-readable message from a problem
// body. Silo's problem documents vary by endpoint; try the common shapes.
func parseDetail(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var doc struct {
		Detail string `json:"detail"`
		Title  string `json:"title"`
		Error  string `json:"error"`
		// Nested problem shapes.
		Problem struct {
			Detail string `json:"detail"`
			Title  string `json:"title"`
		} `json:"problem"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return ""
	}
	switch {
	case doc.Detail != "":
		return doc.Detail
	case doc.Problem.Detail != "":
		return doc.Problem.Detail
	case doc.Title != "":
		return doc.Title
	case doc.Problem.Title != "":
		return doc.Problem.Title
	case doc.Error != "":
		return doc.Error
	}
	return ""
}

func newAPIError(method, path string, resp *Response) *APIError {
	return &APIError{
		Method:   method,
		Path:     path,
		Response: resp,
		Detail:   parseDetail(resp.Body),
	}
}

// IsNotFound reports whether err is a 404 from the API.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Response.StatusCode == http.StatusNotFound
}

// IsUnauthorized reports whether err is a 401 (missing, invalid, or expired
// credential).
func IsUnauthorized(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Response.StatusCode == http.StatusUnauthorized
}

// IsForbidden reports whether err is a 403 (the key's owner or scopes lack
// permission, or a profile needs verification).
func IsForbidden(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Response.StatusCode == http.StatusForbidden
}

// IsPreconditionFailed reports whether err is a 412: the If-Match etag was
// stale, meaning the resource changed outside Terraform. The response
// carries the current ETag.
func IsPreconditionFailed(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Response.StatusCode == http.StatusPreconditionFailed
}

// IsPreconditionRequired reports whether err is a 428: the guarded write
// was missing its If-Match header entirely.
func IsPreconditionRequired(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Response.StatusCode == http.StatusPreconditionRequired
}

// IsUnprocessableEntity reports whether err is a 422 validation failure.
func IsUnprocessableEntity(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Response.StatusCode == http.StatusUnprocessableEntity
}

// CurrentETag returns the API's current ETag from a 412 response, or the
// empty string. Useful for diagnostics that tell the user how to recover.
func CurrentETag(err error) string {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Response.ETag
	}
	return ""
}
