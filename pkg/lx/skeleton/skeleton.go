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
// internal.MergeRanges does. A C header (header, a .h file) is read as C or
// C++, whichever fits it. Source that cannot be parsed, an unsupported
// lang, or asking for neither part keeps every row.
func Ranges(lang string, header bool, src []byte, functions, structs bool) []Range {
	ranges, _ := ranges(lang, header, src, internal.SplitLines(src), functions, structs)
	return ranges
}

// ranges is Ranges over src already split into lines. It also reports whether
// src was reduced at all.
func ranges(lang string, header bool, src []byte, lines [][]byte, functions, structs bool) ([]Range, bool) {
	if functions || structs {
		if kept, ok := extract(lang, header, src, lines, functions, structs); ok {
			return internal.MergeRanges(kept), true
		}
	}
	if n := internal.CountLines(src); n > 0 {
		return []Range{{Start: 0, End: n}}, false
	}
	return nil, false
}

// Keep returns the skeleton of src as kept source lines, ready to slice and
// number by their source rows.
func Keep(lang string, header bool, src []byte, functions, structs bool) Lines {
	lines := internal.SplitLines(src)
	r, _ := ranges(lang, header, src, lines, functions, structs)
	return internal.KeepRanges(lines, r)
}

// Extract returns the skeleton of src as text, with an elision marker line
// wherever source was left out between two kept lines. Source it does not
// reduce (see Ranges) comes back unchanged.
func Extract(lang string, header bool, src []byte, functions, structs bool) []byte {
	lines := internal.SplitLines(src)
	r, reduced := ranges(lang, header, src, lines, functions, structs)
	if !reduced {
		return src
	}
	return internal.KeepRanges(lines, r).Text()
}
