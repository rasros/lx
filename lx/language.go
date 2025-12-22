package lx

import (
	"bytes"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

var extToLang = map[string]string{
	// Systems / Compiled
	".go":    "go",
	".rs":    "rust",
	".c":     "c",
	".h":     "c",
	".cc":    "cpp",
	".cpp":   "cpp",
	".cxx":   "cpp",
	".hpp":   "cpp",
	".hh":    "cpp",
	".hxx":   "cpp",
	".cs":    "csharp",
	".m":     "objectivec",
	".mm":    "objectivec",
	".swift": "swift",
	".val":   "vala",
	".v":     "v",
	".zig":   "zig",
	".nim":   "nim",

	// JVM
	".java":   "java",
	".kt":     "kotlin",
	".kts":    "kotlin",
	".scala":  "scala",
	".groovy": "groovy",
	".gradle": "groovy",
	".clj":    "clojure",
	".cljs":   "clojure",

	// Web / Front-end
	".js":     "javascript",
	".mjs":    "javascript",
	".cjs":    "javascript",
	".jsx":    "jsx",
	".ts":     "typescript",
	".tsx":    "tsx",
	".html":   "html",
	".htm":    "html",
	".css":    "css",
	".scss":   "scss",
	".sass":   "sass",
	".less":   "less",
	".vue":    "vue",
	".svelte": "svelte",
	".wasm":   "wasm",

	// Scripting / Dynamic
	".py":   "python",
	".pyw":  "python",
	".rb":   "ruby",
	".php":  "php",
	".lua":  "lua",
	".pl":   "perl",
	".pm":   "perl",
	".r":    "r",
	".dart": "dart",
	".el":   "lisp",
	".lisp": "lisp",
	".erl":  "erlang",
	".ex":   "elixir",
	".exs":  "elixir",

	// Shell / Batch
	".sh":   "bash",
	".bash": "bash",
	".zsh":  "zsh",
	".ps1":  "powershell",
	".bat":  "batch",
	".cmd":  "batch",

	// Data / Config / Markup
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

	// Utilities
	".diff":  "diff",
	".patch": "diff",
	".txt":   "text",
	".log":   "text",
}

var filenameToLang = map[string]string{
	// Build / Config
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

	// Dotfiles
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

// IsBinary checks if the data looks like binary content.
// It checks for null bytes in the first 1024 bytes or invalid UTF-8.
func IsBinary(data []byte) bool {
	limit := 1024
	if len(data) < limit {
		limit = len(data)
	}
	if len(data) == 0 {
		return false
	}
	if bytes.IndexByte(data[:limit], 0) != -1 {
		return true
	}
	return !utf8.Valid(data)
}

// DetectLanguage returns a language identifier based on file path and content.
// It prioritizes exact filenames, then pattern matching (Dockerfile.*),
// then file extensions, and finally falls back to Shebang detection.
func DetectLanguage(path string, data []byte) string {
	base := strings.ToLower(filepath.Base(path))
	if lang, ok := filenameToLang[base]; ok {
		return lang
	}
	if strings.HasPrefix(base, "dockerfile.") {
		return "dockerfile"
	}
	if lang, ok := extToLang[strings.ToLower(filepath.Ext(path))]; ok {
		return lang
	}
	return parseShebang(data)
}

// parseShebang reads the first line of data to detect standard interpreters.
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
		interpreter := filepath.Base(parts[0])
		if interpreter == "env" && len(parts) > 1 {
			interpreter = filepath.Base(parts[1])
		}
		return shebangToLang[interpreter]
	}
	return ""
}
