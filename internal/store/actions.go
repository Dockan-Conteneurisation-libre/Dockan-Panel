package store

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dockan-panel/internal/config"
	"dockan-panel/internal/runner"
)

// Store holds the paths and runner used by Store actions.
type Store struct {
	R         *runner.Runner
	Dir       string // installed Store (storage/store/Dockan-Store)
	RootDir   string // storage/store
	StateFile string
	Home      string
	Root      bool
}

const maxConfig = 512 * 1024

// UpdateScript downloads the Store release (or main as a fallback) into
// RootDir/Dockan-Store.
func (s *Store) UpdateScript() string {
	return strings.Join([]string{
		"set -eu",
		"base=" + runner.Quote(s.RootDir),
		"url=" + runner.Quote(config.StoreRelURL),
		"fallback_url=" + runner.Quote(config.StoreAltURL),
		`tmp="$(mktemp -d)"`,
		`cleanup() { rm -rf "$tmp"; }`,
		"trap cleanup EXIT INT TERM",
		`mkdir -p "$base"`,
		`source="release"`,
		`if ! curl -fsSL "$url" -o "$tmp/dockan-store.tar.gz"; then`,
		`  echo "Latest release archive not ready, downloading Store from main branch..."`,
		`  curl -fsSL "$fallback_url" -o "$tmp/dockan-store.tar.gz"`,
		`  source="main"`,
		"fi",
		`mkdir -p "$tmp/extract"`,
		`tar -xzf "$tmp/dockan-store.tar.gz" -C "$tmp/extract" --no-same-owner`,
		`src="$(find "$tmp/extract" -mindepth 1 -maxdepth 1 -type d | head -n 1)"`,
		`test -n "$src"`,
		`test -x "$src/dockan-store"`,
		`rm -rf "$base/Dockan-Store"`,
		`mkdir -p "$base/Dockan-Store"`,
		`cp -a "$src/." "$base/Dockan-Store/"`,
		`test -x "$base/Dockan-Store/dockan-store"`,
		`echo "Dockan Store installed from $source in $base/Dockan-Store"`,
	}, "\n")
}

// Update installs or refreshes the Store.
func (s *Store) Update(ctx context.Context) (string, error) {
	return runner.Check(s.R.Shell(ctx, s.UpdateScript()))
}

func (s *Store) dockanLine(args ...string) string {
	return runner.Quote(append([]string{"env", "DOCKAN_PORT_BIND_ADDR=0.0.0.0", s.R.Bin}, args...)...)
}

func (s *Store) installLines(app, target string) []string {
	return []string{
		s.UpdateScript(),
		"PATH=" + runner.Quote(s.R.SudoPath()) + ":$PATH",
		"test -x " + runner.Quote(s.Dir+"/dockan-store"),
		"test -d " + runner.Quote(s.Dir+"/apps/"+app),
		"if [ -f " + runner.Quote(target+"/dockan.yml") + " ]; then",
		`  echo "App folder already exists, keeping its files."`,
		"else",
		"  cd " + runner.Quote(s.Dir),
		"  ./dockan-store install " + runner.Quote(app) + " " + runner.Quote(target),
		"fi",
	}
}

// Request carries the parameters of a Store action.
type Request struct {
	App    string
	Target string
	Config string // dockan.yml content, optional
}

func (s *Store) clean(req Request) (App, string, error) {
	app, err := s.FindApp(req.App)
	if err != nil {
		return App{}, "", err
	}
	target, err := CleanTarget(req.Target)
	if err != nil {
		return App{}, "", err
	}
	return app, target, nil
}

// Install copies the app template, optionally saves its config and starts
// it ("deploy").
func (s *Store) Install(ctx context.Context, req Request, deploy bool) (string, error) {
	app, target, err := s.clean(req)
	if err != nil {
		return "", err
	}
	script := strings.Join(append(append([]string{"set -eu"}, s.installLines(app.ID, target)...),
		runner.Quote("printf", "Store app ready: %s -> %s\n", app.ID, target)), "\n")
	out := []string{}
	text, err := runner.Check(s.R.Shell(ctx, script))
	if err != nil {
		return "", err
	}
	out = append(out, text)
	if strings.TrimSpace(req.Config) != "" {
		msg, err := SaveConfig(target, req.Config)
		if err != nil {
			return "", err
		}
		out = append(out, msg)
	}
	if deploy {
		text, err := runner.Check(s.R.Shell(ctx, s.dockanLine("compose", "up", "-f", target+"/dockan.yml")))
		if err != nil {
			return "", err
		}
		out = append(out, text)
	}
	if err := s.RecordImages(app.Requires); err != nil {
		out = append(out, "Image state not recorded: "+err.Error())
	}
	return strings.TrimSpace(strings.Join(out, "\n")), nil
}

// Launch starts an installed app (as root when possible).
func (s *Store) Launch(ctx context.Context, req Request) (string, error) {
	app, target, err := s.clean(req)
	if err != nil {
		return "", err
	}
	script := strings.Join([]string{
		"set -eu",
		"PATH=" + runner.Quote(s.R.SudoPath()) + ":$PATH",
		"test -f " + runner.Quote(target+"/dockan.yml"),
		s.dockanLine("compose", "up", "-f", target+"/dockan.yml"),
		runner.Quote("printf", "Store app launched: %s -> %s\n", app.ID, target),
	}, "\n")
	return runner.SystemText(s.R.SystemShell(ctx, script))
}

// UpdateImages refreshes the app images, then optionally redeploys.
func (s *Store) UpdateImages(ctx context.Context, req Request, redeploy bool) (string, error) {
	app, target, err := s.clean(req)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(target); err != nil {
		return "", errors.New("the app is not installed yet: install it first")
	}
	refresh := "DOCKAN_STORE_FORCE_IMAGES=1 DOCKAN_STORE_REFRESH_IMAGES=1 ./dockan-store images " + runner.Quote(app.ID)
	update := refresh
	if redeploy {
		update = strings.Join([]string{
			`if ./dockan-store help 2>/dev/null | grep -q "dockan-store update"; then`,
			"  ./dockan-store update " + runner.Quote(app.ID) + " " + runner.Quote(target),
			"else",
			"  " + refresh,
			"  " + s.dockanLine("compose", "up", "-f", target+"/dockan.yml"),
			"fi",
		}, "\n")
	}
	script := strings.Join([]string{
		"set -eu", s.UpdateScript(),
		"PATH=" + runner.Quote(s.R.SudoPath()) + ":$PATH",
		"test -x " + runner.Quote(s.Dir+"/dockan-store"),
		"test -f " + runner.Quote(target+"/dockan.yml"),
		"cd " + runner.Quote(s.Dir),
		update,
		runner.Quote("printf", "Store app updated: %s -> %s\n", app.ID, target),
	}, "\n")
	out, err := runner.Check(s.R.Shell(ctx, script))
	if err != nil {
		return "", err
	}
	if err := s.RecordImages(app.Requires); err != nil {
		out += "\nImage state not recorded: " + err.Error()
	}
	return out, nil
}

// Configure saves dockan.yml and optionally redeploys.
func (s *Store) Configure(ctx context.Context, req Request, redeploy bool) (string, error) {
	_, target, err := s.clean(req)
	if err != nil {
		return "", err
	}
	msg, err := SaveConfig(target, req.Config)
	if err != nil {
		return "", err
	}
	if !redeploy {
		return msg, nil
	}
	text, err := runner.Check(s.R.Shell(context.WithoutCancel(ctx), s.dockanLine("compose", "redeploy", "-f", target+"/dockan.yml")))
	if err != nil {
		return "", err
	}
	return msg + "\n" + text, nil
}

// Autostart installs a systemd service for the app (installing it first
// when install is true).
func (s *Store) Autostart(ctx context.Context, req Request, install bool) (string, error) {
	app, target, err := s.clean(req)
	if err != nil {
		return "", err
	}
	lines := []string{"set -eu", "PATH=" + runner.Quote(s.R.SudoPath()) + ":$PATH"}
	if install {
		lines = append(lines, s.installLines(app.ID, target)...)
	}
	if strings.TrimSpace(req.Config) != "" {
		cfg, err := configShellLines(target, req.Config)
		if err != nil {
			return "", err
		}
		lines = append(lines, cfg...)
	}
	file := target + "/dockan.yml"
	lines = append(lines,
		"test -f "+runner.Quote(file),
		"if ! "+s.dockanLine("compose", "autostart", "-f", file, "--name", app.ID)+"; then",
		`  echo "Native compose autostart unavailable, falling back to service install."`,
		"  "+runner.Quote(s.R.Bin, "service", "install", "-f", file, "--name", app.ID),
		"  systemctl daemon-reload",
		"  "+runner.Quote("systemctl", "enable", "--now", "dockan-"+app.ID+".service"),
		"fi",
		runner.Quote("printf", "Store app auto-start enabled: %s -> %s\n", app.ID, target),
	)
	out, err := runner.SystemText(s.R.SystemShell(ctx, strings.Join(lines, "\n")))
	if err != nil {
		return "", err
	}
	if install {
		if err := s.RecordImages(app.Requires); err != nil {
			out += "\nImage state not recorded: " + err.Error()
		}
	}
	return out, nil
}

// DisableAutostart removes the app's systemd service.
func (s *Store) DisableAutostart(ctx context.Context, req Request) (string, error) {
	app, target, err := s.clean(req)
	if err != nil {
		return "", err
	}
	file := target + "/dockan.yml"
	script := strings.Join([]string{
		"set -eu",
		"PATH=" + runner.Quote(s.R.SudoPath()) + ":$PATH",
		"test -f " + runner.Quote(file),
		"if ! " + runner.Quote(s.R.Bin, "compose", "no-autostart", "-f", file, "--name", app.ID) + "; then",
		`  echo "Native compose no-autostart unavailable, falling back to systemctl."`,
		"  systemctl disable --now " + runner.Quote("dockan-"+app.ID+".service") + " 2>/dev/null || true",
		"  systemctl daemon-reload",
		"fi",
		runner.Quote("printf", "Store app auto-start disabled: %s -> %s\n", app.ID, target),
	}, "\n")
	return runner.SystemText(s.R.SystemShell(ctx, script))
}

// ConfigYAML returns the editable dockan.yml of an app: the installed one,
// or the template before install.
func (s *Store) ConfigYAML(id, target string) string {
	file := filepath.Join(s.Dir, "apps", id, "dockan.yml")
	if fileExists(filepath.Join(target, "dockan.yml")) {
		file = filepath.Join(target, "dockan.yml")
	}
	st, err := os.Stat(file)
	if err != nil || st.Size() > maxConfig {
		return ""
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	return string(raw)
}

func normalizeConfig(yaml string) (string, error) {
	yaml = strings.TrimRight(strings.ReplaceAll(strings.ReplaceAll(yaml, "\r\n", "\n"), "\r", "\n"), " \n\t")
	if yaml == "" {
		return "", errors.New("dockan.yml is empty")
	}
	if len(yaml) > maxConfig {
		return "", errors.New("dockan.yml is too large")
	}
	return yaml, nil
}

// SaveConfig writes target/dockan.yml after a timestamped backup.
func SaveConfig(target, yaml string) (string, error) {
	yaml, err := normalizeConfig(yaml)
	if err != nil {
		return "", err
	}
	if st, err := os.Stat(target); err != nil || !st.IsDir() {
		return "", errors.New("the app is not installed yet: install it first")
	}
	file := filepath.Join(target, "dockan.yml")
	if st, err := os.Lstat(file); err == nil {
		if st.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("refusing to overwrite a symbolic link")
		}
		old, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("read dockan.yml: %w", err)
		}
		if err := os.WriteFile(file+".bak-"+time.Now().Format("20060102-150405"), old, 0o644); err != nil {
			return "", fmt.Errorf("backup dockan.yml: %w", err)
		}
	}
	if err := os.WriteFile(file, []byte(yaml+"\n"), 0o644); err != nil {
		return "", fmt.Errorf("save dockan.yml: %w", err)
	}
	return "Configuration saved: " + file, nil
}

func configShellLines(target, yaml string) ([]string, error) {
	yaml, err := normalizeConfig(yaml)
	if err != nil {
		return nil, err
	}
	file := target + "/dockan.yml"
	return []string{
		"config_file=" + runner.Quote(file),
		`if [ -L "$config_file" ]; then echo "Refusing to overwrite a symbolic link." >&2; exit 1; fi`,
		`if [ -f "$config_file" ]; then cp "$config_file" ` + runner.Quote(file+".bak-"+time.Now().Format("20060102-150405")) + "; fi",
		"printf %s " + runner.Quote(base64.StdEncoding.EncodeToString([]byte(yaml+"\n"))) + ` | base64 -d > "$config_file"`,
	}, nil
}
