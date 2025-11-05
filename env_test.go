package env

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Tests for Updates global function (from env.go)
func TestUpdates_Global(t *testing.T) {
	// Clean the global environment first
	Default().Clean()

	// Test global Updates function (operates on global environment automatically)
	ok := Updates(map[string]string{"X": "1"})
	if !ok {
		t.Fatalf("Updates should return true for default environment")
	}

	// Verify using global Lookup function (not instance method)
	if v, ok := Lookup("X"); !ok || v != "1" {
		t.Fatalf("Global Lookup after Updates failed, got (%q,%v)", v, ok)
	}

	// empty data should succeed (no error)
	if !Updates(map[string]string{}) {
		t.Fatalf("Updates should return true for empty data (no error)")
	}

	// Clean up after test
	Default().Clean()
}

// Test other global functions from env.go
func TestGlobal_Functions(t *testing.T) {
	// These tests use the default global environment (env.Default())

	// Setup global environment using Load (which uses the default env)
	// Clear the default environment first
	Default().Clean()

	// Use the instance method directly on the default environment
	if err := Default().Updates(map[string]string{
		"TEST_STRING": "hello",
		"TEST_INT":    "42",
		"TEST_BOOL":   "true",
		"TEST_LIST":   "a,b,c",
	}); err != nil {
		t.Fatalf("Setup Updates failed: %v", err)
	}

	// Test global Lookup
	if v, ok := Lookup("TEST_STRING"); !ok || v != "hello" {
		t.Fatalf("Lookup failed: got (%q,%v)", v, ok)
	}

	// Test global String
	if v := String("TEST_STRING", "default"); v != "hello" {
		t.Fatalf("String failed: got %q", v)
	}
	if v := String("MISSING", "default"); v != "default" {
		t.Fatalf("String with fallback failed: got %q", v)
	}

	// Test global Int
	if v := Int("TEST_INT", 0); v != 42 {
		t.Fatalf("Int failed: got %d", v)
	}
	if v := Int("MISSING", 99); v != 99 {
		t.Fatalf("Int with fallback failed: got %d", v)
	}

	// Test global Bool
	if v := Bool("TEST_BOOL", false); v != true {
		t.Fatalf("Bool failed: got %v", v)
	}
	if v := Bool("MISSING", true); v != true {
		t.Fatalf("Bool with fallback failed: got %v", v)
	}

	// Test global List
	expected := []string{"a", "b", "c"}
	if v := List("TEST_LIST", []string{"default"}); !reflect.DeepEqual(v, expected) {
		t.Fatalf("List failed: got %v", v)
	}
	if v := List("MISSING", []string{"default"}); !reflect.DeepEqual(v, []string{"default"}) {
		t.Fatalf("List with fallback failed: got %v", v)
	}
}

// prepareTestData injects a synthetic dataset into the provided Environ.
// count keys will be created with several common prefixes to exercise map/where/signed.
func prepareTestData(b *testing.B, e Environ, count int) {
	data := make(map[string]string, count)
	prefixes := []string{"APP_", "CACHE_", "DB_", "FEATURE_"}
	for i := range count {
		p := prefixes[i%len(prefixes)]
		key := p + "KEY_" + strconv.Itoa(i)
		val := "v" + strconv.Itoa(i)
		data[key] = val
	}
	// add some well-known keys for targeted lookups
	data["APP_ENV"] = "prod"
	data["APP_PORT"] = "9000"
	data["APP_DEBUG"] = "false"
	data["APP_LIST"] = "a,b,c,d,e"
	// Signed prefixes examples
	data["APP_WEB_HOST"] = "0.0.0.0"
	data["APP_WEB_TLS"] = "true"

	b.Helper()
	b.Cleanup(func() { e.Clean() })
	_ = e.Updates(data)
}

func BenchmarkLookup(b *testing.B) {
	e := Default()
	prepareTestData(b, e, 50_000)
	b.ReportAllocs()
	b.Run("exists", func(b *testing.B) {
		for b.Loop() {
			_, _ = e.Lookup("APP_ENV")
		}
	})
	b.Run("missing", func(b *testing.B) {
		for b.Loop() {
			_, _ = e.Lookup("NOT_EXISTS")
		}
	})
}

func BenchmarkGetters(b *testing.B) {
	e := Default()
	prepareTestData(b, e, 50_000)
	b.ReportAllocs()
	b.Run("String", func(b *testing.B) {
		for b.Loop() {
			_ = e.String("APP_ENV")
		}
	})
	b.Run("Int", func(b *testing.B) {
		for b.Loop() {
			_ = e.Int("APP_PORT")
		}
	})
	b.Run("Bool", func(b *testing.B) {
		for b.Loop() {
			_ = e.Bool("APP_DEBUG")
		}
	})
	b.Run("List", func(b *testing.B) {
		for b.Loop() {
			_ = e.List("APP_LIST")
		}
	})
}

func BenchmarkSigned(b *testing.B) {
	e := Default()
	prepareTestData(b, e, 100_000)
	s := e.Signed("APP", "WEB")
	b.ReportAllocs()
	for b.Loop() {
		_ = s.String("HOST")
	}
}

func BenchmarkMapPrefix(b *testing.B) {
	e := Default()
	prepareTestData(b, e, 100_000)
	b.ReportAllocs()
	for b.Loop() {
		m := e.Map("APP_")
		if len(m) == 0 {
			b.Fatalf("unexpected empty map")
		}
	}
}

func BenchmarkWhereFilter(b *testing.B) {
	e := Default()
	prepareTestData(b, e, 100_000)
	b.ReportAllocs()
	for b.Loop() {
		m := e.Where(func(name, value string) bool {
			return strings.HasPrefix(name, "CACHE_")
		})
		if len(m) == 0 {
			b.Fatalf("unexpected empty result")
		}
	}
}

// Structure for Fill benchmarks
type cfg struct {
	Port   int    `env:"APP_PORT"`
	Env    string `env:"APP_ENV"`
	Debug  bool   `env:"APP_DEBUG"`
	Hidden string // no tag
}

func BenchmarkFill(b *testing.B) {
	e := Default()
	prepareTestData(b, e, 50_000)
	b.ReportAllocs()
	for b.Loop() {
		var c cfg
		if err := e.Fill(&c); err != nil {
			b.Fatalf("Fill error: %v", err)
		}
	}
}

// ============= MISSING GLOBAL FUNCTION TESTS =============

// TestInit_Global tests the global Init function
func TestInit_Global(t *testing.T) {
	// Save original state
	origEnv := Default()
	origEnv.Clean()

	// Create a temporary .env file for testing
	tempDir := t.TempDir()
	envFile := filepath.Join(tempDir, ".env")
	content := `TEST_INIT_KEY=test_value
TEST_INIT_NUMBER=42
TEST_INIT_BOOL=true
`
	if err := os.WriteFile(envFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test .env file: %v", err)
	}

	// Change to temp directory and test Init
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Test Init function
	if err := Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Verify loaded values
	if v := String("TEST_INIT_KEY", "default"); v != "test_value" {
		t.Fatalf("Expected test_value, got %q", v)
	}
	if v := Int("TEST_INIT_NUMBER", 0); v != 42 {
		t.Fatalf("Expected 42, got %d", v)
	}
	if v := Bool("TEST_INIT_BOOL", false); v != true {
		t.Fatalf("Expected true, got %v", v)
	}

	// Test Init with specific directory
	origEnv.Clean()
	if err := InitWithDir(tempDir); err != nil {
		t.Fatalf("InitWithDir failed: %v", err)
	}

	if v := String("TEST_INIT_KEY", "default"); v != "test_value" {
		t.Fatalf("Expected test_value after InitWithDir, got %q", v)
	}

	// Cleanup
	origEnv.Clean()
}

// TestBytes_Global tests the global Bytes function
func TestBytes_Global(t *testing.T) {
	Default().Clean()

	// Setup test data
	if err := Default().Updates(map[string]string{
		"TEST_BYTES": "hello world",
		"TEST_EMPTY": "",
	}); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Test Bytes function
	if v := Bytes("TEST_BYTES", []byte("default")); !reflect.DeepEqual(v, []byte("hello world")) {
		t.Fatalf("Expected []byte(\"hello world\"), got %v", v)
	}

	// Test with fallback
	if v := Bytes("MISSING_KEY", []byte("default")); !reflect.DeepEqual(v, []byte("default")) {
		t.Fatalf("Expected []byte(\"default\"), got %v", v)
	}

	// Test empty value returns fallback
	if v := Bytes("TEST_EMPTY", []byte("fallback")); !reflect.DeepEqual(v, []byte("fallback")) {
		t.Fatalf("Expected []byte(\"fallback\") for empty value, got %v", v)
	}

	Default().Clean()
}

// TestFloat_Global tests the global Float function
func TestFloat_Global(t *testing.T) {
	Default().Clean()

	// Setup test data
	if err := Default().Updates(map[string]string{
		"TEST_FLOAT":   "3.14159",
		"TEST_FLOAT2":  "2.5",
		"TEST_INVALID": "not_a_number",
		"TEST_EMPTY":   "",
	}); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Test Float function
	if v := Float("TEST_FLOAT", 0.0); v != 3.14159 {
		t.Fatalf("Expected 3.14159, got %f", v)
	}

	// Test with fallback
	if v := Float("MISSING_KEY", 1.0); v != 1.0 {
		t.Fatalf("Expected 1.0, got %f", v)
	}

	// Test another valid float
	if v := Float("TEST_FLOAT2", 0.0); v != 2.5 {
		t.Fatalf("Expected 2.5, got %f", v)
	}

	// Test invalid number returns fallback
	if v := Float("TEST_INVALID", 9.99); v != 9.99 {
		t.Fatalf("Expected fallback 9.99 for invalid number, got %f", v)
	}

	// Test empty value returns fallback
	if v := Float("TEST_EMPTY", 8.88); v != 8.88 {
		t.Fatalf("Expected fallback 8.88 for empty value, got %f", v)
	}

	Default().Clean()
}

// TestDuration_Global tests the global Duration function
func TestDuration_Global(t *testing.T) {
	Default().Clean()

	// Setup test data
	if err := Default().Updates(map[string]string{
		"TEST_DURATION":  "100ms",
		"TEST_DURATION2": "2s",
		"TEST_DURATION3": "5m",
		"TEST_INVALID":   "not_a_duration",
		"TEST_EMPTY":     "",
	}); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Test Duration function
	if v := Duration("TEST_DURATION", 0); v != 100*time.Millisecond {
		t.Fatalf("Expected 100ms, got %v", v)
	}

	// Test with fallback
	if v := Duration("MISSING_KEY", time.Second); v != time.Second {
		t.Fatalf("Expected 1s, got %v", v)
	}

	// Test other valid durations
	if v := Duration("TEST_DURATION2", 0); v != 2*time.Second {
		t.Fatalf("Expected 2s, got %v", v)
	}
	if v := Duration("TEST_DURATION3", 0); v != 5*time.Minute {
		t.Fatalf("Expected 5m, got %v", v)
	}

	// Test invalid duration returns fallback
	if v := Duration("TEST_INVALID", 30*time.Second); v != 30*time.Second {
		t.Fatalf("Expected fallback 30s for invalid duration, got %v", v)
	}

	// Test empty value returns fallback
	if v := Duration("TEST_EMPTY", 45*time.Second); v != 45*time.Second {
		t.Fatalf("Expected fallback 45s for empty value, got %v", v)
	}

	Default().Clean()
}

// TestMap_Global tests the global Map function
func TestMap_Global(t *testing.T) {
	Default().Clean()

	// Setup test data with different prefixes
	if err := Default().Updates(map[string]string{
		"APP_NAME":    "myapp",
		"APP_VERSION": "1.0.0",
		"APP_DEBUG":   "true",
		"DB_HOST":     "localhost",
		"DB_PORT":     "5432",
		"OTHER_KEY":   "other_value",
		"APP_EMPTY":   "",
	}); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Test Map function with "APP_" prefix
	appMap := Map("APP_")
	expectedApp := map[string]string{
		"NAME":    "myapp",
		"VERSION": "1.0.0",
		"DEBUG":   "true",
		"EMPTY":   "",
	}

	if !reflect.DeepEqual(appMap, expectedApp) {
		t.Fatalf("APP_ map mismatch.\nExpected: %#v\nGot:      %#v", expectedApp, appMap)
	}

	// Test Map function with "DB_" prefix
	dbMap := Map("DB_")
	expectedDB := map[string]string{
		"HOST": "localhost",
		"PORT": "5432",
	}

	if !reflect.DeepEqual(dbMap, expectedDB) {
		t.Fatalf("DB_ map mismatch.\nExpected: %#v\nGot:      %#v", expectedDB, dbMap)
	}

	// Test with non-existent prefix
	emptyMap := Map("NONEXISTENT_")
	if len(emptyMap) != 0 {
		t.Fatalf("Expected empty map for non-existent prefix, got %v", emptyMap)
	}

	// Test with empty prefix
	allMap := Map("")
	if len(allMap) == 0 {
		t.Fatalf("Expected non-empty map for empty prefix")
	}

	Default().Clean()
}

// TestWhere_Global tests the global Where function
func TestWhere_Global(t *testing.T) {
	Default().Clean()

	// Setup test data
	if err := Default().Updates(map[string]string{
		"APP_NAME":       "myapp",
		"APP_DEBUG":      "true",
		"DB_HOST":        "localhost",
		"DB_PORT":        "5432",
		"FEATURE_FLAG_A": "enabled",
		"FEATURE_FLAG_B": "disabled",
		"VERSION":        "1.0.0",
		"EMPTY_VALUE":    "",
	}); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Test Where function to find keys starting with "APP_"
	appFilter := func(name, value string) bool {
		return strings.HasPrefix(name, "APP_")
	}
	appMap := Where(appFilter)
	expectedApp := map[string]string{
		"APP_NAME":  "myapp",
		"APP_DEBUG": "true",
	}

	if !reflect.DeepEqual(appMap, expectedApp) {
		t.Fatalf("APP_ filter mismatch.\nExpected: %#v\nGot:      %#v", expectedApp, appMap)
	}

	// Test Where function to find keys with value "enabled"
	enabledFilter := func(name, value string) bool {
		return value == "enabled"
	}
	enabledMap := Where(enabledFilter)
	expectedEnabled := map[string]string{
		"FEATURE_FLAG_A": "enabled",
	}

	if !reflect.DeepEqual(enabledMap, expectedEnabled) {
		t.Fatalf("Enabled filter mismatch.\nExpected: %#v\nGot:      %#v", expectedEnabled, enabledMap)
	}

	// Test Where function to find keys containing "FLAG"
	flagFilter := func(name, value string) bool {
		return strings.Contains(name, "FLAG")
	}
	flagMap := Where(flagFilter)
	expectedFlag := map[string]string{
		"FEATURE_FLAG_A": "enabled",
		"FEATURE_FLAG_B": "disabled",
	}

	if !reflect.DeepEqual(flagMap, expectedFlag) {
		t.Fatalf("Flag filter mismatch.\nExpected: %#v\nGot:      %#v", expectedFlag, flagMap)
	}

	// Test Where function with no matches
	noMatchFilter := func(name, value string) bool {
		return name == "NONEXISTENT"
	}
	noMatchMap := Where(noMatchFilter)
	if len(noMatchMap) != 0 {
		t.Fatalf("Expected empty map for no matches, got %v", noMatchMap)
	}

	// Test Where function to find empty values
	emptyFilter := func(name, value string) bool {
		return value == ""
	}
	emptyMap := Where(emptyFilter)
	expectedEmpty := map[string]string{
		"EMPTY_VALUE": "",
	}

	if !reflect.DeepEqual(emptyMap, expectedEmpty) {
		t.Fatalf("Empty value filter mismatch.\nExpected: %#v\nGot:      %#v", expectedEmpty, emptyMap)
	}

	Default().Clean()
}

// TestFill_Global tests the global Fill function
func TestFill_Global(t *testing.T) {
	Default().Clean()

	// Setup test data
	if err := Default().Updates(map[string]string{
		"APP_NAME":     "testapp",
		"APP_PORT":     "8080",
		"APP_DEBUG":    "true",
		"APP_TIMEOUT":  "30s",
		"APP_RATE":     "2.5",
		"NESTED_VALUE": "nested_data",
	}); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Test struct filling
	type Config struct {
		Name    string  `env:"APP_NAME"`
		Port    int     `env:"APP_PORT"`
		Debug   bool    `env:"APP_DEBUG"`
		Timeout string  `env:"APP_TIMEOUT"` // Using string since Duration needs time.Duration
		Rate    float64 `env:"APP_RATE"`
		Missing string  // No tag - should not be filled
	}

	var config Config
	if err := Fill(&config); err != nil {
		t.Fatalf("Fill failed: %v", err)
	}

	// Verify filled values
	if config.Name != "testapp" {
		t.Fatalf("Expected Name='testapp', got %q", config.Name)
	}
	if config.Port != 8080 {
		t.Fatalf("Expected Port=8080, got %d", config.Port)
	}
	if !config.Debug {
		t.Fatalf("Expected Debug=true, got %v", config.Debug)
	}
	if config.Timeout != "30s" {
		t.Fatalf("Expected Timeout='30s', got %q", config.Timeout)
	}
	if config.Rate != 2.5 {
		t.Fatalf("Expected Rate=2.5, got %f", config.Rate)
	}
	if config.Missing != "" {
		t.Fatalf("Expected Missing to be empty, got %q", config.Missing)
	}

	// Test with nested struct
	type NestedConfig struct {
		Name   string `env:"APP_NAME"`
		Nested struct {
			Value string `env:"NESTED_VALUE"`
		}
	}

	var nestedConfig NestedConfig
	if err := Fill(&nestedConfig); err != nil {
		t.Fatalf("Nested Fill failed: %v", err)
	}

	if nestedConfig.Name != "testapp" {
		t.Fatalf("Expected nested Name='testapp', got %q", nestedConfig.Name)
	}
	if nestedConfig.Nested.Value != "nested_data" {
		t.Fatalf("Expected nested Value='nested_data', got %q", nestedConfig.Nested.Value)
	}

	// Test with pointer to struct
	configPtr := &Config{}
	if err := Fill(configPtr); err != nil {
		t.Fatalf("Pointer Fill failed: %v", err)
	}

	if configPtr.Name != "testapp" {
		t.Fatalf("Expected pointer Name='testapp', got %q", configPtr.Name)
	}

	Default().Clean()
}

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
	// Is checks if APP_ENV matches
	if !Is("dev") {
		t.Fatalf("Is(dev) should be true")
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

	if v, ok := Lookup("A"); !ok || v != "1" {
		t.Fatalf("Lookup A failed")
	}
	if !Exists("A") {
		t.Fatalf("Exists A should be true")
	}
	if String("A") != "1" {
		t.Fatalf("String A")
	}
	if Int("A") != 1 {
		t.Fatalf("Int A")
	}
	if !Bool("B") {
		t.Fatalf("Bool B")
	}
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
