package auth

import "testing"

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := GenerateHash("correct-password", Argon2Params{Memory: 8, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16})
	if err != nil {
		t.Fatal(err)
	}
	ok, err := VerifyPassword("correct-password", hash)
	if err != nil || !ok {
		t.Fatalf("verify = %v, %v", ok, err)
	}
	ok, err = VerifyPassword("wrong-password", hash)
	if err != nil || ok {
		t.Fatalf("wrong verify = %v, %v", ok, err)
	}
}

func TestEffectivePasswordHash(t *testing.T) {
	ok, err := VerifyPassword("password", EffectivePasswordHash(""))
	if err != nil || !ok {
		t.Fatalf("default password verify = %v, %v", ok, err)
	}
	ok, err = VerifyPassword("wrong-password", EffectivePasswordHash(""))
	if err != nil || ok {
		t.Fatalf("wrong default password verify = %v, %v", ok, err)
	}

	custom, err := GenerateHash("custom-password", Argon2Params{Memory: 8, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16})
	if err != nil {
		t.Fatal(err)
	}
	if EffectivePasswordHash(custom) != custom {
		t.Fatal("custom password hash should take precedence")
	}
	ok, err = VerifyPassword("custom-password", EffectivePasswordHash(custom))
	if err != nil || !ok {
		t.Fatalf("custom password verify = %v, %v", ok, err)
	}
}

func TestSessionLifecycle(t *testing.T) {
	manager := NewSessionManager()
	first, err := manager.Create()
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.Create()
	if err != nil {
		t.Fatal(err)
	}
	if !manager.Validate(first) {
		t.Fatal("first session should validate")
	}
	manager.Delete(first)
	if manager.Validate(first) {
		t.Fatal("first session should be deleted")
	}
	if !manager.Validate(second) {
		t.Fatal("second session should remain")
	}
	manager.InvalidateAll()
	if manager.Validate(second) || manager.Len() != 0 {
		t.Fatal("all sessions should be invalidated")
	}
}
