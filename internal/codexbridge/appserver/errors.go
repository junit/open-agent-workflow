package appserver

import "errors"

type Error struct {
	Code   string
	Detail string
	Cause  error
}

func (value *Error) Error() string {
	if value.Detail == "" {
		return value.Code
	}
	return value.Code + ": " + value.Detail
}

func (value *Error) Unwrap() error     { return value.Cause }
func (value *Error) ErrorCode() string { return value.Code }

func NewError(code, detail string, cause error) error {
	return &Error{Code: code, Detail: detail, Cause: cause}
}

func Code(err error) string {
	if err == nil {
		return ""
	}
	var value interface{ ErrorCode() string }
	if errors.As(err, &value) {
		return value.ErrorCode()
	}
	return ""
}
