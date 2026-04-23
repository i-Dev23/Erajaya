package utils

import "errors"

var (
	ErrInvalidClientID         = errors.New("invalid client_id")
	ErrInvalidClientSecret     = errors.New("invalid client_secret")
	ErrInvalidDigitalSignature = errors.New("invalid signature")
	ErrExpiredToken            = errors.New("expired token") // Token has expired
	ErrInvalidUsername         = errors.New("invalid username")
	ErrInvalidEmail            = errors.New("invalid email")
	ErrInvalidID               = errors.New("invalid id")
	ErrInvalidParameter        = errors.New("invalid parameter: missing required field")
)
