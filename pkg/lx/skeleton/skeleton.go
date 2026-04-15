package skeleton

// Supported reports whether skeleton extraction is available for lang.
func Supported(lang string) bool {
	_, ok := langDefs[lang]
	return ok
}

// Extract returns filtered content from src showing only the skeleton
// (function signatures and/or struct/class definitions) for lang.
func Extract(lang string, src []byte, functions, structs bool) []byte {
	if !functions && !structs {
		return src
	}
	return extract(lang, src, functions, structs)
}
