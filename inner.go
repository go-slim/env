package env

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"go-slim.dev/cast"
)

var _ Signer = (*inner)(nil)

type inner struct {
	lookup func(key string) (string, bool)
	exists func(key string) bool
	iter   func() func() (key string, value string, ok bool)
}

func (i *inner) Lookup(key string) (string, bool) {
	return i.lookup(key)
}

func (i *inner) Exists(key string) bool {
	return i.exists(key)
}

// String retrieves the string value.
func (i *inner) String(key string, fallback ...string) string {
	if value, exists := i.Lookup(key); exists {
		return value
	}
	for _, value := range fallback {
		return value
	}
	return ""
}

// Bytes retrieves the byte slice value.
func (i *inner) Bytes(key string, fallback ...[]byte) []byte {
	if value, exists := i.Lookup(key); exists {
		return []byte(value)
	}
	for _, bytes := range fallback {
		return bytes
	}
	return []byte{}
}

// Int retrieves the integer value.
func (i *inner) Int(key string, fallback ...int) int {
	if val, exists := i.Lookup(key); exists {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	for _, value := range fallback {
		return value
	}
	return 0
}

func (i *inner) Float(key string, fallback ...float64) float64 {
	if val, exists := i.Lookup(key); exists {
		if n, err := strconv.ParseFloat(val, 64); err == nil {
			return n
		}
	}
	for _, value := range fallback {
		return value
	}
	return 0
}

func (i *inner) Duration(key string, fallback ...time.Duration) time.Duration {
	if val, ok := i.Lookup(key); ok {
		n, err := strconv.Atoi(val)
		if err == nil {
			return time.Duration(n)
		}
		d, err := time.ParseDuration(val)
		if err == nil {
			return d
		}
	}
	for _, value := range fallback {
		return value
	}
	return 0
}

func (i *inner) Bool(key string, fallback ...bool) bool {
	if val, ok := i.Lookup(key); ok {
		bl, err := strconv.ParseBool(val)
		if err == nil {
			return bl
		}
	}
	for _, value := range fallback {
		return value
	}
	return false
}

// List splits the value by comma and returns a string slice.
func (i *inner) List(key string, fallback ...[]string) []string {
	if value, ok := i.Lookup(key); ok {
		parts := strings.Split(value, ",")
		for i, part := range parts {
			parts[i] = strings.TrimSpace(part)
		}
		return parts
	}
	for _, value := range fallback {
		return value
	}
	return []string{}
}

// Map retrieves all values with the specified prefix.
func (i *inner) Map(prefix string) map[string]string {
	result := map[string]string{}
	next := i.iter()
	for {
		key, value, ok := next()
		if !ok {
			return result
		}
		if strings.HasPrefix(key, prefix) {
			name := strings.TrimPrefix(key, prefix)
			result[name] = strings.TrimSpace(value)
		}
	}
}

// Where retrieves all values that match the filter.
func (i *inner) Where(filter func(name, value string) bool) map[string]string {
	result := map[string]string{}
	next := i.iter()
	for {
		key, value, ok := next()
		if !ok {
			return result
		}
		if filter(key, value) {
			result[key] = value
		}
	}
}

// Fill populates the specified struct with environment variables.
func (i *inner) Fill(structure any) error {
	inputType := reflect.TypeOf(structure)

	if inputType != nil && inputType.Kind() == reflect.Ptr && inputType.Elem().Kind() == reflect.Struct {
		return i.fillStruct(reflect.ValueOf(structure).Elem())
	}

	return errors.New("env: invalid structure")
}

func (i *inner) fillStruct(s reflect.Value) error {
	for j := 0; j < s.NumField(); j++ {
		if t, exist := s.Type().Field(j).Tag.Lookup("env"); exist {
			if osv := i.String(t); osv != "" {
				v, err := cast.FromType(osv, s.Type().Field(j).Type)
				if err != nil {
					return fmt.Errorf("env: cannot set `%v` field; err: %v", s.Type().Field(j).Name, err)
				}
				ptr := reflect.NewAt(s.Field(j).Type(), unsafe.Pointer(s.Field(j).UnsafeAddr())).Elem()
				ptr.Set(reflect.ValueOf(v))
			}
		} else if s.Type().Field(j).Type.Kind() == reflect.Struct {
			if err := i.fillStruct(s.Field(j)); err != nil {
				return err
			}
		} else if s.Type().Field(j).Type.Kind() == reflect.Ptr {
			if s.Field(j).IsZero() == false && s.Field(j).Elem().Type().Kind() == reflect.Struct {
				if err := i.fillStruct(s.Field(j).Elem()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
