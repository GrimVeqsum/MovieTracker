package users

import "errors"

var ErrUserAlreadyExists = errors.New("user already exists")

var ErrEmailRequired = errors.New("email is required")

var ErrInvalidEmail = errors.New("email is invalid")

var ErrPasswordTooShort = errors.New("password is too short")

var ErrInvalidCredentials = errors.New("invalid credentials")

var ErrInvalidAccessToken = errors.New("invalid access token")

var ErrTelegramLinkCodeNotFound = errors.New("telegram link code not found or expired")

var ErrTelegramAccountAlreadyLinked = errors.New("telegram account already linked")

var ErrMovieTrackerAccountAlreadyLinked = errors.New("movietracker account already linked to telegram")

var ErrTelegramUserNotLinked = errors.New("telegram user is not linked")

var ErrInvalidTelegramUserID = errors.New("invalid telegram user id")
