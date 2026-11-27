// Package config gathers the panel settings read from the environment and
// the storage layout shared with the previous PHP panel.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Version is the panel version, set at build time with
// -ldflags "-X dockan-panel/internal/config.Version=vX.Y.Z".
var Version = "v0.2.1"

const (
	AppName     = "Dockan Panel"
	PanelRepo   = "Dockan-Conteneurisation-libre/Dockan-Panel"
	PanelUnit   = "dockan-dockan-panel.service"
	StoreRelURL = "https://github.com/Dockan-Conteneurisation-libre/Dockan-store/releases/latest/download/dockan-store.tar.gz"
	StoreAltURL = "https://github.com/Dockan-Conteneurisation-libre/Dockan-store/archive/refs/heads/main.tar.gz"
)

// Config holds runtime settings.
type Config struct {
	Dockan     string // dockan binary
	Bind       string
	Port       string
	Storage    string // persistent data (users, stacks, backups, store)
	UpdateDir  string // folder the panel updates itself into
	HomeDir    string
	PathPrefix string // extra PATH entries for commands (~/.local/bin)
}

// Load reads the configuration from the environment. Storage defaults to
// "storage" next to the executable, which is where the previous panel kept
// its data and where Dockan mounts the dockan-panel-data volume.
func Load() (Config, error) {
	exe, err := os.Executable()
	if err != nil {
		return Config{}, fmt.Errorf("executable path: %w", err)
	}
	exeDir := filepath.Dir(exe)
	home := os.Getenv("HOME")
	c := Config{
		Dockan:    envOr("DOCKAN_BIN", "dockan"),
		Bind:      envOr("BIND", "0.0.0.0"),
		Port:      envOr("PORT", "9090"),
		Storage:   envOr("PANEL_STORAGE", filepath.Join(exeDir, "storage")),
		UpdateDir: strings.TrimSpace(os.Getenv("PANEL_UPDATE_DIR")),
		HomeDir:   home,
	}
	if c.UpdateDir == "" {
		c.UpdateDir = exeDir
		if strings.HasPrefix(exeDir, "/var/lib/dockan/images/") && isDir("/srv/dockan-panel") {
			c.UpdateDir = "/srv/dockan-panel"
		}
	}
	if home != "" {
		c.PathPrefix = filepath.Join(home, ".local", "bin")
	}
	return c, nil
}

// Paths derived from the storage folder.
func (c Config) BackupDir() string { return filepath.Join(c.Storage, "backups") }
func (c Config) StacksDir() string { return filepath.Join(c.Storage, "stacks") }
func (c Config) StoreRoot() string { return filepath.Join(c.Storage, "store") }
func (c Config) StoreDir() string  { return filepath.Join(c.StoreRoot(), "Dockan-Store") }
func (c Config) AuthFile() string  { return filepath.Join(c.Storage, "auth-users.json") }
func (c Config) ImageStateFile() string {
	return filepath.Join(c.Storage, "store-image-state.json")
}

// EnsureStorage creates the storage folders.
func (c Config) EnsureStorage() error {
	for _, d := range []struct {
		path string
		mode os.FileMode
	}{
		{c.Storage, 0o755}, {c.BackupDir(), 0o755}, {c.StacksDir(), 0o755}, {c.StoreRoot(), 0o755},
	} {
		if err := os.MkdirAll(d.path, d.mode); err != nil {
			return fmt.Errorf("storage %s: %w", d.path, err)
		}
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
