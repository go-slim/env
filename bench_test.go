package env

import (
	"strconv"
	"strings"
	"testing"
)

// prepareTestData injects a synthetic dataset into the provided Environ.
// count keys will be created with several common prefixes to exercise map/where/signed.
func prepareTestData(b *testing.B, e Environ, count int) {
	data := make(map[string]string, count)
	prefixes := []string{"APP_", "CACHE_", "DB_", "FEATURE_"}
	for i := 0; i < count; i++ {
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
	_ = Inject(e, data)
}

func BenchmarkLookup(b *testing.B) {
	e := Default()
	prepareTestData(b, e, 50_000)
	b.ReportAllocs()
	b.Run("exists", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = e.Lookup("APP_ENV")
		}
	})
	b.Run("missing", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = e.Lookup("NOT_EXISTS")
		}
	})
}

func BenchmarkGetters(b *testing.B) {
	e := Default()
	prepareTestData(b, e, 50_000)
	b.ReportAllocs()
	b.Run("String", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = e.String("APP_ENV")
		}
	})
	b.Run("Int", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = e.Int("APP_PORT")
		}
	})
	b.Run("Bool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = e.Bool("APP_DEBUG")
		}
	})
	b.Run("List", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = e.List("APP_LIST")
		}
	})
}

func BenchmarkSigned(b *testing.B) {
	e := Default()
	prepareTestData(b, e, 100_000)
	s := e.Signed("APP", "WEB")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = s.String("HOST")
	}
}

func BenchmarkMapPrefix(b *testing.B) {
	e := Default()
	prepareTestData(b, e, 100_000)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
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
	for i := 0; i < b.N; i++ {
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
	for i := 0; i < b.N; i++ {
		var c cfg
		if err := e.Fill(&c); err != nil {
			b.Fatalf("Fill error: %v", err)
		}
	}
}
