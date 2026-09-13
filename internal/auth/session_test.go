package auth

import "testing"

const testSecret = "test-secret-key"

func TestSessionRoundTrip(t *testing.T) {
	s := &Session{Username: "admin", PasswordVersion: 3, CSRF: "abc123"}
	encoded, err := s.Encode(testSecret)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := Decode(encoded, testSecret)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Username != "admin" || decoded.PasswordVersion != 3 || decoded.CSRF != "abc123" {
		t.Fatalf("roundtrip mismatch: %+v", decoded)
	}
}

func TestSessionTamperRejected(t *testing.T) {
	s := &Session{Username: "admin"}
	encoded, _ := s.Encode(testSecret)
	// 篡改 payload（替换最后一个字符）。
	tampered := encoded[:len(encoded)-1] + "x"
	if _, err := Decode(tampered, testSecret); err == nil {
		t.Fatalf("tampered session should be rejected")
	}
}

func TestSessionWrongSecretRejected(t *testing.T) {
	s := &Session{Username: "admin"}
	encoded, _ := s.Encode(testSecret)
	if _, err := Decode(encoded, "other-secret"); err == nil {
		t.Fatalf("session signed with wrong secret should be rejected")
	}
}

func TestCSRF(t *testing.T) {
	s := &Session{}
	t1 := GenerateCSRFToken(s)
	t2 := GenerateCSRFToken(s)
	if t1 != t2 {
		t.Fatalf("CSRF token should be stable once generated")
	}
	if len(t1) != 64 {
		t.Fatalf("CSRF token length = %d, want 64", len(t1))
	}
}
