// Package store drives the Dockan Store: catalog, install, update, launch,
// auto-start and configuration of ready-made apps.
package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// App is a catalog entry.
type App struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Summary     string   `json:"summary"`
	DefaultPort int      `json:"default_port,omitempty"`
	Requires    []string `json:"requires"`
}

// Catalog reads catalog.json from the installed Store, or the bundled list.
func (s *Store) Catalog() []App {
	raw, err := os.ReadFile(filepath.Join(s.Dir, "catalog.json"))
	if err == nil {
		var data struct {
			Apps []App `json:"apps"`
		}
		if json.Unmarshal(raw, &data) == nil && len(data.Apps) > 0 {
			out := data.Apps[:0]
			for _, a := range data.Apps {
				if appIDRe.MatchString(a.ID) {
					if a.Name == "" {
						a.Name = a.ID
					}
					out = append(out, a)
				}
			}
			return out
		}
	}
	return fallbackApps()
}

// CatalogSource names where the catalog comes from.
func (s *Store) CatalogSource() string {
	p := filepath.Join(s.Dir, "catalog.json")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return "bundled"
}

var appIDRe = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

// FindApp returns a catalog app by id.
func (s *Store) FindApp(id string) (App, error) {
	if !appIDRe.MatchString(id) {
		return App{}, errors.New("invalid Store app")
	}
	for _, a := range s.Catalog() {
		if a.ID == id {
			return a, nil
		}
	}
	return App{}, errors.New("unknown Store app")
}

// CleanTarget validates an absolute install folder.
func CleanTarget(t string) (string, error) {
	t = strings.TrimSpace(t)
	if t == "" || strings.ContainsAny(t, "\x00\n\r") {
		return "", errors.New("invalid install folder")
	}
	if !strings.HasPrefix(t, "/") {
		return "", errors.New("use an absolute install folder")
	}
	for _, part := range strings.Split(t, "/") {
		if part == ".." {
			return "", errors.New("the install folder cannot contain ..")
		}
	}
	return strings.TrimRight(t, "/"), nil
}

// DefaultBase is the parent folder of app installs.
func (s *Store) DefaultBase() string {
	if s.Root {
		return "/srv/dockan-apps"
	}
	return filepath.Join(s.Home, "dockan-apps")
}

// Target finds where an app is installed, or its default folder.
func (s *Store) Target(id string) string {
	cands := []string{filepath.Join(s.DefaultBase(), id), filepath.Join("/srv/dockan-apps", id)}
	if s.Home != "" {
		cands = append(cands, filepath.Join(s.Home, "dockan-apps", id))
	}
	for _, c := range cands {
		if fileExists(filepath.Join(c, "dockan.yml")) {
			return c
		}
	}
	return cands[0]
}

// ServiceEnabled reports whether the app starts on boot.
func ServiceEnabled(id string) bool {
	return fileExists("/etc/systemd/system/dockan-" + id + ".service")
}

var (
	projectRe = regexp.MustCompile(`(?m)^name:\s*["']?([A-Za-z0-9_.-]+)["']?\s*$`)
	serviceRe = regexp.MustCompile(`^  ([A-Za-z0-9_.-]+):\s*$`)
)

// ComposeContainers returns the container names a compose file creates
// (<project>-<service>).
func ComposeContainers(file, fallback string) []string {
	raw, _ := os.ReadFile(file)
	yaml := string(raw)
	project := fallback
	if m := projectRe.FindStringSubmatch(yaml); m != nil {
		project = m[1]
	}
	var services []string
	inServices := false
	for _, line := range strings.Split(yaml, "\n") {
		if strings.TrimRight(line, " \r") == "services:" {
			inServices = true
			continue
		}
		if inServices {
			if line != "" && line[0] != ' ' && line[0] != '#' {
				break
			}
			if m := serviceRe.FindStringSubmatch(strings.TrimRight(line, "\r")); m != nil {
				services = append(services, m[1])
			}
		}
	}
	if len(services) == 0 {
		services = []string{"web"}
	}
	out := make([]string, len(services))
	for i, sv := range services {
		out[i] = project + "-" + sv
	}
	return out
}

// ImageStatus compares the images recorded at install with the Store's
// registry index: "current", "available" or "unknown".
func (s *Store) ImageStatus(requires []string) string {
	latest := s.registryIndex()
	if len(latest) == 0 || len(requires) == 0 {
		return "unknown"
	}
	state := s.loadImageState()
	outdated := false
	for _, img := range requires {
		sha := latest[img]
		rec := state[img].SHA
		if sha == "" || rec == "" {
			return "unknown"
		}
		if rec != sha {
			outdated = true
		}
	}
	if outdated {
		return "available"
	}
	return "current"
}

type imageState struct {
	SHA        string `json:"sha"`
	Archive    string `json:"archive"`
	RecordedAt string `json:"recorded_at"`
}

func (s *Store) registryIndex() map[string]string {
	raw, err := os.ReadFile(filepath.Join(s.Dir, "registry", "index.tsv"))
	out := map[string]string{}
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) >= 3 && strings.TrimSpace(parts[0]) != "" {
			out[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[2])
		}
	}
	return out
}

func (s *Store) loadImageState() map[string]imageState {
	out := map[string]imageState{}
	if raw, err := os.ReadFile(s.StateFile); err == nil {
		_ = json.Unmarshal(raw, &out) // an unreadable state means "unknown"
	}
	return out
}

// RecordImages stores the current registry digests of an app's images.
func (s *Store) RecordImages(requires []string) error {
	latest := s.registryIndex()
	if len(latest) == 0 || len(requires) == 0 {
		return nil
	}
	state := s.loadImageState()
	for _, img := range requires {
		if sha := latest[img]; sha != "" {
			state[img] = imageState{SHA: sha, RecordedAt: time.Now().Format(time.RFC3339)}
		}
	}
	raw, err := json.MarshalIndent(state, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.StateFile, append(raw, '\n'), 0o644)
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func fallbackApps() []App {
	a := func(id, name, cat, summary string, port int, req ...string) App {
		return App{ID: id, Name: name, Category: cat, Summary: summary, DefaultPort: port, Requires: req}
	}
	return []App{
		a("anarcosyndicalismebook", "AnarcosyndicalismeBOOK", "cms", "Anarcho-syndicalist CMS and publishing engine with SQLite/PHP.", 8097, "anarcosyndicalismebook:local"),
		a("bookstack", "BookStack", "wiki", "Documentation wiki with MariaDB.", 8087, "bookstack:local", "mariadb:local"),
		a("discourse", "Discourse", "community", "Community discussion platform with PostgreSQL and Redis.", 8091, "discourse:local", "postgres:local", "redis:local"),
		a("drawio", "draw.io", "diagrams", "Diagram editor.", 8089, "drawio:local"),
		a("forgejo", "Forgejo", "git", "Community-driven lightweight Git forge with PostgreSQL.", 3004, "forgejo:local", "postgres:local"),
		a("ghost", "Ghost", "publishing", "Publishing platform with MySQL.", 2368, "ghost:local", "mysql:local"),
		a("gitea", "Gitea", "git", "Lightweight Git forge with PostgreSQL.", 3000, "gitea:local", "postgres:local"),
		a("grafana", "Grafana", "monitoring", "Dashboards and visualization for metrics.", 3002, "grafana:local"),
		a("hedgedoc", "HedgeDoc", "notes", "Collaborative markdown notes with PostgreSQL.", 3003, "hedgedoc:local", "postgres:local"),
		a("immich", "Immich", "photos", "High performance self-hosted photo and video backup solution.", 2283, "immich-server:local", "immich-machine-learning:local", "postgres:local", "redis:local"),
		a("jellyfin", "Jellyfin", "media", "Local media server.", 8096, "jellyfin:local"),
		a("libretranslate", "LibreTranslate", "ai", "Local machine translation API and web UI.", 5000, "libretranslate:local"),
		a("matomo", "Matomo", "analytics", "Web analytics with MariaDB.", 8083, "matomo:local", "mariadb:local"),
		a("mattermost", "Mattermost", "chat", "Open source secure collaboration and chat platform.", 8065, "mattermost:local", "postgres:local"),
		a("mealie", "Mealie", "food", "Recipe manager and meal planner with SQLite/PostgreSQL.", 9925, "mealie:local"),
		a("miniflux", "Miniflux", "rss", "Minimal RSS reader with PostgreSQL.", 8085, "miniflux:local", "postgres:local"),
		a("n8n", "n8n", "automation", "Workflow automation with PostgreSQL.", 5678, "n8n:local", "postgres:local"),
		a("nextcloud", "Nextcloud", "files", "Private files, sync, calendar, contacts, and collaboration.", 8081, "nextcloud:local", "mariadb:local", "redis:local"),
		a("nginx-proxy-manager", "Nginx Proxy Manager", "proxy", "Web UI for reverse proxy hosts and TLS certificates.", 8181, "nginx-proxy-manager:local"),
		a("paperless-ngx", "Paperless-ngx", "documents", "Document management with OCR, PostgreSQL, and Redis.", 8000, "paperless-ngx:local", "postgres:local", "redis:local"),
		a("plausible", "Plausible Analytics", "analytics", "Lightweight privacy-focused web analytics with PostgreSQL and ClickHouse.", 8001, "plausible:local", "postgres:local", "clickhouse:local"),
		a("prestashop", "PrestaShop", "ecommerce", "Open source e-commerce web platform with MySQL.", 8092, "prestashop:local", "mysql:local"),
		a("prometheus", "Prometheus", "monitoring", "Metrics database and scraper.", 9091, "prometheus:local"),
		a("static-site", "Static Site", "web", "Serve a static public folder with Caddy.", 8088, "caddy:local"),
		a("stirling-pdf", "Stirling-PDF", "documents", "Robust locally hosted web application for PDF manipulation.", 8094, "stirling-pdf:local"),
		a("syncthing", "Syncthing", "sync", "Peer-to-peer file synchronization.", 8384, "syncthing:local"),
		a("uptime-kuma", "Uptime Kuma", "monitoring", "Monitoring dashboard for services and websites.", 3001, "uptime-kuma:local"),
		a("vaultwarden", "Vaultwarden", "passwords", "Lightweight Bitwarden-compatible password manager.", 8082, "vaultwarden:local"),
		a("vikunja", "Vikunja", "tasks", "To-do list and task management application with PostgreSQL.", 3456, "vikunja:local", "postgres:local"),
		a("wallabag", "Wallabag", "read-it-later", "Save articles and read them later.", 8086, "wallabag:local", "postgres:local"),
		a("wordpress", "WordPress", "cms", "Blog and CMS with MariaDB.", 8080, "wordpress:local", "mariadb:local"),
	}
}
