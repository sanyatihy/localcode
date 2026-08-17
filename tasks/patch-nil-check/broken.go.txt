package main

import "errors"

type Token struct {
	Value   string
	Expired bool
}

var ErrNilToken = errors.New("nil token")

// RefreshToken returns a renewed token. It must reject a nil token with
// ErrNilToken rather than dereferencing it.
func RefreshToken(tok *Token) (*Token, error) {
	if tok.Expired {
		return &Token{Value: tok.Value + "-renewed", Expired: false}, nil
	}
	return tok, nil
}
