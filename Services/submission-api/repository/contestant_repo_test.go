package repository

import "testing"

func TestHashAPIKeyDeterministicHex(t *testing.T) {
	h1 := HashAPIKey("key-alice-0001")
	h2 := HashAPIKey("key-alice-0001")
	if h1 != h2 {
		t.Fatal("hash is not deterministic")
	}
	if len(h1) != 64 {
		t.Fatalf("want 64 hex chars, got %d", len(h1))
	}
	if HashAPIKey("key-alice-0001") == HashAPIKey("key-bob-0002") {
		t.Fatal("distinct keys produced the same hash")
	}
}

// TestSeededHashesMatch pins the hashes the seed inserts to what the code
// computes, so the lookup path stays consistent with scripts/local-dev/seed.sql.
func TestSeededHashesMatch(t *testing.T) {
	cases := map[string]string{
		"key-alice-0001": "01f9350b55022160f9b24feea1557eeec5995bbd4b459d2d107ee247b2b17375",
		"key-bob-0002":   "4ead32619d45c41952a53c1c6ef77ec7ca83f2d03e18abc8cb339f3d28e3ecec",
		"key-carol-0003": "97385903a9bf179686c78b4d92e5ce2efb98123e8a9db8c3a69696e0a3980312",
		"key-dave-0004":  "63e4a50b751506036063c83a06927a217e67c466bc495e187deef5cd8ab4c3e2",
		"key-eve-0005":   "27755369a1432103597857c798bb54d24498c9cd07c10895d450f91cc34c2703",
	}
	for key, want := range cases {
		if got := HashAPIKey(key); got != want {
			t.Errorf("HashAPIKey(%q) = %s, want %s", key, got, want)
		}
	}
}
