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

func Errorf(code int, format string, args ...interface{}) *HttpError {
	return &HttpError{
		code: code,
		err:  fmt.Errorf(format, args...),
	}
}

func NewError(code int, err error) *HttpError {
	return &HttpError{
		code: code,
		err:  err,
	}
}

var (
	errNotFound       = http.StatusNotFound
	errBadRequest     = http.StatusBadRequest
	errConflict       = http.StatusConflict
	errNotImplemented = http.StatusNotImplemented
)

func ErrNotFound(format string, args ...interface{}) *HttpError {
	return Errorf(errNotFound, format, args...)
}

func ErrBadRequest(format string, args ...interface{}) *HttpError {
	return Errorf(errBadRequest, format, args...)
}

func ErrConflict(format string, args ...interface{}) *HttpError {
	return Errorf(errConflict, format, args...)
}

func ErrNotImplemented(format string, args ...interface{}) *HttpError {
	return Errorf(errNotImplemented, format, args...)
}
