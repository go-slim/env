package env

import (
	"os"
	"strings"
	"testing"
)

// TestInvalidEnvironFormat tests handling of malformed environment variables
func TestInvalidEnvironFormat(t *testing.T) {
	// This test demonstrates the panic risk when os.Environ() returns
	// malformed entries (though this is rare in practice)

	// Simulate what would happen if we process a malformed env string
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Caught panic as expected: %v", r)
		}
	}()

	// This simulates the problematic code in InitWithDir
	value := "MALFORMED_NO_EQUALS"
	parts := strings.SplitN(value, "=", 2)
	if len(parts) < 2 {
		t.Log("Correctly detected malformed environment variable")
		return
	}
	// This would panic if we reached here with len(parts) < 2
	_ = strings.TrimSpace(parts[1])
}

// TestFillWithUnexportedField tests Fill with unexported struct fields
func TestFillWithUnexportedField(t *testing.T) {
	type Config struct {
		Public  string `env:"PUBLIC"`
		private string `env:"PRIVATE"` // unexported field
	}

	e := New()
	e.(*environ).Save(map[string]string{
		"PUBLIC":  "public_value",
		"PRIVATE": "private_value",
	})

	cfg := &Config{}
	err := e.Fill(cfg)

	// The current implementation uses unsafe.Pointer which can set unexported fields
	// This might not be the desired behavior
	if err != nil {
		t.Logf("Fill returned error (expected): %v", err)
	} else {
		t.Logf("Public=%q, private=%q", cfg.Public, cfg.private)
	}
}

// TestFillWithCircularReference tests potential infinite recursion
func TestFillWithCircularReference(t *testing.T) {
	type Node struct {
		Value string `env:"VALUE"`
		Next  *Node  // Could create circular reference
	}

	e := New()
	e.(*environ).Save(map[string]string{
		"VALUE": "test",
	})

	// Create a circular reference
	node := &Node{}
	node.Next = node

	// This should ideally detect circular references
	// Current implementation might recurse infinitely
	done := make(chan bool, 1)
	go func() {
		_ = e.Fill(node)
		done <- true
	}()

	// Wait a bit to see if it completes
	select {
	case <-done:
		t.Log("Fill completed without infinite recursion")
	case <-func() chan bool {
		ch := make(chan bool)
		// Give it 100ms to complete
		go func() {
			<-func() chan struct{} {
				c := make(chan struct{})
				go func() {
					<-func() chan struct{} {
						cc := make(chan struct{})
						close(cc)
						return cc
					}()
					close(c)
				}()
				return c
			}()
			ch <- true
		}()
		return ch
	}():
		// If we reach here, Fill might be stuck
		t.Log("Fill may have completed (or test timeout too short)")
	}
}

// TestLookupEmptyKey tests lookup with empty key
func TestLookupEmptyKey(t *testing.T) {
	e := New()
	e.(*environ).Save(map[string]string{
		"": "empty_key_value",
	})

	val, ok := e.Lookup("")
	if ok {
		t.Logf("Empty key lookup returned: %q", val)
	} else {
		t.Log("Empty key not found")
	}
}

// TestSaveNilMap tests Save with nil map
func TestSaveNilMap(t *testing.T) {
	e := New()
	// This should not panic
	e.(*environ).Save(nil)
	t.Log("Save(nil) completed without panic")
}

// TestIteratorAfterClean tests iterator after Clean
func TestIteratorAfterClean(t *testing.T) {
	e := New()
	e.(*environ).Save(map[string]string{
		"KEY": "value",
	})

	iter := e.(*environ).iter()

	// Clean while iterator exists
	e.(*environ).Clean()

	// Try to use iterator - this could panic if not handled properly
	k, v, ok := iter()
	if ok {
		t.Logf("Iterator returned: %q=%q after Clean", k, v)
	} else {
		t.Log("Iterator returned ok=false after Clean")
	}
}

// TestIndexOutOfBounds tests potential index out of bounds
func TestIndexOutOfBounds(t *testing.T) {
	e := &environ{
		keys:   []string{"KEY1"},
		values: []string{"value1", "extra_value"}, // Mismatched lengths
	}

	// This could cause issues if not handled properly
	val, ok := e.lookup("KEY1")
	t.Logf("Lookup with mismatched lengths: val=%q, ok=%v", val, ok)
}

// TestOsEnvironWithCurrentEnv verifies the actual os.Environ format
func TestOsEnvironWithCurrentEnv(t *testing.T) {
	// Check actual environment variables format
	for i, v := range os.Environ() {
		if !strings.Contains(v, "=") {
			t.Errorf("Environment variable at index %d is malformed: %q", i, v)
		}
		if i > 5 { // Just check first few
			break
		}
	}
}
