package env

import (
	"reflect"
	"testing"
	"time"
)

func TestEnviron_Updates_Lookup_Exists_Map_Where(t *testing.T) {
	e := New().(*environ)
	// Updates values, including an empty one
	e.Updates(map[string]string{
		"A":         "1",
		"B":         "",
		"GROUP_X":   "x",
		"GROUP_Y":   "y",
		"IGNORED_Z": "z",
	})

	// Lookup returns ok=false for empty value, Exists still true
	if v, ok := e.Lookup("A"); !ok || v != "1" {
		t.Fatalf("Lookup A failed, got (%q,%v)", v, ok)
	}
	if v, ok := e.Lookup("B"); ok || v != "" {
		t.Fatalf("Lookup B should be empty and not ok, got (%q,%v)", v, ok)
	}
	if !e.Exists("B") {
		t.Fatalf("Exists B should be true even if value empty")
	}
	if e.Exists("NOPE") {
		t.Fatalf("Exists NOPE should be false")
	}

	m := e.Map("GROUP_")
	if !reflect.DeepEqual(m, map[string]string{"X": "x", "Y": "y"}) {
		t.Fatalf("Map GROUP_ unexpected: %#v", m)
	}

	w := e.Where(func(name, value string) bool { return len(name) == 1 })
	if !reflect.DeepEqual(w, map[string]string{"A": "1", "B": ""}) {
		t.Fatalf("Where unexpected: %#v", w)
	}
}

func TestSigner_TypedGetters_WithFallbacks(t *testing.T) {
	e := New().(*environ)
	// Category-level
	e.Updates(map[string]string{
		"APP_WEB_HOST":    "0.0.0.0",
		"APP_WEB_DEBUG":   "true",
		"APP_WEB_TIMEOUT": "150ms",
		"APP_WEB_RATE":    "3.14",
		"APP_WEB_TAGS":    "a, b, c",
	})
	// Prefix-level fallbacks
	e.Updates(map[string]string{
		"APP_PORT":    "8080",
		"APP_TIMEOUT": "200ms",
		"APP_RATE":    "2.5",
		"APP_ENABLED": "1",
	})

	s := e.Signed("APP", "WEB")

	if got := s.String("HOST"); got != "0.0.0.0" {
		t.Fatalf("String HOST: want 0.0.0.0, got %q", got)
	}
	if got := string(s.Bytes("HOST")); got != "0.0.0.0" {
		t.Fatalf("Bytes HOST: want 0.0.0.0, got %q", got)
	}
	if got := s.Int("PORT"); got != 8080 {
		t.Fatalf("Int PORT (fallback): want 8080, got %d", got)
	}
	if got := s.String("RATE"); got != "3.14" { // prefer category-level
		t.Fatalf("String RATE: want 3.14, got %v", got)
	}
	if got := s.Duration("TIMEOUT"); got != 150*time.Millisecond {
		t.Fatalf("Duration TIMEOUT: want 150ms, got %v", got)
	}
	if got := s.Bool("ENABLED"); got != true { // fallback prefix-level
		t.Fatalf("Bool ENABLED fallback: want true, got %v", got)
	}
	list := s.List("TAGS")
	if !reflect.DeepEqual(list, []string{"a", "b", "c"}) {
		t.Fatalf("List TAGS: want [a b c], got %#v", list)
	}
}

func TestSigner_Fill_Struct(t *testing.T) {
	e := New().(*environ)
	e.Updates(map[string]string{
		"APP_WEB_HOST":     "localhost",
		"APP_PORT":         "9000", // fallback for WEB
		"APP_WEB_DEBUG":    "false",
		"APP_TIMEOUT":      "1s", // fallback
		"APP_WEB_RATE":     "1.5",
		"APP_FEATURE_FLAG": "true", // nested
	})
	type Feature struct {
		Flag bool `env:"FEATURE_FLAG"`
	}
	type Config struct {
		Host    string  `env:"HOST"`
		Port    int     `env:"PORT"`
		Debug   bool    `env:"DEBUG"`
		Timeout string  `env:"TIMEOUT"` // cast.FromType lacks time.Duration support
		Rate    float64 `env:"RATE"`
		Feat    Feature
		Opt     *Feature
	}
	cfg := &Config{Opt: &Feature{}}

	s := e.Signed("APP", "WEB")
	if err := s.Fill(cfg); err != nil {
		t.Fatalf("Fill error: %v", err)
	}

	if cfg.Host != "localhost" || cfg.Port != 9000 || cfg.Debug != false || cfg.Timeout != "1s" || cfg.Rate != 1.5 {
		t.Fatalf("filled primitives mismatch: %+v", cfg)
	}
	if cfg.Feat.Flag != true || cfg.Opt.Flag != true {
		t.Fatalf("filled nested/ptr mismatch: %+v", cfg)
	}
}
