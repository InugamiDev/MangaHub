package auth

import "testing"

func TestPasswordHashAndCheck(t *testing.T) {
	hash, err := HashPassword("Password123")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if hash == "Password123" {
		t.Fatal("password hash should not equal plain password")
	}
	if !CheckPassword(hash, "Password123") {
		t.Fatal("expected password to validate")
	}
	if CheckPassword(hash, "wrong-password") {
		t.Fatal("expected wrong password to fail")
	}
}

func TestJWTGenerateAndParse(t *testing.T) {
	manager := NewJWTManager("test-secret")
	token, _, err := manager.Generate("usr_1", "demo")
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	claims, err := manager.Parse(token)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if claims.UserID != "usr_1" || claims.Username != "demo" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
	if claims.Role != "reader" {
		t.Fatalf("default role = %q, want reader", claims.Role)
	}
	adminToken, _, err := manager.Generate("usr_2", "admin", "admin")
	if err != nil {
		t.Fatalf("Generate admin token returned error: %v", err)
	}
	adminClaims, err := manager.Parse(adminToken)
	if err != nil {
		t.Fatalf("Parse admin token returned error: %v", err)
	}
	if adminClaims.Role != "admin" {
		t.Fatalf("admin role = %q, want admin", adminClaims.Role)
	}
}
