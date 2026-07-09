package users

import "errors"

var ErrUserAlreadyExists = errors.New("user already exists")
var ErrEmailRequired = errors.New("email is required")
var ErrInvalidEmail = errors.New("email is invalid")
var ErrPasswordTooShort = errors.New("password is too short")
var ErrInvalidCredentials = errors.New("invalid credentials")
