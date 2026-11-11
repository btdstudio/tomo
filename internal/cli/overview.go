package cli

import (
	"net/url"
	"path"
	"strings"

	"github.com/btdstudio/kiso/ticket"
)

// webRepo is a planning repo's web address: base is its home page, and azure says
// whether files are addressed Azure DevOps style (?path=) instead of GitHub style
// (/blob/HEAD/).
type webRepo struct {
	base  string
	azure bool
}

// webURL maps a recorded remote to its repo's web address, for the hosts tomo
// knows: GitHub, and Azure DevOps over SSH or HTTPS. Anything else, including a
// folder name, has none.
func webURL(remote string) (webRepo, bool) {
	host, p, ok := splitRemote(remote)
	if !ok {
		return webRepo{}, false
	}
	p = strings.TrimSuffix(strings.TrimSuffix(p, "/"), ".git")
	segs := strings.Split(p, "/")
	for _, s := range segs {
		if s == "" || s == "." || s == ".." {
			return webRepo{}, false
		}
	}
	switch strings.ToLower(host) {
	case "github.com":
		if len(segs) == 2 {
			return webRepo{base: "https://github.com/" + escapePath(segs)}, true
		}
	case "ssh.dev.azure.com": // v3/<org>/<project>/<repo>
		if len(segs) == 4 && segs[0] == "v3" {
			return azureRepo(segs[1], segs[2], segs[3]), true
		}
	case "dev.azure.com": // <org>/<project>/_git/<repo>
		if len(segs) == 4 && segs[2] == "_git" {
			return azureRepo(segs[0], segs[1], segs[3]), true
		}
	}
	return webRepo{}, false
}

func azureRepo(org, project, repo string) webRepo {
	return webRepo{base: "https://dev.azure.com/" + escapePath([]string{org, project, "_git", repo}), azure: true}
}

// splitRemote returns a remote's host and repo path, from a URL (https, http or
// ssh) or an scp-like [user@]host:path.
func splitRemote(remote string) (host, p string, ok bool) {
	if strings.Contains(remote, "://") {
		u, err := url.Parse(remote)
		if err != nil {
			return "", "", false
		}
		switch u.Scheme {
		case "https", "http", "ssh":
		default:
			return "", "", false
		}
		return u.Hostname(), strings.TrimPrefix(u.Path, "/"), u.Hostname() != ""
	}
	i := strings.Index(remote, ":")
	if i <= 0 || strings.Contains(remote[:i], "/") {
		return "", "", false // no host, or a local path such as ./a:b
	}
	host = remote[:i]
	if j := strings.LastIndex(host, "@"); j >= 0 {
		host = host[j+1:]
	}
	return host, remote[i+1:], host != ""
}

// overviewPath is the change overview for a ticket at p: the README.md next to
// the tickets folder that holds it. A path outside a tickets folder, or one that
// could leave the repo, has none.
func overviewPath(p string) (string, bool) {
	if p == "" || strings.HasPrefix(p, "/") {
		return "", false
	}
	for _, s := range strings.Split(p, "/") {
		if s == ".." {
			return "", false
		}
	}
	dir := path.Dir(path.Clean(p))
	if path.Base(dir) != "tickets" {
		return "", false
	}
	return path.Join(path.Dir(dir), "README.md"), true
}

// overviewLink is the web link to the change overview a ticket was planned in, or
// "" when its origin's host or path doesn't allow one.
func overviewLink(o ticket.Origin) string {
	repo, ok := webURL(o.Workspace)
	if !ok {
		return ""
	}
	p, ok := overviewPath(o.Path)
	if !ok {
		return ""
	}
	segs := strings.Split(p, "/")
	if repo.azure {
		// The path sits in the query, where & and + would break it.
		for i, s := range segs {
			segs[i] = strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
		}
		return repo.base + "?path=/" + strings.Join(segs, "/")
	}
	return repo.base + "/blob/HEAD/" + escapePath(segs)
}

// escapePath escapes each segment for a URL path and joins them with "/".
func escapePath(segs []string) string {
	out := make([]string, len(segs))
	for i, s := range segs {
		out[i] = url.PathEscape(s)
	}
	return strings.Join(out, "/")
}
