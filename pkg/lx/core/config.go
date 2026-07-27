package core

import "reflect"

// NewConfig returns a default configuration.
func NewConfig() *Config {
	return &Config{
		OutputFormat: "markdown",
	}
}

// Merge applies non-zero fields from src to dst. Every field is a string, so
// "set" and "non-zero" coincide; a future non-string field whose zero value is
// meaningful would need explicit handling.
func Merge(dst *Config, src *Config) {
	d := reflect.ValueOf(dst).Elem()
	s := reflect.ValueOf(src).Elem()
	for i := 0; i < s.NumField(); i++ {
		if f := s.Field(i); !f.IsZero() {
			d.Field(i).Set(f)
		}
	}
}
