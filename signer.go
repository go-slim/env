package env

import "strings"

var _ Signer = (*signer)(nil)

// signer implements the Signer interface with scoped environment variable lookups.
//
// It provides a two-level hierarchy for organizing environment variables:
// - prefix: The top-level grouping (e.g., "CACHE", "DATABASE")
// - category: An optional subcategory for more specific scoping (e.g., "BOOK", "USER")
//
// The lookup strategy follows a fallback pattern:
//  1. First tries: PREFIX_CATEGORY_KEY
//  2. Falls back to: PREFIX_KEY (if category is set but specific key not found)
//
// This allows you to define common settings at the prefix level and override
// them for specific categories without duplicating configuration.
//
// Example:
//
//	Given environment variables:
//	  CACHE_DRIVER=redis
//	  CACHE_DATABASE=1
//	  CACHE_BOOK_DATABASE=10
//
//	Using env.Signed("CACHE", "BOOK"):
//	  .String("DRIVER")   → "redis" (from CACHE_DRIVER, fallback)
//	  .Int("DATABASE")    → 10 (from CACHE_BOOK_DATABASE, category-specific)
type signer struct {
	inner
	prefix   string   // The prefix for grouping related variables (e.g., "CACHE")
	category string   // Optional category for sub-grouping (e.g., "BOOK")
	environ  *environ // Reference to the underlying environment storage
}

// newSigner creates a new Signer instance with the specified prefix and category.
//
// Parameters:
//   - prefix: The top-level prefix for variable grouping (can be empty)
//   - category: The subcategory for more specific scoping (can be empty)
//   - environ: The environment variable storage to query
//
// Returns a Signer that automatically scopes all lookups using the prefix/category pattern.
func newSigner(prefix, category string, environ *environ) Signer {
	s := &signer{
		prefix:   prefix,
		category: category,
		environ:  environ,
	}
	s.inner.lookup = s.lookup
	s.inner.exists = s.exists
	s.inner.iter = s.iter
	return s
}

// lookup retrieves the value for the given key using the signer's scoped lookup strategy.
//
// Lookup Order:
//  1. If category is set, first tries: PREFIX_CATEGORY_KEY
//  2. If not found (or category is empty), falls back to: PREFIX_KEY
//
// Example with prefix="CACHE", category="BOOK", key="DATABASE":
//  1. First looks for: CACHE_BOOK_DATABASE
//  2. If not found, looks for: CACHE_DATABASE
//
// Returns:
//   - value: The environment variable value (empty string if not found)
//   - exists: true only if the variable exists and has a non-empty value
func (s *signer) lookup(key string) (string, bool) {
	// Use prefix as the group and category as the subcategory,
	// forming a key name like prefix_category_key
	value, exists := s.lookup2(s.category, key)
	if exists || s.category == "" {
		return value, exists
	}
	// When unable to find data by category,
	// use prefix_key as the fallback
	return s.lookup2("", key)
}

// lookup2 is a helper that constructs the full environment variable key and performs the lookup.
//
// It builds the key by combining: [prefix]_[category]_[key]
// If category is empty, it builds: [prefix]_[key]
// If prefix is also empty, it uses just: [key]
func (s *signer) lookup2(category, key string) (string, bool) {
	if category != "" {
		key = category + "_" + key
	}
	if s.prefix != "" {
		key = s.prefix + "_" + key
	}
	return s.environ.Lookup(key)
}

// exists checks whether the given key exists using the signer's scoped lookup strategy.
//
// Unlike lookup, exists returns true as long as the key is defined, even if the value is empty.
//
// Lookup Order:
//  1. If category is set, first checks: PREFIX_CATEGORY_KEY
//  2. If not found (or category is empty), falls back to: PREFIX_KEY
//
// Example with prefix="CACHE", category="BOOK", key="DATABASE":
//  1. First checks for: CACHE_BOOK_DATABASE
//  2. If not found, checks for: CACHE_DATABASE
//
// Returns true if the variable exists at any level, false otherwise.
func (s *signer) exists(key string) bool {
	// Use prefix as the group and category as the subcategory,
	// forming a key name like prefix_category_key
	exists := s.exists2(s.category, key)
	if exists || s.category == "" {
		return exists
	}
	// When unable to determine data existence by category,
	// use prefix_key as the fallback
	return s.exists2("", key)
}

// exists2 is a helper that constructs the full environment variable key and checks existence.
//
// It builds the key using the same pattern as lookup2 and checks if it exists in the environ.
func (s *signer) exists2(category, key string) bool {
	if category != "" {
		key = category + "_" + key
	}
	if s.prefix != "" {
		key = s.prefix + "_" + key
	}
	return s.environ.Exists(key)
}

// iter returns an iterator function that provides scoped iteration over environment variables.
//
// The iteration follows a two-phase approach to implement the fallback behavior:
//
// Phase 1: Category-specific iteration
//   - Returns all variables matching PREFIX_CATEGORY_* (with prefix stripped)
//   - Simultaneously buffers variables matching PREFIX_* for fallback
//
// Phase 2: Fallback iteration
//   - After category-specific variables are exhausted, returns buffered PREFIX_* variables
//   - These represent fallback values that weren't overridden by category-specific ones
//
// Example with prefix="CACHE", category="BOOK":
//
//	Given environment:
//	  CACHE_DRIVER=redis
//	  CACHE_DATABASE=1
//	  CACHE_BOOK_DATABASE=10
//	  CACHE_BOOK_SCOPE=books:
//
//	Iteration yields (in order):
//	  1. ("DATABASE", "10", true)       ← from CACHE_BOOK_DATABASE (category-specific)
//	  2. ("SCOPE", "books:", true)      ← from CACHE_BOOK_SCOPE (category-specific)
//	  3. ("DRIVER", "redis", true)      ← from CACHE_DRIVER (fallback, buffered)
//	  4. ("DATABASE", "1", true)        ← from CACHE_DATABASE (fallback, buffered)
//	  5. ("", "", false)                ← iteration complete
//
// Note: Methods like Map() and Where() use this iterator to build result sets.
// The category-specific values appear first, followed by prefix-level fallbacks.
func (s *signer) iter() func() (key string, value string, ok bool) {
	next := s.environ.inner.iter()
	// Build full prefix (prefix_ + category_)
	// This is used to match category-specific variables
	fullPrefix := s.prefix
	if fullPrefix != "" {
		fullPrefix += "_"
	}
	if s.category != "" {
		fullPrefix += s.category + "_"
	}
	// Root-level prefix for fallback buffering (prefix_)
	// This is used to buffer prefix-level variables for fallback
	rootPrefix := s.prefix
	if rootPrefix != "" {
		rootPrefix += "_"
	}
	var keys, values []string
	var index int
	return func() (key string, value string, ok bool) {
		// Phase 1: Process underlying iterator
		if next != nil {
			for {
				k, v, b := next()
				if !b {
					// Switch to buffered results (Phase 2)
					next = nil
					break
				}
				// Return category-specific variables immediately
				if after, ok0 := strings.CutPrefix(k, fullPrefix); ok0 {
					return after, v, true
				}
				// Buffer keys with root prefix for fallback (only when prefix is set)
				if rootPrefix != "" && strings.HasPrefix(k, rootPrefix) {
					keys = append(keys, strings.TrimPrefix(k, rootPrefix))
					values = append(values, v)
				}
			}
		}
		// Phase 2: Return buffered fallback values
		if index >= len(keys) {
			return "", "", false
		}
		k := keys[index]
		v := values[index]
		index++
		return k, v, true
	}
}
