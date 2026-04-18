package skeleton

import (
	"testing"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func TestExtract_Unsupported(t *testing.T) {
	src := []byte("hello world\n")
	got := Extract("erlang", src, true, true)
	if string(got) != string(src) {
		t.Errorf("unsupported lang should return src unchanged")
	}
}

func TestExtract_NeitherFlag(t *testing.T) {
	src := []byte("int foo() { return 1; }\n")
	got := Extract("c", src, false, false)
	if string(got) != string(src) {
		t.Errorf("no flags should return src unchanged")
	}
}

func TestParserPoolFor_CachesLanguageAndPool(t *testing.T) {
	langName := "test-cache-" + t.Name()
	calls := 0

	newLang := func() *gotreesitter.Language {
		calls++
		return grammars.GoLanguage()
	}

	pool1, lang1 := parserPoolFor(langName, newLang)
	pool2, lang2 := parserPoolFor(langName, newLang)

	if pool1 != pool2 {
		t.Fatalf("expected parser pool to be cached per language")
	}
	if lang1 != lang2 {
		t.Fatalf("expected language to be cached per language")
	}
	if calls != 1 {
		t.Fatalf("expected newLang to be called once, got %d", calls)
	}
}
