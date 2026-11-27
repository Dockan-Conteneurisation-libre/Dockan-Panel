// Package system covers host maintenance from the panel: dependency
// profiles, language runtimes, Dockan CLI updates and panel self-updates.
package system

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"dockan-panel/internal/config"
	"dockan-panel/internal/runner"
)

// Profiles are the "dockan deps install" profiles offered in the panel.
var Profiles = []string{"core", "tools", "frontend", "network", "database", "web", "build", "debug", "isolation", "full"}

// Runtimes are the "dockan deps runtime" references offered in the panel.
var Runtimes = []string{"php:8.3", "node:20", "python:3.12", "golang:1.22", "openjdk:21", "frankenphp"}

// Mode is what a maintenance action does.
type Mode string

const (
	Preview Mode = "preview" // dry run, without privileges
	Install Mode = "install" // run as root
	Command Mode = "command" // show the command line only
)

var (
	versionRe = regexp.MustCompile(`^v?[0-9][A-Za-z0-9._-]{0,63}$`)
	refRe     = regexp.MustCompile(`^v[0-9][A-Za-z0-9._-]{0,63}$`)
	pkgRe     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.+:=*~_@-]{0,127}$`)
)

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// Info describes the panel host.
type Info struct {
	PanelVersion  string `json:"panelVersion"`
	DockanVersion string `json:"dockanVersion"`
	DockanPath    string `json:"dockanPath"`
	User          string `json:"user"`
	Root          bool   `json:"root"`
	Automation    string `json:"automation"` // "root", "sudo" or "disabled"
	UpdateDir     string `json:"updateDir"`
	Storage       string `json:"storage"`
	Arch          string `json:"arch"`
	Doctor        string `json:"doctor"`
}

// Collect gathers host information.
func Collect(ctx context.Context, r *runner.Runner, c config.Config) Info {
	info := Info{
		PanelVersion: config.Version, Root: runner.IsRoot(), UpdateDir: c.UpdateDir,
		Storage: c.Storage, Arch: runtime.GOOS + "/" + runtime.GOARCH,
		DockanVersion: r.DockanOrEmpty(ctx, "version"), Doctor: r.DockanOrEmpty(ctx, "doctor"),
	}
	if p, err := exec.LookPath(r.Bin); err == nil {
		info.DockanPath = p
	} else if c.PathPrefix != "" {
		if _, err := os.Stat(c.PathPrefix + "/" + r.Bin); err == nil {
			info.DockanPath = c.PathPrefix + "/" + r.Bin
		}
	}
	if u, err := r.Run(ctx, nil, "id", "-un"); err == nil {
		info.User = fmt.Sprintf("%s (%d)", strings.TrimSpace(u.Stdout), os.Geteuid())
	}
	switch {
	case info.Root:
		info.Automation = "root"
	default:
		res, err := r.Run(ctx, nil, "sudo", "-n", "true")
		if err == nil && res.Code == 0 {
			info.Automation = "sudo"
		} else {
			info.Automation = "disabled"
		}
	}
	return info
}

// Deps runs a dependency profile, a package list or a runtime install.
func Deps(ctx context.Context, r *runner.Runner, kind, value string, mode Mode) (string, error) {
	var args []string
	switch kind {
	case "profile":
		if !contains(Profiles, value) {
			return "", errors.New("unknown dependency profile")
		}
		args = []string{"deps", "install", value}
	case "runtime":
		if !contains(Runtimes, value) {
			return "", errors.New("unknown runtime")
		}
		args = []string{"deps", "runtime", value}
	case "packages":
		pkgs := strings.Fields(value)
		if len(pkgs) == 0 || len(pkgs) > 64 {
			return "", errors.New("give between 1 and 64 packages")
		}
		for _, p := range pkgs {
			if !pkgRe.MatchString(p) {
				return "", fmt.Errorf("invalid package name: %s", p)
			}
		}
		args = append([]string{"deps", "install"}, pkgs...)
	default:
		return "", errors.New("unknown dependency action")
	}
	switch mode {
	case Preview:
		return r.DockanText(ctx, "", append(args, "--dry-run")...)
	case Install:
		return runner.SystemText(r.SystemDockan(ctx, append(args, "-y")...))
	case Command:
		return r.SystemCommandLine(append(args, "-y")...), nil
	}
	return "", errors.New("unknown mode")
}

// UpdateDockan runs "dockan update".
func UpdateDockan(ctx context.Context, r *runner.Runner, version string, systemWide bool, mode Mode) (string, error) {
	version = strings.TrimSpace(version)
	if version != "" && !versionRe.MatchString(version) {
		return "", errors.New("invalid release version")
	}
	args := []string{"update"}
	if version != "" {
		args = append(args, "--version", version)
	}
	if systemWide {
		args = append(args, "--system")
	}
	// dockan update has no dry run: a preview only shows the command.
	if mode == Preview {
		mode = Command
	}
	switch {
	case mode == Command && systemWide:
		return r.SystemCommandLine(args...), nil
	case mode == Command:
		return runner.Quote(append([]string{r.Bin}, args...)...), nil
	case systemWide:
		return runner.SystemText(r.SystemDockan(ctx, args...))
	}
	out, err := r.DockanText(ctx, "", args...)
	if out == "" && err == nil {
		out = "Dockan updated."
	}
	return out, err
}

// PanelUpdateScript downloads a panel release for this machine, checks its
// SHA-256 sum, installs the binary in dir and restarts the service.
func PanelUpdateScript(ref, dir string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		ref = "latest"
	}
	if ref != "latest" && !refRe.MatchString(ref) {
		return "", errors.New("invalid release tag: use latest or a tag such as v0.2.1")
	}
	asset := "dockan-panel-linux-" + runtime.GOARCH + ".tar.gz"
	base := "https://github.com/" + config.PanelRepo + "/releases/download/" + ref
	if ref == "latest" {
		base = "https://github.com/" + config.PanelRepo + "/releases/latest/download"
	}
	return strings.Join([]string{
		"set -eu",
		"app_dir=" + runner.Quote(dir),
		"base=" + runner.Quote(base),
		"asset=" + runner.Quote(asset),
		"service=" + runner.Quote(config.PanelUnit),
		`tmp="$(mktemp -d)"`,
		`cleanup() { rm -rf "$tmp"; }`,
		"trap cleanup EXIT INT TERM",
		`test -d "$app_dir"`,
		`curl -fsSL "$base/$asset" -o "$tmp/$asset"`,
		`curl -fsSL "$base/SHA256SUMS" -o "$tmp/SHA256SUMS"`,
		`(cd "$tmp" && grep " $asset\$" SHA256SUMS | sha256sum -c -)`,
		`tar -xzf "$tmp/$asset" -C "$tmp" --no-same-owner`,
		`test -x "$tmp/dockan-panel/dockan-panel"`,
		`install -m 0755 "$tmp/dockan-panel/dockan-panel" "$app_dir/dockan-panel.new"`,
		`mv -f "$app_dir/dockan-panel.new" "$app_dir/dockan-panel"`,
		`for f in README.md dockan.yml Dockanfile restore-prod-storage.sh; do`,
		`  if [ -f "$tmp/dockan-panel/$f" ]; then install -m 0644 "$tmp/dockan-panel/$f" "$app_dir/$f"; fi`,
		`done`,
		`chmod 0755 "$app_dir/restore-prod-storage.sh" 2>/dev/null || true`,
		`if command -v restorecon >/dev/null 2>&1; then restorecon -RF "$app_dir" 2>/dev/null || true; fi`,
		`echo "Dockan Panel updated from release $asset ($(basename "$base"))."`,
		`echo "Storage was not modified: $app_dir/storage"`,
		`if command -v systemctl >/dev/null 2>&1; then systemctl try-restart --no-block "$service" 2>/dev/null || true; fi`,
	}, "\n"), nil
}

// UpdatePanel runs or shows the panel update.
func UpdatePanel(ctx context.Context, r *runner.Runner, ref, dir string, mode Mode) (string, error) {
	script, err := PanelUpdateScript(ref, dir)
	if err != nil {
		return "", err
	}
	if mode == Command {
		return runner.SystemScriptLine(script), nil
	}
	return runner.SystemText(r.SystemShell(ctx, script))
}
