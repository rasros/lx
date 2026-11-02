package langs

// LooksLikeCpp reports C or C++ source with a token only C++ has, outside
// comments, string and character literals: a `::`, a `class` or `namespace`
// keyword before a name (not `struct class`, a C struct named class),
// `template` before `<`, or an access label (`public:`). It is a cheap scan
// that keeps C source from being parsed as C++.
func LooksLikeCpp(src []byte) bool {
	prev := "" // the identifier before the current token, if it was one
	for i := 0; i < len(src); {
		ch := src[i]
		switch {
		case ch == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		case ch == '/' && i+1 < len(src) && src[i+1] == '*':
			i += 2
			for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}
			i += 2
			continue
		case ch == '"' || ch == '\'':
			i++
			for i < len(src) && src[i] != ch && src[i] != '\n' {
				if src[i] == '\\' {
					i++
				}
				i++
			}
			i++
			prev = ""
			continue
		case ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n':
			i++
			continue
		case ch == ':' && i+1 < len(src) && src[i+1] == ':':
			return true
		case isIdentByte(ch) && !(ch >= '0' && ch <= '9'):
			start := i
			for i < len(src) && isIdentByte(src[i]) {
				i++
			}
			word := string(src[start:i])
			next := nextNonSpace(src, i)
			switch word {
			case "class", "namespace":
				if prev != "struct" && prev != "union" && prev != "enum" && next < len(src) && isIdentByte(src[next]) {
					return true
				}
			case "template":
				if next < len(src) && src[next] == '<' {
					return true
				}
			case "public", "private", "protected":
				if next < len(src) && src[next] == ':' && (next+1 >= len(src) || src[next+1] != ':') {
					return true
				}
			}
			prev = word
			continue
		}
		prev = ""
		i++
	}
	return false
}

func isIdentByte(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

func nextNonSpace(src []byte, i int) int {
	for i < len(src) && (src[i] == ' ' || src[i] == '\t') {
		i++
	}
	return i
}
