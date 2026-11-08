package domain

import "errors"

var (
	// ErrNotFound reports that the requested entity does not exist.
	ErrNotFound = errors.New("not found")
	// ErrInvalidInput reports that user supplied input failed validation.
	ErrInvalidInput = errors.New("invalid input")
	// ErrUpstreamUnavailable reports that an external dependency failed.
	ErrUpstreamUnavailable = errors.New("upstream unavailable")
	// ErrConflict reports that the change collides with existing data.
	ErrConflict = errors.New("conflict")
	// ErrUnauthorized reports that the credentials or the session are not valid.
	ErrUnauthorized = errors.New("unauthorized")
)
