package domain

import "errors"

var (
	ErrDuplicate    = errors.New("duplicate record")
	ErrNotFound     = errors.New("resource not found")
	ErrForbidden    = errors.New("cross-tenant access denied")
	ErrUnauthorized = errors.New("unauthorized")
	ErrBadRequest   = errors.New("bad request")
)
