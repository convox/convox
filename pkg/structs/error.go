package structs

import (
	"fmt"
	"net/http"
)

type HttpError struct {
	code int
	err  error
}

func (e *HttpError) Error() string {
	return e.err.Error()
}

func (e *HttpError) Code() int {
	return e.code
}

func (e *HttpError) Unwrap() error {
	return e.err
}

func newHttpError(code int, format string, args ...interface{}) *HttpError {
	return &HttpError{
		code: code,
		err:  fmt.Errorf(format, args...),
	}
}

func ErrNotFound(format string, args ...interface{}) *HttpError {
	return newHttpError(http.StatusNotFound, format, args...)
}

func ErrBadRequest(format string, args ...interface{}) *HttpError {
	return newHttpError(http.StatusBadRequest, format, args...)
}

func ErrConflict(format string, args ...interface{}) *HttpError {
	return newHttpError(http.StatusConflict, format, args...)
}

func ErrNotImplemented(format string, args ...interface{}) *HttpError {
	return newHttpError(http.StatusNotImplemented, format, args...)
}
