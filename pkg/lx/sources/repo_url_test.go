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
		{"github https", "https://github.com/a/b", "https://github.com/a/b/archive/HEAD.zip", true},
		{"github http", "http://github.com/a/b", "https://github.com/a/b/archive/HEAD.zip", true},
		{"github trailing slash", "https://github.com/a/b/", "https://github.com/a/b/archive/HEAD.zip", true},
		{"github .git", "https://github.com/a/b.git", "https://github.com/a/b/archive/HEAD.zip", true},
		{"github tree ref", "https://github.com/a/b/tree/dev", "https://github.com/a/b/archive/refs/heads/dev.zip", true},
		{"github case-insensitive host", "GitHub.com/a/b", "https://github.com/a/b/archive/HEAD.zip", true},

		{"gitlab short", "gitlab.com/a/b", "https://gitlab.com/a/b/-/archive/HEAD/b-HEAD.zip", true},
		{"gitlab tree ref", "https://gitlab.com/a/b/-/tree/dev", "https://gitlab.com/a/b/-/archive/dev/b-dev.zip", true},

		{"github tree slashed ref", "https://github.com/a/b/tree/feat/x", "https://github.com/a/b/archive/refs/heads/feat/x.zip", true},
		{"github repo named get", "github.com/a/get", "https://github.com/a/get/archive/HEAD.zip", true},
		{"github repo named archive", "github.com/a/archive", "https://github.com/a/archive/archive/HEAD.zip", true},
		{"github repo named tree", "github.com/a/tree", "https://github.com/a/tree/archive/HEAD.zip", true},

		{"gitlab subgroup", "gitlab.com/group/sub/proj", "https://gitlab.com/group/sub/proj/-/archive/HEAD/proj-HEAD.zip", true},
		{"gitlab subgroup tree ref", "https://gitlab.com/group/sub/proj/-/tree/dev", "https://gitlab.com/group/sub/proj/-/archive/dev/proj-dev.zip", true},
		{"gitlab repo named archive", "gitlab.com/a/archive", "https://gitlab.com/a/archive/-/archive/HEAD/archive-HEAD.zip", true},

		{"bitbucket short", "bitbucket.org/a/b", "https://bitbucket.org/a/b/get/HEAD.zip", true},
		{"bitbucket src ref", "https://bitbucket.org/a/b/src/dev", "https://bitbucket.org/a/b/get/dev.zip", true},
		{"bitbucket src slashed ref", "https://bitbucket.org/a/b/src/feat/x", "https://bitbucket.org/a/b/get/feat/x.zip", true},
		{"bitbucket repo named archive", "bitbucket.org/a/archive", "https://bitbucket.org/a/archive/get/HEAD.zip", true},

		{"codeberg short", "codeberg.org/a/b", "https://codeberg.org/a/b/archive/HEAD.zip", true},
		{"codeberg src/branch ref", "https://codeberg.org/a/b/src/branch/dev", "https://codeberg.org/a/b/archive/dev.zip", true},
		{"codeberg src/branch slashed ref", "https://codeberg.org/a/b/src/branch/feat/x", "https://codeberg.org/a/b/archive/feat/x.zip", true},

		// Negative cases
		{"unknown host", "https://example.com/a/b", "", false},
		{"missing repo", "github.com/a", "", false},
		{"empty", "", "", false},
		{"deep github path leave alone", "https://github.com/a/b/blob/main/x.go", "", false},
		{"already archive zip", "https://github.com/a/b/archive/HEAD.zip", "", false},
		{"already archive tar.gz", "https://github.com/a/b/archive/refs/heads/main.tar.gz", "", false},
		{"github releases", "https://github.com/a/b/releases", "", false},
		{"gitlab archive passthrough", "https://gitlab.com/a/b/-/archive/main/b-main.zip", "", false},
		{"bitbucket get passthrough", "https://bitbucket.org/a/b/get/HEAD.zip", "", false},
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
