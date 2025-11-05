package env

import "testing"

func TestSigner_LookupExists_Fallback(t *testing.T) {
	e := New().(*environ)
	// Only prefix-level value
	e.Updates(map[string]string{"APP_PORT": "8080"})
	// Category-level value
	e.Updates(map[string]string{"APP_WEB_HOST": "0.0.0.0"})
	// Another category value
	e.Updates(map[string]string{"APP_DB_HOST": "127.0.0.1"})

	web := e.Signed("APP", "WEB")
	if v, ok := web.Lookup("HOST"); !ok || v != "0.0.0.0" {
		t.Fatalf("WEB HOST lookup failed, got (%v,%v)", v, ok)
	}
	if !web.Exists("HOST") {
		t.Fatalf("WEB HOST exists should be true")
	}

	// Fallback to prefix-level when category missing
	if v, ok := web.Lookup("PORT"); !ok || v != "8080" {
		t.Fatalf("WEB PORT fallback failed, got (%v,%v)", v, ok)
	}

	db := e.Signed("APP", "DB")
	if v, ok := db.Lookup("HOST"); !ok || v != "127.0.0.1" {
		t.Fatalf("DB HOST lookup failed, got (%v,%v)", v, ok)
	}
}

func TestSigner_Iter_FilterAndBuffering(t *testing.T) {
	e := New().(*environ)
	// Control Updates order by single-entry updates
	e.Updates(map[string]string{"APP_A_X": "1"})
	e.Updates(map[string]string{"APP_B_Y": "2"})
	e.Updates(map[string]string{"APP_Z": "3"})
	e.Updates(map[string]string{"OTHER": "9"})

	s := e.Signed("APP", "B")
	next := s.(*signer).iter()

	k, v, ok := next()
	if !ok || k != "Y" || v != "2" {
		t.Fatalf("first: want (Y,2,true), got (%q,%q,%v)", k, v, ok)
	}
	k, v, ok = next()
	if !ok || k != "A_X" || v != "1" { // buffered from APP_A_X
		t.Fatalf("second: want (A_X,1,true), got (%q,%q,%v)", k, v, ok)
	}
	k, v, ok = next()
	if !ok || k != "Z" || v != "3" { // buffered from APP_Z
		t.Fatalf("third: want (Z,3,true), got (%q,%q,%v)", k, v, ok)
	}
	_, _, ok = next()
	if ok {
		t.Fatalf("fourth: want ok=false when exhausted")
	}
}
