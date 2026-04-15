package skeleton

import "testing"

func TestExtract_Unsupported(t *testing.T) {
	src := []byte("hello world\n")
	got := Extract("rust", src, true, true)
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
