package sthpkgs

// resolve (plan) -> execute (do) -> record (manifest) -> idempotency checks (verify)

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/aottr/sth/internal/platform"
	"github.com/aottr/sth/internal/utils"
)

type ResolveResult struct {
	Recipe   Recipe
	Target   Target
	Paths    Paths
	Resolved ArtifactResolved
	Actions  []InstallAction
}

func ResolveRecipe(ctx context.Context, r Recipe) (ResolveResult, error) {
	pi := platform.GetPlatformInfo()
	target := Target{
		OS:     r.Target.OS,
		Arch:   r.Target.Arch,
		Distro: utils.FirstNonEmpty(r.Target.Distro, pi.Distro),
		Family: utils.FirstNonEmpty(r.Target.Family, pi.Family),
	}

	if err := ensureTargetSupported(target, pi); err != nil {
		return ResolveResult{}, err
	}

	paths := resolvePaths(r.Scope, r.Paths)
	if r.Artifact.IsEmpty() {
		return ResolveResult{
			Recipe:  r,
			Target:  target,
			Paths:   paths,
			Actions: r.Actions, // may be shell/system steps only
		}, nil
	}
	version, err := resolveVersion(ctx, r.Artifact.Version)
	if err != nil {
		return ResolveResult{}, err
	}

	name := r.Artifact.Name
	if name == "" {
		name = r.Name
	}

	tctx := map[string]string{
		"Name":    name,
		"Version": version,
		"OS":      pi.OS,
		"Arch":    pi.Arch,
		"Distro":  pi.Distro,
		"Family":  pi.Family,
	}
	url, err := renderTemplate(r.Artifact.URLTemplate, tctx)
	if err != nil {
		fmt.Println("Could not render template")
		return ResolveResult{}, err
	}

	var sha string
	if strings.TrimSpace(r.Artifact.SHA256Template) != "" {
		sha, err = resolveSHA256(ctx, r.Artifact.SHA256Template, tctx)
		if err != nil {
			return ResolveResult{}, err
		}
	}

	inner := r.Artifact.InnerPath
	if strings.TrimSpace(inner) != "" {
		inner, err = renderTemplate(inner, tctx)
		if err != nil {
			return ResolveResult{}, err
		}
	}

	ext := r.Artifact.GetFormatExtension()
	cacheFile := filepath.Join(paths.CacheDir, fmt.Sprintf("%s-%s%s", name, version, ext))
	installDir := filepath.Join(paths.PkgsDir, fmt.Sprintf("%s-%s", name, version))
	binName := utils.WithDefault(r.Artifact.BinName, name)

	binaryPath := installDir
	if r.Artifact.Format == "raw" {
		binaryPath = filepath.Join(installDir, binName)
	} else if inner != "" {
		binaryPath = filepath.Join(installDir, inner)
	} else if binName != "" {
		// Archive format but no innerPath provided => assume the binary is at archive root
		binaryPath = filepath.Join(installDir, binName)
	}

	res := ArtifactResolved{
		Name:       name,
		Version:    version,
		URL:        url,
		SHA256:     sha,
		Format:     r.Artifact.Format,
		InnerPath:  inner,
		Mode:       r.Artifact.Mode,
		BinName:    binName,
		CacheFile:  cacheFile,
		InstallDir: installDir,
		BinaryPath: binaryPath,
	}

	// generate default actions if there are none
	actions := r.Actions
	if len(actions) == 0 {
		actions = resolveCommonArtifactActions(res, paths)
	} else {
		// extend tctx with artifact resolved values
		tctx["URL"] = res.URL
		tctx["CacheFile"] = res.CacheFile
		tctx["InstallDir"] = res.InstallDir
		tctx["BinDir"] = paths.BinDir

		rendered := make([]InstallAction, 0, len(actions))
		for _, act := range actions {
			args, err := renderActionArgs(act, tctx)
			if err != nil {
				return ResolveResult{}, err
			}
			act.Args = args
			rendered = append(rendered, act)
		}
		actions = rendered
	}

	return ResolveResult{
		Recipe:   r,
		Target:   target,
		Paths:    paths,
		Resolved: res,
		Actions:  actions,
	}, nil
}

func renderActionArgs(a InstallAction, tctx map[string]string) (map[string]string, error) {
	if len(a.Args) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(a.Args))
	for k, v := range a.Args {
		s, err := renderTemplate(v, tctx)
		if err != nil {
			return nil, fmt.Errorf("render action %q arg %q: %w", a.Type, k, err)
		}
		out[k] = s
	}
	return out, nil
}

// ensureTargetSupported validates detected OS/Arch against allowlists if provided
func ensureTargetSupported(t Target, pi platform.Info) error {
	if len(t.OS) == 0 && len(t.Arch) == 0 {
		return nil
	}
	if len(t.OS) > 0 {
		if !containsFold(t.OS, pi.OS) {
			return fmt.Errorf("unsupported OS %q (allowed: %v)", pi.OS, t.OS)
		}
	}
	if len(t.Arch) > 0 {
		if !containsFold(t.Arch, pi.Arch) {
			return fmt.Errorf("unsupported Arch %q (allowed: %v)", pi.Arch, t.Arch)
		}
	}
	return nil
}

func containsFold(list []string, v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	for _, it := range list {
		if strings.ToLower(strings.TrimSpace(it)) == v {
			return true
		}
	}
	return false
}

func resolveCommonArtifactActions(a ArtifactResolved, paths Paths) []InstallAction {
	var actions []InstallAction

	actions = append(actions, InstallAction{
		Type: "download",
		Args: map[string]string{
			"url":  a.URL,
			"dest": a.CacheFile,
		},
	})

	if strings.TrimSpace(a.SHA256) != "" {
		actions = append(actions, InstallAction{
			Type: "verify",
			Args: map[string]string{
				"file":   a.CacheFile,
				"sha256": a.SHA256,
			},
		})
	}

	actions = append(actions, InstallAction{
		Type: "mkdir",
		Args: map[string]string{
			"path": a.InstallDir,
			"mode": "0755",
		},
	})

	switch strings.ToLower(strings.TrimSpace(a.Format)) {
	case "raw", "":
		actions = append(actions, InstallAction{
			Type: "move",
			Args: map[string]string{
				"src":  a.CacheFile,
				"dest": a.BinaryPath,
			},
		})
	case "gz":
		actions = append(actions, InstallAction{
			Type: "gunzip",
			Args: map[string]string{
				"src":  a.CacheFile,
				"dest": a.BinaryPath,
			},
		})
	case "tar.gz", "tgz":
		actions = append(actions, InstallAction{
			Type: "extract",
			Args: map[string]string{
				"src":  a.CacheFile,
				"dest": a.InstallDir,
			},
		})
	case "zip":
		actions = append(actions, InstallAction{
			Type: "extract",
			Args: map[string]string{
				"src":  a.CacheFile,
				"dest": a.InstallDir,
			},
		})
	default:
	}

	if strings.TrimSpace(a.Mode) != "" {
		actions = append(actions, InstallAction{
			Type: "chmod",
			Args: map[string]string{
				"path": a.BinaryPath,
				"mode": a.Mode,
			},
		})
	}

	// lets hope we end up here to actually use it
	actions = append(actions, InstallAction{
		Type: "symlink",
		Args: map[string]string{
			"src":  a.BinaryPath,
			"dest": filepath.Join(paths.BinDir, a.BinName),
		},
	})

	return actions
}

func resolvePaths(scope InstallScope, override Paths) Paths {
	userRoot := filepath.Join(os.Getenv("HOME"), ".local", "sth")
	systemRoot := filepath.Join(string(os.PathSeparator), "usr", "local", "sth")

	root := override.RootDir
	if root == "" {
		if scope == "" || scope == InstallScopeUser {
			root = userRoot
		} else {
			root = systemRoot
		}
	}

	p := Paths{
		RootDir:   root,
		BinDir:    utils.WithDefault(override.BinDir, filepath.Join(root, "bin")),
		PkgsDir:   utils.WithDefault(override.PkgsDir, filepath.Join(root, "pkgs")),
		CacheDir:  utils.WithDefault(override.CacheDir, filepath.Join(root, "cache")),
		Manifests: utils.WithDefault(override.Manifests, filepath.Join(root, "manifests")),
	}
	return p
}

func resolveVersion(ctx context.Context, vs VersionSource) (string, error) {
	switch vs.Type {
	case "static":
		if vs.Value != "" {
			return vs.Value, nil
		}
		if vs.Fallback != "" {
			return vs.Fallback, nil
		}
		return "", fmt.Errorf("static version empty")
	case "githubRelease":
		return githubLatestRelease(ctx, vs)
	case "githubTag":
		return githubLatestTag(ctx, vs)
	case "httpJson":
		return httpJSONVersion(ctx, vs)
	case "regex":
		return httpRegexVersion(ctx, vs)
	default:
		if vs.Fallback != "" {
			return vs.Fallback, nil
		}
		return "", fmt.Errorf("unsupported version source: %s", vs.Type)
	}
}

func httpRegexVersion(ctx context.Context, vs VersionSource) (string, error) {
	if strings.TrimSpace(vs.URL) == "" || strings.TrimSpace(vs.Pattern) == "" {
		if vs.Fallback != "" {
			return vs.Fallback, nil
		}
		return "", fmt.Errorf("httpRegexVersion: url or pattern missing")
	}

	re, err := regexp.Compile(vs.Pattern)
	if err != nil {
		if vs.Fallback != "" {
			return vs.Fallback, nil
		}
		return "", fmt.Errorf("httpRegexVersion: invalid pattern: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, vs.URL, nil)
	if err != nil {
		if vs.Fallback != "" {
			return vs.Fallback, nil
		}
		return "", fmt.Errorf("httpRegexVersion: request: %w", err)
	}
	req.Header.Set("User-Agent", "sth/1.0")
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Do(req)
	if err != nil || resp == nil {
		if vs.Fallback != "" {
			return vs.Fallback, nil
		}
		return "", fmt.Errorf("httpRegexVersion: do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if vs.Fallback != "" {
			return vs.Fallback, nil
		}
		return "", fmt.Errorf("httpRegexVersion: status %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if vs.Fallback != "" {
			return vs.Fallback, nil
		}
		return "", fmt.Errorf("httpRegexVersion: read: %w", err)
	}
	m := re.FindSubmatch(body)
	if len(m) == 0 {
		if vs.Fallback != "" {
			return vs.Fallback, nil
		}
		return "", fmt.Errorf("httpRegexVersion: no match")
	}
	// named capture "version" if present
	if idx := re.SubexpIndex("version"); idx > 0 && idx < len(m) {
		v := strings.TrimSpace(string(m[idx]))
		if v != "" {
			return v, nil
		}
	}
	if len(m) > 1 {
		return strings.TrimSpace(string(m[1])), nil
	}
	return strings.TrimSpace(string(m[0])), nil
}

func httpJSONVersion(ctx context.Context, vs VersionSource) (string, error) {
	panic("unimplemented")
}

func githubLatestTag(ctx context.Context, vs VersionSource) (string, error) {
	if strings.TrimSpace(vs.Repo) == "" {
		if vs.Fallback != "" {
			return vs.Fallback, nil
		}
		return "", fmt.Errorf("githubTag: repo missing")
	}
	client := githubClient()
	// Fetch first 100 tags and pick the highest semver
	url := fmt.Sprintf("https://api.github.com/repos/%s/tags?per_page=100", vs.Repo)
	req, err := githubReq(ctx, url)
	if err != nil {
		if vs.Fallback != "" {
			return vs.Fallback, nil
		}
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		if vs.Fallback != "" {
			return vs.Fallback, nil
		}
		return "", fmt.Errorf("githubTag: request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		if vs.Fallback != "" {
			return vs.Fallback, nil
		}
		return "", fmt.Errorf("githubTag: status %s", resp.Status)
	}
	var tags []ghTag
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		if vs.Fallback != "" {
			return vs.Fallback, nil
		}
		return "", fmt.Errorf("githubTag: decode: %w", err)
	}
	// Find best semver, strip leading v
	bestVer := ""
	var best utils.SemVer
	for _, t := range tags {
		name := strings.TrimSpace(t.Name)
		if name == "" {
			continue
		}
		name = strings.TrimPrefix(name, "v")
		if v, ok := utils.ParseSemVer(name); ok {
			if bestVer == "" || utils.CmpSemVer(v, best) > 0 {
				bestVer, best = name, v
			}
		}
	}
	if bestVer != "" {
		return bestVer, nil
	}
	// Fallback: if no semver parse succeeded, return first tag without 'v' prefix
	if len(tags) > 0 {
		return strings.TrimPrefix(tags[0].Name, "v"), nil
	}
	if vs.Fallback != "" {
		return vs.Fallback, nil
	}
	return "", fmt.Errorf("githubTag: no tags found")
}

func githubLatestRelease(ctx context.Context, vs VersionSource) (string, error) {
	if strings.TrimSpace(vs.Repo) == "" {
		if vs.Fallback != "" {
			return vs.Fallback, nil
		}
		return "", fmt.Errorf("githubRelease: repo missing")
	}
	client := githubClient()

	if !vs.Prerelease && strings.TrimSpace(vs.Constraint) == "" {
		url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", vs.Repo)
		req, err := githubReq(ctx, url)
		if err != nil {
			if vs.Fallback != "" {
				return vs.Fallback, nil
			}
			return "", err
		}
		resp, err := client.Do(req)
		if err != nil {
			if vs.Fallback != "" {
				return vs.Fallback, nil
			}
			return "", fmt.Errorf("githubRelease: request: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			if vs.Fallback != "" {
				return vs.Fallback, nil
			}
			return "", fmt.Errorf("githubRelease: status %s", resp.Status)
		}
		var rel ghRelease
		if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
			if vs.Fallback != "" {
				return vs.Fallback, nil
			}
			return "", fmt.Errorf("githubRelease: decode: %w", err)
		}
		return strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v"), nil
	}

	// Let's hope we don't have to go this far...
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=100", vs.Repo)
	req, err := githubReq(ctx, url)
	if err != nil {
		if vs.Fallback != "" {
			return vs.Fallback, nil
		}
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		if vs.Fallback != "" {
			return vs.Fallback, nil
		}
		return "", fmt.Errorf("githubRelease: request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		if vs.Fallback != "" {
			return vs.Fallback, nil
		}
		return "", fmt.Errorf("githubRelease: status %s", resp.Status)
	}
	var rels []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rels); err != nil {
		if vs.Fallback != "" {
			return vs.Fallback, nil
		}
		return "", fmt.Errorf("githubRelease: decode: %w", err)
	}
	for _, r := range rels {
		if r.Prerelease && !vs.Prerelease {
			continue
		}
		tag := strings.TrimPrefix(strings.TrimSpace(r.TagName), "v")
		if tag == "" {
			continue
		}
		// return the first non-prerelease release
		return tag, nil
	}
	if vs.Fallback != "" {
		return vs.Fallback, nil
	}
	return "", fmt.Errorf("githubRelease: no matching releases")
}

func renderTemplate(tpl string, ctx map[string]string) (string, error) {
	if strings.TrimSpace(tpl) == "" {
		return "", nil
	}

	funcs := template.FuncMap{
		"lower":   strings.ToLower,
		"upper":   strings.ToUpper,
		"replace": strings.ReplaceAll,
		"get": func(key string) string {
			if v, ok := ctx[key]; ok {
				return v
			}
			return ""
		},
	}

	t, err := template.New("tpl").Funcs(funcs).Option("missingkey=default").Parse(tpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

func resolveSHA256(ctx context.Context, shaTpl string, tctx map[string]string) (string, error) {
	rendered, err := renderTemplate(shaTpl, tctx)
	if err != nil {
		return "", fmt.Errorf("resolveSHA256: render: %w", err)
	}
	val := strings.TrimSpace(rendered)
	if val == "" {
		return "", fmt.Errorf("resolveSHA256: empty rendered value")
	}
	if isHexSHA256(val) {
		return strings.ToLower(val), nil
	}

	// if it's a file to download...
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, val, nil)
	if err != nil {
		return "", fmt.Errorf("resolveSHA256: request: %w", err)
	}
	req.Header.Set("User-Agent", "sth/1.0")
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("resolveSHA256: do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("resolveSHA256: status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("resolveSHA256: read: %w", err)
	}
	hash := strings.TrimSpace(string(body))
	if !isHexSHA256(hash) {
		return "", fmt.Errorf("resolveSHA256: body is not a sha256 hex")
	}
	return strings.ToLower(hash), nil
}

func isHexSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < 64; i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}
