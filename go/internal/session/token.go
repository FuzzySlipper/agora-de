package session

import (
	"errors"
	"strings"
)

var ErrInvalidToken = errors.New("invalid session token")

type Token string

func NewToken(value string) (Token, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ErrInvalidToken
	}
	return Token(value), nil
}

func MustToken(value string) Token {
	token, err := NewToken(value)
	if err != nil {
		panic(err)
	}
	return token
}

func (token Token) String() string {
	return string(token)
}

