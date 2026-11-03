package skeleton

import "github.com/rasros/lx/pkg/lx/internal"

// Ranges returns the merged rows of src that make up its skeleton for lang.
// A .h header is read as C or C++, whichever fits. Unparseable source, an
// unsupported lang, or asking for neither part keeps every row.
func Ranges(lang string, header bool, src []byte, functions, structs bool) []Range {
	ranges, _ := ranges(lang, header, src, internal.SplitLines(src), functions, structs)
	return ranges
}

// Extract returns the skeleton of src as text, with an elision marker
// wherever source was left out.
func Extract(lang string, header bool, src []byte, functions, structs bool) []byte {
	lines := internal.SplitLines(src)
	r, reduced := ranges(lang, header, src, lines, functions, structs)
	if !reduced {
		return src
	}
	return internal.KeepRanges(lines, r).Text()
}
