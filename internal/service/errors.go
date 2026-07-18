package service

import "errors"

var ErrRepositoryUnavailable = errors.New("database is not configured")
var ErrForbidden = errors.New("forbidden")

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}
