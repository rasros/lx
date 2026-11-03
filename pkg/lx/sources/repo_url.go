package sources

import (
	"slices"
	"strings"
	"sync"
)

// RewriteRepoURL returns the archive zip URL for a repository URL on GitHub,
// GitLab, Bitbucket or Codeberg, with or without a scheme. URLs that already
// point at an archive are left alone.
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
	// parse extracts owner, repo and ref from path segments, accepting only
	// "/owner/repo[/{tree,src,...}/{ref}]".
	parse func(parts []string) (owner, repo, ref string, ok bool)
	// archiveURL composes the archive-zip URL. ref="" means "default branch".
	archiveURL func(owner, repo, ref string) string
}

func trimRepo(s string) string { return strings.TrimSuffix(s, ".git") }

// parseOwnerRepo parses the "/owner/repo[/refPrefix.../{ref}]" paths of a
// host that names a ref below refPrefix.
func parseOwnerRepo(refPrefix ...string) func(parts []string) (owner, repo, ref string, ok bool) {
	return func(parts []string) (owner, repo, ref string, ok bool) {
		if len(parts) < 2 {
			return "", "", "", false
		}
		owner, repo = parts[0], trimRepo(parts[1])
		rest := parts[2:]
		if len(rest) == 0 {
			return owner, repo, "", true
		}
		if len(rest) > len(refPrefix) && slices.Equal(rest[:len(refPrefix)], refPrefix) {
			return owner, repo, strings.Join(rest[len(refPrefix):], "/"), true
		}
		return "", "", "", false
	}
}

// refOrHead is ref, or HEAD, the default branch, when ref is "".
func refOrHead(ref string) string {
	if ref == "" {
		return "HEAD"
	}
	return ref
}

var (
	repoHostsMu sync.RWMutex
	repoHosts   = map[string]repoRewriter{
		"github.com": {
			parse: parseOwnerRepo("tree"),
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
				r := refOrHead(ref)
				return "https://gitlab.com/" + owner + "/" + repo + "/-/archive/" + r + "/" + repo + "-" + r + ".zip"
			},
		},
		"bitbucket.org": {
			parse: parseOwnerRepo("src"),
			archiveURL: func(owner, repo, ref string) string {
				return "https://bitbucket.org/" + owner + "/" + repo + "/get/" + refOrHead(ref) + ".zip"
			},
		},
		"codeberg.org": {
			parse: parseOwnerRepo("src", "branch"),
			archiveURL: func(owner, repo, ref string) string {
				return "https://codeberg.org/" + owner + "/" + repo + "/archive/" + refOrHead(ref) + ".zip"
			},
		},
	}
)

// SetRepoHostForTest registers a host rewriter accepting "/owner/repo" so
// tests can route URLs at an httptest.Server. The returned function restores
// the previous registration.
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
