package env

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

func TestClean_ResetsState_And_PathJoin(t *testing.T) {
    dir := t.TempDir()
    writeFile(t, dir, ".env", "K=V\nAPP_ENV=dev\n")
    if err := InitWithDir(dir); err != nil {
        t.Fatalf("InitWithDir: %v", err)
    }

    if v, ok := Lookup("K"); !ok || v != "V" {
        t.Fatalf("pre-clean Lookup K failed: %q,%v", v, ok)
    }
    // Path join
    if got := Path("a", "b"); got != filepath.Join(dir, "a", "b") {
        t.Fatalf("Path join unexpected: %q", got)
    }
    // Deprecated IsEnv mirrors String("APP_ENV")
    if !IsEnv("dev") {
        t.Fatalf("IsEnv(dev) should be true")
    }

    // Clean and verify
    Default().Clean()
    if _, ok := Lookup("K"); ok {
        t.Fatalf("Lookup K should be false after Clean")
    }
}

func TestGlobal_Wrappers_Work(t *testing.T) {
    dir := t.TempDir()
    writeFile(t, dir, ".env", "A=1\nB=true\nC=a,b\n")
    if err := InitWithDir(dir); err != nil {
        t.Fatalf("InitWithDir: %v", err)
    }
    t.Cleanup(func() { Default().Clean() })

    if v, ok := Lookup("A"); !ok || v != "1" { t.Fatalf("Lookup A failed") }
    if !Exists("A") { t.Fatalf("Exists A should be true") }
    if String("A") != "1" { t.Fatalf("String A") }
    if Int("A") != 1 { t.Fatalf("Int A") }
    if !Bool("B") { t.Fatalf("Bool B") }
    list := List("C")
    if len(list) != 2 || list[0] != "a" || list[1] != "b" {
        t.Fatalf("List C unexpected: %#v", list)
    }
    all := All()
    if all["A"] != "1" || all["B"] != "true" {
        t.Fatalf("All unexpected: %#v", all)
    }
}

func TestGlobal_Init_Path_Is_All_Load_Signed(t *testing.T) {
	dir := t.TempDir()
	// .env
	writeFile(t, dir, ".env", "FOO=base\nAPP_WEB_HOST=ignored\nAPP_PORT=9000\n")
	// .env.local overrides .env
	writeFile(t, dir, ".env.local", "FOO=local\nAPP_FEATURE_FLAG=true\n")
	// default APP_ENV will be prod, so .env.prod is loaded
	writeFile(t, dir, ".env.prod", "BAR=prod\nAPP_WEB_HOST=0.0.0.0\n")

	if err := InitWithDir(dir); err != nil {
		t.Fatalf("InitWithDir: %v", err)
	}
	t.Cleanup(func() { Default().Clean() })

	// Path should reflect initialized root
	if got := Path(); got != dir {
		t.Fatalf("Path(): want %q, got %q", dir, got)
	}

	// Is("prod") should be true by default
	if !Is("prod") {
		t.Fatalf("Is(prod) should be true")
	}

	// All should include merged values with overrides
	all := All()
	if all["FOO"] != "local" {
		t.Fatalf("FOO expected local, got %q", all["FOO"])
	}
	if all["BAR"] != "prod" {
		t.Fatalf("BAR expected prod, got %q", all["BAR"])
	}

	// Global Signed works with loaded data and fallback
	s := Signed("APP", "WEB")
	if v := s.String("HOST"); v != "0.0.0.0" { // from .env.prod
		t.Fatalf("Signed HOST want 0.0.0.0, got %q", v)
	}
	if v := s.Int("PORT"); v != 9000 { // fallback from APP_PORT in .env
		t.Fatalf("Signed PORT want 9000, got %d", v)
	}
	if v := s.Bool("FEATURE_FLAG"); v != true { // from .env.local (prefix-only)
		t.Fatalf("Signed FEATURE_FLAG want true, got %v", v)
	}

	// Load additional file
	extra := writeFile(t, dir, "extra.env", "BAZ=ok\n")
	if err := Load(extra); err != nil {
		t.Fatalf("Load(extra): %v", err)
	}
	if v, ok := Lookup("BAZ"); !ok || v != "ok" {
		t.Fatalf("Lookup BAZ want ok,true; got %q,%v", v, ok)
	}
}
