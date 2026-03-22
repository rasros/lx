package detect

import (
	"bytes"
	"path"
	"strings"
	"unicode/utf8"
)

var extToLang = map[string]string{
	".go":         "go",
	".rs":         "rust",
	".c":          "c",
	".h":          "c",
	".cc":         "cpp",
	".cpp":        "cpp",
	".cxx":        "cpp",
	".hpp":        "cpp",
	".hh":         "cpp",
	".hxx":        "cpp",
	".cs":         "csharp",
	".m":          "objectivec",
	".mm":         "objectivec",
	".swift":      "swift",
	".val":        "vala",
	".v":          "v",
	".zig":        "zig",
	".nim":        "nim",
	".java":       "java",
	".kt":         "kotlin",
	".kts":        "kotlin",
	".scala":      "scala",
	".groovy":     "groovy",
	".gradle":     "groovy",
	".clj":        "clojure",
	".cljs":       "clojure",
	".js":         "javascript",
	".mjs":        "javascript",
	".cjs":        "javascript",
	".jsx":        "jsx",
	".ts":         "typescript",
	".tsx":        "tsx",
	".html":       "html",
	".htm":        "html",
	".css":        "css",
	".scss":       "scss",
	".sass":       "sass",
	".less":       "less",
	".vue":        "vue",
	".svelte":     "svelte",
	".wasm":       "wasm",
	".py":         "python",
	".pyw":        "python",
	".rb":         "ruby",
	".php":        "php",
	".lua":        "lua",
	".pl":         "perl",
	".pm":         "perl",
	".r":          "r",
	".dart":       "dart",
	".el":         "lisp",
	".lisp":       "lisp",
	".erl":        "erlang",
	".ex":         "elixir",
	".exs":        "elixir",
	".sh":         "bash",
	".bash":       "bash",
	".zsh":        "zsh",
	".ps1":        "powershell",
	".bat":        "batch",
	".cmd":        "batch",
	".json":       "json",
	".yml":        "yaml",
	".yaml":       "yaml",
	".toml":       "toml",
	".xml":        "xml",
	".svg":        "xml",
	".ini":        "ini",
	".conf":       "ini",
	".md":         "markdown",
	".markdown":   "markdown",
	".csv":        "csv",
	".tsv":        "tsv",
	".sql":        "sql",
	".graphql":    "graphql",
	".gql":        "graphql",
	".proto":      "protobuf",
	".tf":         "hcl",
	".tfvars":     "hcl",
	".dockerfile": "dockerfile",
	".diff":       "diff",
	".patch":      "diff",
	".txt":        "text",
	".log":        "text",
}

var filenameToLang = map[string]string{
	"dockerfile":    "dockerfile",
	"makefile":      "makefile",
	"jenkinsfile":   "groovy",
	"rakefile":      "ruby",
	"gemfile":       "ruby",
	"vagrantfile":   "ruby",
	"procfile":      "yaml",
	"go.mod":        "gomod",
	"go.sum":        "gosum",
	"package.json":  "json",
	"tsconfig.json": "json",
	"composer.json": "json",
	"cargo.toml":    "toml",
	".gitignore":    "gitignore",
	".gitmodules":   "gitconfig",
	".bashrc":       "bash",
	".bash_profile": "bash",
	".zshrc":        "zsh",
	".vimrc":        "vim",
	".npmrc":        "ini",
	".editorconfig": "ini",
	".env":          "bash",
}

var shebangToLang = map[string]string{
	"bash":    "bash",
	"sh":      "bash",
	"zsh":     "zsh",
	"python":  "python",
	"python2": "python",
	"python3": "python",
	"node":    "javascript",
	"nodejs":  "javascript",
	"ruby":    "ruby",
	"perl":    "perl",
	"php":     "php",
	"lua":     "lua",
	"make":    "makefile",
}

// IsBinary detects if the given data sample contains null bytes or is invalid UTF-8.
func IsBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	if bytes.IndexByte(data, 0) != -1 {
		return true
	}

	chunk := data

	// Boundary cleanup: The caller passes a chunked read (e.g., 1024 bytes).
	// This read might have sliced a multi-byte character in half.
	// Backtrack up to 3 bytes to find the start of the last rune and drop it if incomplete.
	for i := 1; i <= 3 && len(chunk) >= i; i++ {
		if utf8.RuneStart(chunk[len(chunk)-i]) {
			if !utf8.FullRune(chunk[len(chunk)-i:]) {
				chunk = chunk[:len(chunk)-i]
			}
			break
		}
	}

	return !utf8.Valid(chunk)
}

// DetectLanguage identifies the programming language based on extension, filename, or shebang.
func DetectLanguage(p string, data []byte) string {
	base := strings.ToLower(path.Base(p))
	if lang, ok := filenameToLang[base]; ok {
		return lang
	}
	if strings.HasPrefix(base, "dockerfile.") {
		return "dockerfile"
	}
	if lang, ok := extToLang[strings.ToLower(path.Ext(p))]; ok {
		return lang
	}
	return parseShebang(data)
}

func parseShebang(data []byte) string {
	if len(data) < 2 || data[0] != '#' || data[1] != '!' {
		return ""
	}
	end := bytes.IndexByte(data, '\n')
	if end == -1 {
		end = len(data)
	}
	line := string(bytes.TrimSpace(data[2:end]))
	if parts := strings.Fields(line); len(parts) > 0 {
		interpreter := path.Base(parts[0])
		if interpreter == "env" && len(parts) > 1 {
			interpreter = path.Base(parts[1])
		}
		return shebangToLang[interpreter]
	}
	return ""
}
