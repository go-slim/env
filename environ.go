package env

import (
	"errors"
	"io"
	"sync/atomic"

	"github.com/joho/godotenv"
)

var _ Signer = (*environ)(nil)

// ErrLocked is returned when attempting to modify a locked Environ.
var ErrLocked = errors.New("env: environ is locked, write operations are not allowed after Lock() is called")

// environ is the internal implementation of the Environ interface.
//
// Thread Safety Model:
//   - WRITE operations (Updates, Load, Read, Clean): NOT thread-safe, for initialization only
//   - READ operations (lookup, exists, iter): Thread-safe AFTER initialization completes
//
// Design Intent:
//   - Initialize once: Call Updates/Load/Read during application startup (single-threaded)
//   - Read many: All read operations are safe after initialization (multi-threaded, read-only)
//
// No locks are needed because:
//   - Initialization is single-threaded (no concurrent writes)
//   - Runtime is read-only (Go slice reads are safe without locks when no writes occur)
//   - keys and values slices are never modified after initialization
//
// The locked field enforces the read-only contract at runtime. Call Lock() after
// initialization to prevent accidental writes during the runtime phase.
type environ struct {
	inner
	keys   []string    // Environment variable keys (read-only after init)
	values []string    // Environment variable values (read-only after init, parallel to keys)
	locked atomic.Bool // Prevents write operations after Lock() is called
}

// New creates a new Environ instance.
//
// The returned Environ follows the same thread-safety model as the package:
// - Write operations (Updates, Load): Use during initialization only
// - Read operations (Lookup, String, etc.): Safe for concurrent use after initialization
func New() Environ {
	e := &environ{}
	e.inner.lookup = e.lookup
	e.inner.exists = e.exists
	e.inner.iter = e.iter
	return e
}

// Read parses environment variables from an io.Reader.
//
// NOT Thread-Safe: This is an initialization function. Do not call during runtime.
// Returns ErrLocked if Lock() has been called.
func (e *environ) Read(r io.Reader) error {
	if e.locked.Load() {
		return ErrLocked
	}
	data, err := godotenv.Parse(r)
	if err == nil {
		e.Updates(data)
	}
	return err
}

// Load loads environment variable files.
//
// NOT Thread-Safe: This is an initialization function. Do not call during runtime.
// Returns ErrLocked if Lock() has been called.
func (e *environ) Load(filenames ...string) error {
	if e.locked.Load() {
		return ErrLocked
	}
	data, err := godotenv.Read(filenames...)
	if err == nil {
		e.Updates(data)
	}
	return err
}

// Updates stores data into the cached environment variables.
//
// NOT Thread-Safe: This is an initialization function. Do not call during runtime.
// Must be called in a single-threaded context during application startup.
// Panics if Lock() has been called.
func (e *environ) Updates(data map[string]string) {
	if e.locked.Load() {
		panic("env: cannot call Updates() after Lock() - environ is locked for write operations")
	}
	for key, value := range data {
		if i := e.index(key); i > -1 {
			e.values[i] = value
		} else {
			e.keys = append(e.keys, key)
			e.values = append(e.values, value)
		}
	}
}

func (e *environ) Signed(prefix, category string) Signer {
	return newSigner(prefix, category, e)
}

func (e *environ) index(key string) int {
	if e.keys != nil {
		for i, s := range e.keys {
			if s == key {
				return i
			}
		}
	}
	return -1
}

// lookup retrieves the environment variable value. If it does not exist or is empty,
// the second return value is false.
//
// Thread Safety: Safe for concurrent use after initialization (read-only access).
func (e *environ) lookup(key string) (string, bool) {
	if i := e.index(key); i > -1 {
		v := e.values[i]
		return v, len(v) > 0
	}
	return "", false
}

// exists checks whether an environment variable exists.
//
// Thread Safety: Safe for concurrent use after initialization (read-only access).
func (e *environ) exists(key string) bool {
	return e.index(key) > -1
}

// iter returns an iterator function over environment variables.
//
// Thread Safety: Safe for concurrent use AFTER initialization completes.
//
// Implementation Note:
// This function returns a closure that uses atomic operations to track position.
// The returned iterator is safe for concurrent use because:
//
//  1. Initialization Phase: e.keys/e.values are populated (single-threaded)
//  2. Runtime Phase: e.keys/e.values are read-only (never modified)
//  3. Atomic position counter ensures each goroutine gets unique indices
//
// Multiple goroutines can safely call the same iterator concurrently because:
//   - atomic.AddInt32 ensures unique position values
//   - Slice reads are safe in Go when no concurrent writes occur
//   - e.keys and e.values are never modified after initialization
//
// WARNING: If Save/Load are called during runtime (violating the design pattern),
// this will cause data races. Don't do that.
func (e *environ) iter() func() (key string, value string, ok bool) {
	var pos int32 = -1
	return func() (key string, value string, ok bool) {
		index := int(atomic.AddInt32(&pos, 1))
		if index >= len(e.keys) {
			return "", "", false
		}
		return e.keys[index], e.values[index], true
	}
}

// Clean clears all cached environment variables.
//
// NOT Thread-Safe: This is an initialization/cleanup function.
// Typically called during testing or re-initialization. Do not call during
// normal runtime when other goroutines are reading environment variables.
// Must be called in a single-threaded context.
// Panics if Lock() has been called.
func (e *environ) Clean() {
	if e.locked.Load() {
		panic("env: cannot call Clean() after Lock() - environ is locked for write operations")
	}
	e.keys = nil
	e.values = nil
}

// Lock prevents further write operations on this Environ.
//
// After calling Lock(), any attempt to call Updates(), Load(), Read(), or Clean()
// will result in an error (for Load/Read) or panic (for Updates/Clean).
//
// This should be called after initialization is complete to enforce the
// "read-only" contract during the runtime phase.
//
// Example:
//
//	func main() {
//	    env.Init()           // Initialize
//	    env.Lock()           // Lock for runtime
//	    // Now any write attempt will fail
//	    go worker()          // Safe: only reads
//	}
//
// Lock is idempotent - calling it multiple times is safe.
// Once locked, an Environ cannot be unlocked.
func (e *environ) Lock() {
	e.locked.Store(true)
}
