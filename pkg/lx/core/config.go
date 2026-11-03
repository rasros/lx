package core

import "reflect"

// NewConfig returns a default configuration.
func NewConfig() *Config {
	return &Config{
		OutputFormat: "markdown",
	}
}

// Merge applies the fields src sets to dst. A field is set when it is not
// zero: a template is a non-empty string, and a layout property a non-nil
// pointer, so false overrides too.
func Merge(dst *Config, src *Config) {
	d := reflect.ValueOf(dst).Elem()
	s := reflect.ValueOf(src).Elem()
	for i := 0; i < s.NumField(); i++ {
		if f := s.Field(i); !f.IsZero() {
			d.Field(i).Set(f)
		}
	}
}
