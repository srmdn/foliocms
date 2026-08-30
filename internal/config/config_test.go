package config

import "testing"

func TestLoadJWTSecretTooShort(t *testing.T) {
	t.Setenv("JWT_SECRET", "short-secret")
	_, err := Load("")
	if err == nil {
		t.Fatal("expected error for short JWT_SECRET")
	}
}

func TestLoadJWTSecretOK(t *testing.T) {
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.JWTSecret != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("JWTSecret = %q", cfg.JWTSecret)
	}
}
