package main

import (
	"errors"
	"testing"
)

func TestRefreshTokenNil(t *testing.T) {
	got, err := RefreshToken(nil)
	if !errors.Is(err, ErrNilToken) {
		t.Fatalf("RefreshToken(nil) err = %v, want ErrNilToken", err)
	}
	if got != nil {
		t.Fatalf("RefreshToken(nil) = %v, want nil", got)
	}
}

func TestRefreshTokenExpired(t *testing.T) {
	got, err := RefreshToken(&Token{Value: "abc", Expired: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Value != "abc-renewed" || got.Expired {
		t.Fatalf("got %+v, want {abc-renewed false}", got)
	}
}

func TestRefreshTokenValid(t *testing.T) {
	in := &Token{Value: "xyz", Expired: false}
	got, err := RefreshToken(in)
	if err != nil || got != in {
		t.Fatalf("valid token should pass through unchanged, got %+v err %v", got, err)
	}
}
