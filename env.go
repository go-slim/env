// Package env provides environment variable management for Go applications.
//
// # Design Philosophy: Initialize Once, Read Many
//
// This package is designed with a specific usage pattern in mind:
//
//  1. INITIALIZATION PHASE: Load environment variables once during application startup
//     (typically in main() before starting any goroutines)
//  2. RUNTIME PHASE: Read environment variables concurrently throughout the application lifecycle
//
// # Correct Usage Pattern
//
//	func main() {
//	    // ✓ INITIALIZATION: Call Init() once at startup (single-threaded)
//	    if err := env.Init(); err != nil {
//	        log.Fatal(err)
//	    }
//
//	    // ✓ BEST PRACTICE: Lock after initialization to prevent accidental writes
//	    env.Lock()
//
//	    // ✓ RUNTIME: Read operations are safe for concurrent use
//	    go func() {
//	        cache := env.Signed("CACHE", "BOOK")
//	        driver := cache.String("DRIVER")  // Safe: read-only
//	    }()
//
//	    go func() {
//	        port := env.Int("PORT", 8080)     // Safe: read-only
//	    }()
//	}
//
// # Incorrect Usage Pattern
//
//	func main() {
//	    // ✗ WRONG: Calling Init() concurrently causes data races
//	    go env.Init()  // DON'T DO THIS
//	    go env.Init()  // DON'T DO THIS
//
//	    // ✗ WRONG: Modifying environment during runtime causes data races
//	    go func() {
//	        env.Load(".env.runtime")  // DON'T DO THIS during runtime
//	    }()
//	}
//
// # Thread Safety Guarantees
//
//   - Init/InitWithDir/Load: NOT thread-safe. Call once during initialization.
//   - Lookup/String/Int/Bool/etc: Thread-safe. Safe for concurrent reads.
//   - Signed: Thread-safe. Returns a reader that is safe for concurrent use.
//   - Fill: Thread-safe for reading. Safe to call concurrently after initialization.
//
// No locks are needed because:
//   - Initialization happens in a single-threaded context
//   - Runtime access is read-only (Go slice reads are safe without locks)
//   - Environment data never changes after initialization completes
package env

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Signer is a scoped query interface for environment variables.
//
// It is useful when working with environment variables that share the same prefix
// but need to be distinguished by different scenarios. For example, when configuring
// cache settings via environment variables:
//
//	CACHE_DRIVER=redis
//	CACHE_DATABASE=1
//	CACHE_SCOPE=app:
//	CACHE_BOOK_DATABASE=10
//	CACHE_BOOK_SCOPE=app:books:
//
// You can conveniently use:
//
//	cache := env.Signed("CACHE", "BOOK")
//	cache.String("DRIVER") // redis
//	cache.Int("DATABASE")  // 10
//	cache.String("SCOPE")  // app:books:
//
// This allows easy grouping and scenario-based usage of environment variables.
type Signer interface {
	// Lookup returns the value for the specified key. The second return value is true
	// only if the environment variable exists and its value is not empty. Otherwise,
	// it returns false. This differs from the Exists method.
	Lookup(key string) (string, bool)
	// Exists checks whether the specified key exists.
	// Returns true if the key exists, false otherwise.
	Exists(key string) bool
	// String returns the string value for the specified key, or the fallback value
	// if the key does not exist or the value is empty.
	String(key string, fallback ...string) string
	// Bytes returns the byte slice value for the specified key, or the fallback value
	// if the key does not exist or the value is empty.
	Bytes(key string, fallback ...[]byte) []byte
	// Int returns the integer value for the specified key, or the fallback value
	// if the key does not exist or the value is empty.
	Int(key string, fallback ...int) int
	// Duration returns the duration value for the specified key, or the fallback value
	// if the key does not exist or the value is empty.
	Duration(key string, fallback ...time.Duration) time.Duration
	// Bool returns the boolean value for the specified key, or the fallback value
	// if the key does not exist or the value is empty.
	Bool(key string, fallback ...bool) bool
	// List returns the string list (comma-separated) for the specified key, or the fallback value
	// if the key does not exist or the value is empty.
	List(key string, fallback ...[]string) []string
	// Map aggregates and returns data for keys with the same prefix.
	Map(prefix string) map[string]string
	// Where returns data filtered by a custom function.
	Where(filter func(name, value string) bool) map[string]string
	// Fill populates a struct with environment variables.
	Fill(structure any) error
}

type Environ interface {
	Signer
	// Read parses environment variables from an io.Reader.
	Read(r io.Reader) error
	// Load loads environment variable files.
	Load(filenames ...string) error
	// Signed returns a Signer that follows a specific prefix and category rule.
	Signed(prefix, category string) Signer
	// Clean clears all cached data.
	Clean()
	// Lock prevents further write operations (Save/Load/Read/Clean).
	// After calling Lock, any attempt to modify the environment will result in
	// a panic or error. This should be called after initialization is complete
	// to enforce the "read-only" contract during runtime.
	Lock()
}

var (
	// env is the global cached environment variable storage.
	env = New().(*environ)
	// root is the directory where the `.env` file is located.
	// Typically the working directory of the program.
	root string
)

// Default returns the default Environ instance.
//
// Thread Safety: The returned Environ is safe for concurrent READ operations
// after initialization. Do not call Save/Load methods concurrently.
func Default() Environ {
	return env
}

// Init loads .env files from the runtime directory.
//
// # Initialization Function - NOT Thread-Safe
//
// This function modifies global state and MUST be called:
//   - Once during application startup (typically in main())
//   - Before any goroutines are started
//   - In a single-threaded context
//
// Concurrent calls to Init/InitWithDir will cause data races and undefined behavior.
//
// After Init completes successfully, all read operations (Lookup, String, Int, etc.)
// are safe for concurrent use across multiple goroutines.
//
// Example:
//
//	func main() {
//	    // ✓ CORRECT: Call once at startup
//	    if err := env.Init(); err != nil {
//	        log.Fatal(err)
//	    }
//
//	    // Now safe to use concurrently
//	    go worker1()
//	    go worker2()
//	}
//
//	// ✗ WRONG: Never do this
//	func main() {
//	    go env.Init()  // Data race!
//	    go env.Init()  // Data race!
//	}
func Init(root ...string) error {
	var dir string
	if len(root) > 0 {
		dir = root[0]
	}
	if dir == "" {
		dir = "."
	}
	return InitWithDir(dir)
}

// InitWithDir loads .env files from the specified directory.
//
// # Initialization Function - NOT Thread-Safe
//
// This function modifies global state. See Init() documentation for usage guidelines.
// Must be called once during initialization, not concurrently.
func InitWithDir(dir string) (err error) {
	dir, err = filepath.Abs(dir)
	if err != nil {
		return
	}

	defer func() {
		if err != nil {
			root = ""
			env.Clean()
		} else {
			root = dir
		}
	}()

	// Reset cached environment variables
	root = ""
	env.Clean()

	// Load system environment variables
	result := make(map[string]string)
	for _, value := range os.Environ() {
		parts := strings.SplitN(value, "=", 2)
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		result[key] = val
	}
	env.Save(result)

	// Load .env and .env.local files
	err = loadEnv(dir, "")
	if err != nil {
		return err
	}

	// Load environment-specific variables
	appEnv := String("APP_ENV", "prod")
	if len(appEnv) > 0 {
		env.Save(map[string]string{
			"APP_ENV": appEnv,
		})
		// Load .env.{APP_ENV} and .env.{APP_ENV}.local files
		err = loadEnv(dir, "."+strings.ToLower(appEnv))
		if err != nil {
			return err
		}
	}

	return
}

func loadEnv(dir, env string) error {
	filename := filepath.Join(dir, ".env"+env)
	if err := Load(filename); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := Load(filename + ".local"); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// Load loads the specified environment variable files.
//
// # Initialization Function - NOT Thread-Safe
//
// This function modifies the global environment state. It should only be called
// during the initialization phase, not during runtime when multiple goroutines
// are accessing environment variables.
//
// Typical usage: call during startup to load additional .env files.
// Returns ErrLocked if Lock() has been called.
func Load(filenames ...string) error {
	return env.Load(filenames...)
}

// Lock prevents further write operations on the global environment.
//
// After calling Lock(), any attempt to call Init(), Load(), or modify the
// global environment will result in errors or panics. This enforces the
// "read-only" contract during the runtime phase.
//
// Best Practice: Call Lock() immediately after initialization to prevent
// accidental modifications during runtime.
//
// Example:
//
//	func main() {
//	    // Initialization phase
//	    if err := env.Init(); err != nil {
//	        log.Fatal(err)
//	    }
//
//	    // Lock to prevent further modifications
//	    env.Lock()
//
//	    // Runtime phase - only reads allowed
//	    go worker1()
//	    go worker2()
//	}
//
// Lock is idempotent - calling it multiple times is safe.
// Once locked, the global environment cannot be unlocked.
func Lock() {
	env.Lock()
}

// Signed returns a Signer scoped by the given prefix and category.
//
// Thread Safety: This function is safe to call concurrently after initialization.
// The returned Signer is safe for concurrent read operations.
func Signed(prefix, category string) Signer {
	return env.Signed(prefix, category)
}

// Path returns a path relative to the initialization directory.
func Path(path ...string) string {
	switch len(path) {
	case 0:
		return root
	case 1:
		return filepath.Join(root, path[0])
	default:
		return filepath.Join(root, filepath.Join(path...))
	}
}

// Is checks if the application environment matches any of the provided values.
func Is(env ...string) bool {
	if len(env) == 0 {
		return false
	}
	if val, exists := Lookup("APP_ENV"); exists {
		for _, s := range env {
			if val == s {
				return true
			}
		}
	}
	return false
}

// Inject attempts to inject data into the Environ instance.
func Inject(env Environ, data map[string]string) bool {
	if env == nil || len(data) == 0 {
		return false
	}
	if s, ok := env.(interface{ Save(data map[string]string) }); ok {
		s.Save(data)
		return true
	}
	return false
}

// Lookup retrieves the value of an environment variable.
//
// Thread Safety: Safe for concurrent use after initialization.
func Lookup(name string) (string, bool) {
	return env.Lookup(name)
}

// Exists checks if an environment variable exists.
//
// Thread Safety: Safe for concurrent use after initialization.
func Exists(name string) bool {
	return env.Exists(name)
}

// String retrieves the string value of an environment variable.
//
// Thread Safety: Safe for concurrent use after initialization.
func String(name string, value ...string) string {
	return env.String(name, value...)
}

// Bytes retrieves the byte slice value of an environment variable.
//
// Thread Safety: Safe for concurrent use after initialization.
func Bytes(name string, value ...[]byte) []byte {
	return env.Bytes(name, value...)
}

// Int retrieves the integer value of an environment variable.
//
// Thread Safety: Safe for concurrent use after initialization.
func Int(name string, value ...int) int {
	return env.Int(name, value...)
}

// Float retrieves the float64 value of an environment variable.
//
// Thread Safety: Safe for concurrent use after initialization.
func Float(name string, value ...float64) float64 {
	return env.Float(name, value...)
}

// Duration retrieves the duration value of an environment variable.
//
// Thread Safety: Safe for concurrent use after initialization.
func Duration(name string, value ...time.Duration) time.Duration {
	return env.Duration(name, value...)
}

// Bool retrieves the boolean value of an environment variable.
//
// Thread Safety: Safe for concurrent use after initialization.
func Bool(name string, value ...bool) bool {
	return env.Bool(name, value...)
}

// List splits the value by comma and returns a string slice.
//
// Thread Safety: Safe for concurrent use after initialization.
func List(name string, fallback ...[]string) []string {
	return env.List(name, fallback...)
}

// Map returns all environment variables with the given prefix.
//
// Thread Safety: Safe for concurrent use after initialization.
func Map(prefix string) map[string]string {
	return env.Map(prefix)
}

// Where returns all environment variables that match the filter function.
//
// Thread Safety: Safe for concurrent use after initialization.
func Where(filter func(name string, value string) bool) map[string]string {
	return env.Where(filter)
}

// Fill populates the specified struct with environment variables.
//
// Thread Safety: Safe for concurrent use after initialization.
func Fill(structure any) error {
	return env.Fill(structure)
}

// All returns all environment variables.
//
// Thread Safety: Safe for concurrent use after initialization.
func All() map[string]string {
	return env.Where(func(name, value string) bool {
		return true
	})
}
