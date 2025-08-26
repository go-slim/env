package env

import "testing"

func TestInject_Helper(t *testing.T) {
	e := New()
	ok := Inject(e, map[string]string{"X": "1"})
	if !ok {
		t.Fatalf("Inject should return true for Environ")
	}
	if v, ok := e.Lookup("X"); !ok || v != "1" {
		t.Fatalf("Lookup after Inject failed, got (%q,%v)", v, ok)
	}

	// nil environ should fail
	if Inject(nil, map[string]string{"Y": "2"}) {
		t.Fatalf("Inject should return false for nil env")
	}
	// empty data should fail
	if Inject(e, map[string]string{}) {
		t.Fatalf("Inject should return false for empty data")
	}
}
