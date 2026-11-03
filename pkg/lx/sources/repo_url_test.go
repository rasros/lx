package sources

import "testing"

func TestRewriteRepoURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"github short", "github.com/a/b", "https://github.com/a/b/archive/HEAD.zip", true},
		{"github tree ref", "https://github.com/a/b/tree/dev", "https://github.com/a/b/archive/refs/heads/dev.zip", true},

		{"gitlab short", "gitlab.com/a/b", "https://gitlab.com/a/b/-/archive/HEAD/b-HEAD.zip", true},

		{"github tree slashed ref", "https://github.com/a/b/tree/feat/x", "https://github.com/a/b/archive/refs/heads/feat/x.zip", true},

		{"gitlab subgroup", "gitlab.com/group/sub/proj", "https://gitlab.com/group/sub/proj/-/archive/HEAD/proj-HEAD.zip", true},

		{"bitbucket short", "bitbucket.org/a/b", "https://bitbucket.org/a/b/get/HEAD.zip", true},
		{"bitbucket src ref", "https://bitbucket.org/a/b/src/dev", "https://bitbucket.org/a/b/get/dev.zip", true},

		{"codeberg short", "codeberg.org/a/b", "https://codeberg.org/a/b/archive/HEAD.zip", true},
		{"codeberg src/branch ref", "https://codeberg.org/a/b/src/branch/dev", "https://codeberg.org/a/b/archive/dev.zip", true},

		// Negative cases
		{"unknown host", "https://example.com/a/b", "", false},
		{"missing repo", "github.com/a", "", false},
		{"deep github path leave alone", "https://github.com/a/b/blob/main/x.go", "", false},
		{"already archive zip", "https://github.com/a/b/archive/HEAD.zip", "", false},
		{"non-http scheme", "ftp://github.com/a/b", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := RewriteRepoURL(tt.in)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v (got %q)", ok, tt.ok, got)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
