package skeleton

import "github.com/rasros/lx/pkg/lx/internal"

// Range is a run of source rows kept in a skeleton: 0-based, half-open, with
// an optional cut of the last row. See internal.LineRange.
type Range = internal.LineRange

// Lines are the kept source lines of a skeleton, each with its 1-based source
// row. See internal.KeptLines.
type Lines = internal.KeptLines

// Supported reports whether skeleton extraction is available for lang.
func Supported(lang string) bool {
	_, ok := langDefs[lang]
	return ok
}

// Ranges returns the rows of src that make up its skeleton (function
// signatures and/or struct/class definitions) for lang, sorted and merged as
// internal.MergeRanges does. Source that cannot be parsed, an unsupported
// lang, or asking for neither part keeps every row.
func Ranges(lang string, src []byte, functions, structs bool) []Range {
	if functions || structs {
		if ranges, ok := extract(lang, src, functions, structs); ok {
			return internal.MergeRanges(ranges)
		}
	}
	if n := internal.CountLines(src); n > 0 {
		return []Range{{Start: 0, End: n}}
	}
	return nil
}

// Keep returns the skeleton of src as kept source lines, ready to slice and
// number by their source rows.
func Keep(lang string, src []byte, functions, structs bool) Lines {
	return internal.KeepRanges(src, Ranges(lang, src, functions, structs))
}

// Extract returns the skeleton of src as text, with an elision marker line
// wherever source rows were left out between two kept ones.
func Extract(lang string, src []byte, functions, structs bool) []byte {
	return Keep(lang, src, functions, structs).Text()
}
