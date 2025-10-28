package env

import (
	"errors"
	"strings"
	"testing"
)

// TestLock_PreventsSave tests that Save panics after Lock
func TestLock_PreventsSave(t *testing.T) {
	e := New()
	e.(*environ).Save(map[string]string{"KEY": "value"})

	// Lock the environ
	e.Lock()

	// Attempt to Save should panic
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Expected Save to panic after Lock, but it didn't")
		} else {
			msg := r.(string)
			if !strings.Contains(msg, "locked") {
				t.Fatalf("Expected panic message to mention 'locked', got: %v", r)
			}
		}
	}()

	e.(*environ).Save(map[string]string{"KEY2": "value2"})
}

// TestLock_PreventsLoad tests that Load returns error after Lock
func TestLock_PreventsLoad(t *testing.T) {
	e := New()

	// Lock the environ
	e.Lock()

	// Attempt to Load should return ErrLocked
	err := e.Load(".env")
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("Expected ErrLocked, got: %v", err)
	}
}

// TestLock_PreventsRead tests that Read returns error after Lock
func TestLock_PreventsRead(t *testing.T) {
	e := New()

	// Lock the environ
	e.Lock()

	// Attempt to Read should return ErrLocked
	err := e.Read(strings.NewReader("KEY=value"))
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("Expected ErrLocked, got: %v", err)
	}
}

// TestLock_PreventsClean tests that Clean panics after Lock
func TestLock_PreventsClean(t *testing.T) {
	e := New()
	e.(*environ).Save(map[string]string{"KEY": "value"})

	// Lock the environ
	e.Lock()

	// Attempt to Clean should panic
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Expected Clean to panic after Lock, but it didn't")
		} else {
			msg := r.(string)
			if !strings.Contains(msg, "locked") {
				t.Fatalf("Expected panic message to mention 'locked', got: %v", r)
			}
		}
	}()

	e.Clean()
}

// TestLock_AllowsReads tests that read operations still work after Lock
func TestLock_AllowsReads(t *testing.T) {
	e := New()
	e.(*environ).Save(map[string]string{
		"KEY1": "value1",
		"KEY2": "value2",
	})

	// Lock the environ
	e.Lock()

	// All read operations should still work
	if val, ok := e.Lookup("KEY1"); !ok || val != "value1" {
		t.Fatalf("Lookup failed after Lock: got %q, %v", val, ok)
	}

	if !e.Exists("KEY2") {
		t.Fatal("Exists failed after Lock")
	}

	if val := e.String("KEY1"); val != "value1" {
		t.Fatalf("String failed after Lock: got %q", val)
	}

	// Map should work
	m := e.Map("")
	if len(m) != 2 {
		t.Fatalf("Map failed after Lock: got %d items", len(m))
	}

	// Signed should work
	s := e.Signed("", "")
	if val := s.String("KEY1"); val != "value1" {
		t.Fatalf("Signed lookup failed after Lock: got %q", val)
	}
}

// TestLock_Idempotent tests that calling Lock multiple times is safe
func TestLock_Idempotent(t *testing.T) {
	e := New()
	e.(*environ).Save(map[string]string{"KEY": "value"})

	// Lock multiple times
	e.Lock()
	e.Lock()
	e.Lock()

	// Should still be locked
	err := e.Load(".env")
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("Expected ErrLocked after multiple Lock calls, got: %v", err)
	}

	// Reads should still work
	if val := e.String("KEY"); val != "value" {
		t.Fatalf("String failed after multiple Locks: got %q", val)
	}
}

// TestGlobalLock tests the global Lock function
func TestGlobalLock(t *testing.T) {
	// Save original state
	origEnv := env
	defer func() { env = origEnv }()

	// Create new global env for this test
	env = New().(*environ)
	env.Save(map[string]string{"TEST_KEY": "test_value"})

	// Lock globally
	Lock()

	// Attempt to Load should return ErrLocked
	err := Load(".env")
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("Expected ErrLocked after global Lock, got: %v", err)
	}

	// Reads should still work
	if val := String("TEST_KEY"); val != "test_value" {
		t.Fatalf("String failed after global Lock: got %q", val)
	}
}

// TestLock_CorrectUsagePattern demonstrates the recommended pattern
func TestLock_CorrectUsagePattern(t *testing.T) {
	e := New()

	// Phase 1: Initialization (single-threaded)
	e.(*environ).Save(map[string]string{
		"APP_NAME": "myapp",
		"PORT":     "8080",
	})

	// Additional loads during init
	_ = e.Read(strings.NewReader("DEBUG=true"))

	// Phase 2: Lock after initialization
	e.Lock()

	// Phase 3: Runtime (read-only, can be concurrent)
	// All read operations work fine
	if val := e.String("APP_NAME"); val != "myapp" {
		t.Fatalf("Unexpected value: %q", val)
	}

	// But writes are prevented
	err := e.Load(".env")
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("Expected writes to be prevented after Lock")
	}
}
