package utils

import "fmt"

type Error struct {
	code    int
	reason  string
	server  bool
	recover bool
}

func NewError(c int, r string, s bool, rc bool) *Error {
	return &Error{
		code:    c,
		reason:  r,
		server:  s,
		recover: rc,
	}
}

func (e Error) Error() string {
	return fmt.Sprintf("Exception (%d) Reason: %q", e.code, e.reason)
}

func (e Error) Code() int {
	return e.code
}

func (e Error) Reason() string {
	return e.reason
}

func (e Error) Server() bool {
	return e.server
}

func (e Error) Recover() bool {
	return e.recover
}
