package display

import "errors"

var (
	errCodeNotFound = errors.New("display: pair code not found")
	errCodeExpired  = errors.New("display: pair code expired")
	errNotFound     = errors.New("display: display not found")
)
