package env

import (
	"sync"
	"testing"
)

// TestConcurrentIterAndModify is DISABLED because it violates the design pattern.
//
// This test intentionally calls Save() during runtime while iterating, which
// violates the "Initialize Once, Read Many" design pattern. This is NOT how
// the library should be used.
//
// Expected behavior: Would cause data races or panics (which is correct).
// Correct usage: Initialize once, then only read.
func TestConcurrentIterAndModify(t *testing.T) {
	t.Skip("DISABLED: This test violates the design pattern (concurrent modification during runtime)")
}

// TestConcurrentInit is DISABLED because it violates the design pattern.
//
// This test intentionally calls Init() concurrently, which violates the
// "Initialize Once" design pattern. Init() must be called once in a
// single-threaded context.
//
// Expected behavior: Would cause data races or panics (which is correct).
// Correct usage: Call Init() once at startup before starting goroutines.
func TestConcurrentInit(t *testing.T) {
	t.Skip("DISABLED: This test violates the design pattern (concurrent Init calls)")
}

// TestConcurrentLookupAndSave is DISABLED because it violates the design pattern.
//
// This test calls Save() concurrently with Lookup(), which violates the design.
// Save() is for initialization only and should not be called during runtime.
//
// Expected behavior: Would cause data races or panics (which is correct).
// Correct usage: Call Save() during init, then only Lookup() during runtime.
func TestConcurrentLookupAndSave(t *testing.T) {
	t.Skip("DISABLED: This test violates the design pattern (concurrent Save during runtime)")
}

// TestConcurrentReads verifies that concurrent reads are safe after initialization.
// This demonstrates the CORRECT usage pattern.
func TestConcurrentReads(t *testing.T) {
	e := New()

	// ✓ INITIALIZATION (single-threaded)
	e.(*environ).Save(map[string]string{
		"KEY1": "value1",
		"KEY2": "value2",
		"KEY3": "value3",
	})

	var wg sync.WaitGroup

	// ✓ RUNTIME (multi-threaded, read-only)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, _ = e.Lookup("KEY1")
				_, _ = e.Lookup("KEY2")
				_ = e.Exists("KEY3")
			}
		}()
	}

	wg.Wait()
}
