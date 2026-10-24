// Package install installs Tomo into a project or into harness-global command
// dirs. It is a faithful port of scripts/install.js, adapted so the compiled
// `tomo` binary replaces the .tomo/lib/*.js runtime helpers.
package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/btdstudio/tomo/internal/providers"
	"github.com/btdstudio/tomo/internal/render"
)

// Notice is a grant-seeding result; when Manual is set the user must paste the
// snippet themselves.
type Notice struct {
	Pattern string
	Path    string
	Reason  string
	Snippet string
	Manual  bool
	Changed bool
}

// Options drives Install.
type Options struct {
	Project    string
	Global     bool
	GlobalDirs map[string]string // test-injected dirs: <id>, <id>Agents, tomo, opencodeConfig, antigravitySettings
	Providers  []string
	Notices    *[]Notice
	SelfPath   string // the tomo binary to copy into .tomo/bin (defaults to os.Executable)
}

func selfPath(opts Options) string {
	if opts.SelfPath != "" {
		return opts.SelfPath
	}
	p, _ := os.Executable()
	return p
}

// outFile is one file to write during install: a destination path and its
// rendered/verbatim content.
type outFile struct {
	path    string
	content []byte
}

// copyPairs returns the files to write for a per-project install, sourced from
// the embedded content store and the in-memory rendered output.
func copyPairs(project string, selected []string, all map[string]string) ([]outFile, error) {
	ids := selected
	if len(ids) == 0 {
		ids = providers.ProviderIds()
	}
	var files []outFile

	commands, err := render.ReadCommands()
	if err != nil {
		return nil, err
	}
	for _, cmd := range commands {
		for _, id := range ids {
			prov := providers.PROVIDERS[id]
			files = append(files, outFile{
				path:    filepath.Join(prov.ProjectCommandDir(project), prov.CommandFilename(cmd.Name)),
				content: []byte(all[render.ProviderCommandName(cmd.Name, id)]),
			})
		}
	}

	templates, err := render.TemplatesDir()
	if err != nil {
		return nil, err
	}
	for _, t := range templates {
		if t.IsDir() {
			continue
		}
		b, err := render.ReadTemplate(t.Name())
		if err != nil {
			return nil, err
		}
		files = append(files, outFile{
			path:    filepath.Join(project, ".tomo", "templates", t.Name()),
			content: b,
		})
	}

	personas, err := render.ReadPersonas()
	if err != nil {
		return nil, err
	}
	for _, p := range personas {
		for _, id := range ids {
			prov := providers.PROVIDERS[id]
			files = append(files, outFile{
				path:    filepath.Join(prov.ProjectAgentDir(project), prov.Filename(p.Name)),
				content: []byte(all[render.PersonaCastName(p.Name, prov.ID)]),
			})
		}
		// Neutral copy for lens adoption (.tomo/personas/<name>.md).
		b, err := render.ReadPersonaSource(p.Name)
		if err != nil {
			return nil, err
		}
		files = append(files, outFile{
			path:    filepath.Join(project, ".tomo", "personas", p.Name+".md"),
			content: b,
		})
	}
	return files, nil
}

// GlobalDirMap resolves each provider's target dirs for a global install.
func GlobalDirMap(globalDirs map[string]string, agents bool) map[string]string {
	out := map[string]string{}
	for _, id := range providers.ProviderIds() {
		prov := providers.PROVIDERS[id]
		key := id
		fallback := prov.GlobalCommandDir()
		if agents {
			key = id + "Agents"
			fallback = prov.GlobalAgentDir()
		}
		if globalDirs != nil && globalDirs[key] != "" {
			out[id] = globalDirs[key]
		} else {
			out[id] = fallback
		}
	}
	return out
}

// SeedAntigravityGrants seeds a read_file grant for the tomo config dir into
// Antigravity's settings.json.
type grantResult struct {
	Grant   string
	Path    string
	Changed bool
}

func SeedAntigravityGrants(settingsPath, tomoDir string) (grantResult, error) {
	grant := fmt.Sprintf("read_file(%s)", tomoDir)
	settings := map[string]any{}
	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return grantResult{}, fmt.Errorf("tomo: cannot parse %s: %v", settingsPath, err)
		}
		if settings == nil {
			return grantResult{}, fmt.Errorf("tomo: antigravity settings must be a JSON object: %s", settingsPath)
		}
	}
	perms, _ := settings["permissions"].(map[string]any)
	if perms == nil {
		perms = map[string]any{}
		settings["permissions"] = perms
	}
	allow, _ := perms["allow"].([]any)
	for _, a := range allow {
		if s, ok := a.(string); ok && s == grant {
			return grantResult{Grant: grant, Path: settingsPath, Changed: false}, nil
		}
	}
	perms["allow"] = append(allow, grant)
	buf, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return grantResult{}, err
	}
	buf = append(buf, '\n')
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return grantResult{}, err
	}
	if err := os.WriteFile(settingsPath, buf, 0o644); err != nil {
		return grantResult{}, err
	}
	return grantResult{Grant: grant, Path: settingsPath, Changed: true}, nil
}

// ParseJsonc parses opencode's JSON/JSONC config without dependencies: strips
// // and /* */ comments and trailing commas outside strings. Reports whether
// comments were present.
func ParseJsonc(src string) (map[string]any, bool, error) {
	var out strings.Builder
	hadComments := false
	i := 0
	for i < len(src) {
		c := src[i]
		if c == '"' {
			j := i + 1
			for j < len(src) && src[j] != '"' {
				if src[j] == '\\' {
					j += 2
				} else {
					j++
				}
			}
			end := j + 1
			if end > len(src) {
				end = len(src)
			}
			out.WriteString(src[i:end])
			i = end
			continue
		}
		if c == '/' && i+1 < len(src) && src[i+1] == '/' {
			hadComments = true
			for i < len(src) && src[i] != '\n' {
				i++
			}
			out.WriteByte('\n')
			continue
		}
		if c == '/' && i+1 < len(src) && src[i+1] == '*' {
			hadComments = true
			end := strings.Index(src[i+2:], "*/")
			if end == -1 {
				i = len(src)
			} else {
				i = i + 2 + end + 2
			}
			continue
		}
		out.WriteByte(c)
		i++
	}

	var clean strings.Builder
	outStr := out.String()
	i = 0
	for i < len(outStr) {
		c := outStr[i]
		if c == '"' {
			j := i + 1
			for j < len(outStr) && outStr[j] != '"' {
				if outStr[j] == '\\' {
					j += 2
				} else {
					j++
				}
			}
			end := j + 1
			if end > len(outStr) {
				end = len(outStr)
			}
			clean.WriteString(outStr[i:end])
			i = end
			continue
		}
		if c == ',' && trailingCommaRe.MatchString(outStr[i+1:]) {
			// drop the trailing comma
			i++
			continue
		}
		clean.WriteByte(c)
		i++
	}

	var v any
	if err := json.Unmarshal([]byte(clean.String()), &v); err != nil {
		return nil, false, err
	}
	m, _ := v.(map[string]any)
	return m, hadComments, nil
}

var trailingCommaRe = regexp.MustCompile(`^\s*[}\]]`)

func grantSnippet(pattern string) string {
	type ext struct {
		ExternalDirectory map[string]string `json:"external_directory"`
	}
	type perm struct {
		Permission ext `json:"permission"`
	}
	b, _ := json.MarshalIndent(perm{Permission: ext{ExternalDirectory: map[string]string{pattern: "allow"}}}, "", "  ")
	return string(b)
}

// SeedOpenCodeGrants seeds an external_directory allow rule for the tomo config
// dir into the global opencode config.
func SeedOpenCodeGrants(configDir, tomoDir string) (Notice, error) {
	pattern := providers.HomeRelative(tomoDir) + "/**"
	snippet := grantSnippet(pattern)
	configPath := ""
	for _, f := range []string{"opencode.json", "opencode.jsonc"} {
		if _, err := os.Stat(filepath.Join(configDir, f)); err == nil {
			configPath = filepath.Join(configDir, f)
			break
		}
	}
	if configPath == "" {
		configPath = filepath.Join(configDir, "opencode.json")
	}

	config := map[string]any{"$schema": "https://opencode.ai/config.json"}
	if _, err := os.Stat(configPath); err == nil {
		parsed, hadComments, err := ParseJsonc(readFile(configPath))
		if err != nil {
			return Notice{Pattern: pattern, Path: configPath, Manual: true, Snippet: snippet, Reason: fmt.Sprintf("cannot parse: %v", err)}, nil
		}
		if parsed == nil {
			return Notice{Pattern: pattern, Path: configPath, Manual: true, Snippet: snippet, Reason: "config is not a JSON object"}, nil
		}
		config = parsed
		if granted(config["permission"], pattern) {
			return Notice{Pattern: pattern, Path: configPath}, nil
		}
		if hadComments {
			return Notice{Pattern: pattern, Path: configPath, Manual: true, Snippet: snippet, Reason: "file has comments"}, nil
		}
	}

	perm := normalizePerm(config["permission"])
	if perm == nil {
		return Notice{Pattern: pattern, Path: configPath, Manual: true, Snippet: snippet, Reason: "unexpected permission shape"}, nil
	}
	ext := normalizeExt(perm["external_directory"])
	delete(ext, pattern)
	ext[pattern] = "allow"
	perm["external_directory"] = ext
	config["permission"] = perm

	buf, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return Notice{}, err
	}
	buf = append(buf, '\n')
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return Notice{}, err
	}
	if err := os.WriteFile(configPath, buf, 0o644); err != nil {
		return Notice{}, err
	}
	return Notice{Pattern: pattern, Path: configPath, Changed: true}, nil
}

func readFile(p string) string {
	b, _ := os.ReadFile(p)
	return string(b)
}

func normalizePerm(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	if s, ok := v.(string); ok {
		return map[string]any{"*": s}
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	return m
}

func normalizeExt(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	if s, ok := v.(string); ok {
		return map[string]any{"*": s}
	}
	m, ok := v.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return m
}

func granted(perm any, pattern string) bool {
	if s, ok := perm.(string); ok {
		return s == "allow"
	}
	if perm == nil {
		return false
	}
	m, ok := perm.(map[string]any)
	if !ok {
		return false
	}
	ext := m["external_directory"]
	if s, ok := ext.(string); ok {
		return s == "allow"
	}
	if ext == nil && m["*"] == "allow" {
		return true
	}
	extM, ok := ext.(map[string]any)
	if !ok {
		return false
	}
	// The seed always places pattern last (opencode's last matching rule
	// wins), so presence of an allow rule for pattern means it is already
	// granted.
	return extM[pattern] == "allow"
}

// Install folds the build into install: it renders the wrappers and casts
// in-memory (from the embedded store) and places them — copy for a project,
// copy for a global install. Nothing depends on a source checkout.
func Install(opts Options) ([]string, error) {
	all, err := render.GenerateAll()
	if err != nil {
		return nil, err
	}
	ids := opts.Providers
	if len(ids) == 0 {
		ids = providers.ProviderIds()
	}
	var written []string

	if opts.Global {
		cmdDirs := GlobalDirMap(opts.GlobalDirs, false)
		agentDirs := GlobalDirMap(opts.GlobalDirs, true)
		tomoDir := providers.TomoConfigDir()
		if opts.GlobalDirs != nil && opts.GlobalDirs["tomo"] != "" {
			tomoDir = opts.GlobalDirs["tomo"]
		}
		// Retire a stale content-store link from a pre-self-contained install.
		staleContent := filepath.Join(tomoDir, "content")
		if info, err := os.Lstat(staleContent); err == nil && info.Mode()&os.ModeSymlink != 0 {
			os.Remove(staleContent)
		}
		for _, id := range ids {
			if providers.PROVIDERS[id].GlobalConfigDir != nil {
				configDir := providers.PROVIDERS[id].GlobalConfigDir()
				if opts.GlobalDirs != nil && opts.GlobalDirs[id+"Config"] != "" {
					configDir = opts.GlobalDirs[id+"Config"]
				}
				if id == "opencode" {
					seeded, err := SeedOpenCodeGrants(configDir, tomoDir)
					if err != nil {
						return nil, err
					}
					if seeded.Changed {
						written = append(written, seeded.Path)
					}
					if seeded.Manual && opts.Notices != nil {
						*opts.Notices = append(*opts.Notices, seeded)
					}
				}
			}
			if providers.PROVIDERS[id].SettingsPath != nil && id == "antigravity" {
				settingsPath := providers.PROVIDERS[id].SettingsPath()
				if opts.GlobalDirs != nil && opts.GlobalDirs[id+"Settings"] != "" {
					settingsPath = opts.GlobalDirs[id+"Settings"]
				}
				seeded, err := SeedAntigravityGrants(settingsPath, tomoDir)
				if err != nil {
					return nil, err
				}
				if seeded.Changed {
					written = append(written, seeded.Path)
				}
			}
		}
		for _, id := range ids {
			os.MkdirAll(cmdDirs[id], 0o755)
			os.MkdirAll(agentDirs[id], 0o755)
		}
		commands, err := render.ReadCommands()
		if err != nil {
			return nil, err
		}
		for _, cmd := range commands {
			for _, id := range ids {
				prov := providers.PROVIDERS[id]
				dst := filepath.Join(cmdDirs[id], prov.CommandFilename(cmd.Name))
				os.Remove(dst)
				if err := writeBytes(dst, []byte(all[render.ProviderCommandName(cmd.Name, id)]), 0o644); err != nil {
					return nil, err
				}
				written = append(written, dst)
			}
		}
		personas, err := render.ReadPersonas()
		if err != nil {
			return nil, err
		}
		for _, p := range personas {
			for _, id := range ids {
				prov := providers.PROVIDERS[id]
				dst := filepath.Join(agentDirs[id], prov.Filename(p.Name))
				os.Remove(dst)
				if err := writeBytes(dst, []byte(all[render.PersonaCastName(p.Name, prov.ID)]), 0o644); err != nil {
					return nil, err
				}
				written = append(written, dst)
			}
		}
	} else {
		files, err := copyPairs(opts.Project, ids, all)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if err := writeBytes(f.path, f.content, 0o644); err != nil {
				return nil, err
			}
			written = append(written, f.path)
		}
		// Place the compiled tomo binary in .tomo/bin/tomo.
		binDest := filepath.Join(opts.Project, ".tomo", "bin", "tomo")
		if err := copyFile(selfPath(opts), binDest, 0o755); err != nil {
			return nil, err
		}
		written = append(written, binDest)

		changed, p, err := providers.SeedSettingsProvider(opts.Project, ids)
		if err != nil {
			return nil, err
		}
		if changed {
			written = append(written, p)
		}
	}
	return written, nil
}

func writeBytes(dst string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, content, mode)
}

func copyFile(src, dst string, mode os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return writeBytes(dst, data, mode)
}
