package sources

import (
	"strings"
	"sync"
)

// RewriteRepoURL inspects raw and, if it matches a known repo-host pattern
// (GitHub, GitLab, Bitbucket, Codeberg), returns the archive-zip URL for that
// repo and true. Otherwise returns "", false.
//
// Accepts input with or without an http(s) scheme. URLs that already point at
// a host-specific archive segment (e.g. /archive/, /-/archive/, /get/) are
// left alone.
func RewriteRepoURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}

	withScheme := raw
	if !strings.Contains(raw, "://") {
		withScheme = "https://" + raw
	}

	u, err := parseHTTPURL(withScheme)
	if err != nil {
		return "", false
	}

	host := strings.ToLower(u.Host)
	repoHostsMu.RLock()
	rewriter, ok := repoHosts[host]
	repoHostsMu.RUnlock()
	if !ok {
		return "", false
	}

	path := strings.Trim(u.Path, "/")
	if path == "" {
		return "", false
	}
	parts := strings.Split(path, "/")

	owner, repo, ref, ok := rewriter.parse(parts)
	if !ok || owner == "" || repo == "" {
		return "", false
	}
	return rewriter.archiveURL(owner, repo, ref), true
}

type repoRewriter struct {
	// parse extracts (owner, repo, ref) from path segments. Returns ok=false
	// when the path doesn't match the host's recognized shape (e.g. a
	// /blob/, /releases/, or /archive/ path) — the caller leaves such URLs
	// alone. The host is responsible for rejecting its own archive shape;
	// the matcher should only accept "/owner/repo[/{tree,src,...}/{ref}]".
	parse func(parts []string) (owner, repo, ref string, ok bool)
	// archiveURL composes the archive-zip URL. ref="" means "default branch".
	archiveURL func(owner, repo, ref string) string
}

func trimRepo(s string) string { return strings.TrimSuffix(s, ".git") }

var (
	repoHostsMu sync.RWMutex
	repoHosts   = map[string]repoRewriter{
		"github.com": {
			parse: func(parts []string) (owner, repo, ref string, ok bool) {
				if len(parts) < 2 {
					return "", "", "", false
				}
				owner, repo = parts[0], trimRepo(parts[1])
				rest := parts[2:]
				if len(rest) == 0 {
					return owner, repo, "", true
				}
				if rest[0] == "tree" && len(rest) >= 2 {
					return owner, repo, strings.Join(rest[1:], "/"), true
				}
				return "", "", "", false
			},
			archiveURL: func(owner, repo, ref string) string {
				if ref == "" {
					return "https://github.com/" + owner + "/" + repo + "/archive/HEAD.zip"
				}
				return "https://github.com/" + owner + "/" + repo + "/archive/refs/heads/" + ref + ".zip"
			},
		},
		"gitlab.com": {
			parse: func(parts []string) (owner, repo, ref string, ok bool) {
				// GitLab supports subgroups: group[/subgroup...]/repo[/-/tree/REF].
				sep := -1
				for i, p := range parts {
					if p == "-" {
						sep = i
						break
					}
				}
				var project, rest []string
				if sep >= 0 {
					project = parts[:sep]
					rest = parts[sep+1:]
				} else {
					project = parts
				}
				if len(project) < 2 {
					return "", "", "", false
				}
				owner = strings.Join(project[:len(project)-1], "/")
				repo = trimRepo(project[len(project)-1])
				if len(rest) == 0 {
					return owner, repo, "", true
				}
				if len(rest) >= 2 && rest[0] == "tree" {
					return owner, repo, strings.Join(rest[1:], "/"), true
				}
				return "", "", "", false
			},
			archiveURL: func(owner, repo, ref string) string {
				r := ref
				if r == "" {
					r = "HEAD"
				}
				return "https://gitlab.com/" + owner + "/" + repo + "/-/archive/" + r + "/" + repo + "-" + r + ".zip"
			},
		},
		"bitbucket.org": {
			parse: func(parts []string) (owner, repo, ref string, ok bool) {
				if len(parts) < 2 {
					return "", "", "", false
				}
				owner, repo = parts[0], trimRepo(parts[1])
				rest := parts[2:]
				if len(rest) == 0 {
					return owner, repo, "", true
				}
				if rest[0] == "src" && len(rest) >= 2 {
					return owner, repo, strings.Join(rest[1:], "/"), true
				}
				return "", "", "", false
			},
			archiveURL: func(owner, repo, ref string) string {
				r := ref
				if r == "" {
					r = "HEAD"
				}
				return "https://bitbucket.org/" + owner + "/" + repo + "/get/" + r + ".zip"
			},
		},
		"codeberg.org": {
			parse: func(parts []string) (owner, repo, ref string, ok bool) {
				if len(parts) < 2 {
					return "", "", "", false
				}
				owner, repo = parts[0], trimRepo(parts[1])
				rest := parts[2:]
				if len(rest) == 0 {
					return owner, repo, "", true
				}
				if len(rest) >= 3 && rest[0] == "src" && rest[1] == "branch" {
					return owner, repo, strings.Join(rest[2:], "/"), true
				}
				return "", "", "", false
			},
			archiveURL: func(owner, repo, ref string) string {
				r := ref
				if r == "" {
					r = "HEAD"
				}
				return "https://codeberg.org/" + owner + "/" + repo + "/archive/" + r + ".zip"
			},
		},
	}
)

// SetRepoHostForTest registers a host rewriter so tests can route short URLs
// at an httptest.Server. Use only from tests; the returned function restores
// the previous registration. The test parser accepts the simple
// "/owner/repo" shape (no ref segment).
func SetRepoHostForTest(host string, archiveURL func(owner, repo, ref string) string) func() {
	host = strings.ToLower(host)
	repoHostsMu.Lock()
	prev, existed := repoHosts[host]
	repoHosts[host] = repoRewriter{
		parse: func(parts []string) (string, string, string, bool) {
			if len(parts) < 2 {
				return "", "", "", false
			}
			return parts[0], trimRepo(parts[1]), "", true
		},
		archiveURL: archiveURL,
	}
	repoHostsMu.Unlock()
	return func() {
		repoHostsMu.Lock()
		defer repoHostsMu.Unlock()
		if existed {
			repoHosts[host] = prev
		} else {
			delete(repoHosts, host)
		}
	}
}
