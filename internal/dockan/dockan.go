// Package dockan reads Dockan's state (containers, images, volumes,
// networks) by running the dockan CLI and parsing its tables.
package dockan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"dockan-panel/internal/runner"
)

// Container is one row of "dockan ps".
type Container struct {
	Store  string `json:"store"`
	Name   string `json:"name"`
	Status string `json:"status"`
	PID    string `json:"pid"`
	Image  string `json:"image"`
	Ports  string `json:"ports"`
}

// Image is one row of "dockan images".
type Image struct {
	Tag  string `json:"tag"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// Volume is one row of "dockan volume ls".
type Volume struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Network is one row of "dockan network ls".
type Network struct {
	Name   string `json:"name"`
	Driver string `json:"driver"`
	Subnet string `json:"subnet"`
	Bridge string `json:"bridge"`
}

var (
	nameRe      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,95}$`)
	statusWords = `running|exited|stopped|created|restarting|paused`
	psRe        = regexp.MustCompile(`^(.+?)\s+(` + statusWords + `)\s+(\d+)\s+(\S+)(?:\s+(.*))?$`)
	psScopedRe  = regexp.MustCompile(`^(\S+)\s+(.+?)\s+(` + statusWords + `)\s+(\d+)\s+(\S+)(?:\s+(.*))?$`)
	colSplit    = regexp.MustCompile(`\s{2,}`)
)

// ValidName reports whether s is a valid container or volume name.
func ValidName(s string) bool { return nameRe.MatchString(s) }

// CleanName returns an error for an invalid resource name.
func CleanName(s, label string) (string, error) {
	s = strings.TrimSpace(s)
	if !ValidName(s) {
		return "", fmt.Errorf("invalid %s", label)
	}
	return s, nil
}

// CleanStore keeps only the store scopes "dockan ps --scope all" reports.
func CleanStore(s string) string {
	switch s = strings.TrimSpace(s); s {
	case "current", "system", "user":
		return s
	}
	return ""
}

// StoreHome maps a store scope to the DOCKAN_HOME it designates ("" keeps
// the default store).
func StoreHome(store string) string {
	switch store {
	case "system":
		return "/var/lib/dockan"
	case "user":
		if dh := os.Getenv("XDG_DATA_HOME"); dh != "" {
			return filepath.Join(dh, "dockan")
		}
		home := os.Getenv("HOME")
		if home == "" {
			home = "/tmp"
		}
		return filepath.Join(home, ".local", "share", "dockan")
	}
	return ""
}

// Client wraps a runner for read operations.
type Client struct{ R *runner.Runner }

// Containers lists containers of every store when the CLI supports scopes.
func (c Client) Containers(ctx context.Context) []Container {
	if out := c.R.DockanOrEmpty(ctx, "ps", "-a", "--scope", "all"); out != "" {
		return ParseContainers(out)
	}
	return ParseContainers(c.R.DockanOrEmpty(ctx, "ps", "-a"))
}

// ParseContainers reads "dockan ps" output, with or without a STORE column.
func ParseContainers(text string) []Container {
	lines := nonEmptyLines(text)
	if len(lines) < 2 {
		return []Container{}
	}
	scoped := strings.HasPrefix(strings.TrimSpace(lines[0]), "STORE")
	out := []Container{}
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if scoped {
			if m := psScopedRe.FindStringSubmatch(line); m != nil {
				out = append(out, Container{Store: m[1], Name: m[2], Status: m[3], PID: m[4], Image: m[5], Ports: strings.TrimSpace(m[6])})
			}
			continue
		}
		if m := psRe.FindStringSubmatch(line); m != nil {
			out = append(out, Container{Name: m[1], Status: m[2], PID: m[3], Image: m[4], Ports: strings.TrimSpace(m[5])})
		}
	}
	return out
}

// Table parses a whitespace-aligned table into rows keyed by header.
func Table(text string) []map[string]string {
	lines := nonEmptyLines(text)
	rows := []map[string]string{}
	if len(lines) < 2 {
		return rows
	}
	headers := colSplit.Split(strings.TrimSpace(lines[0]), -1)
	for _, line := range lines[1:] {
		parts := colSplit.Split(strings.TrimSpace(line), len(headers))
		row := map[string]string{}
		for i, h := range headers {
			if i < len(parts) {
				row[h] = strings.TrimSpace(parts[i])
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// Images lists local images.
func (c Client) Images(ctx context.Context) []Image {
	out := []Image{}
	for _, r := range Table(c.R.DockanOrEmpty(ctx, "images")) {
		out = append(out, Image{Tag: r["TAG"], Name: r["NAME"], Path: r["PATH"]})
	}
	return out
}

// Volumes lists named volumes.
func (c Client) Volumes(ctx context.Context) []Volume {
	out := []Volume{}
	for _, r := range Table(c.R.DockanOrEmpty(ctx, "volume", "ls")) {
		out = append(out, Volume{Name: r["NAME"], Path: r["PATH"]})
	}
	return out
}

// Networks lists networks.
func (c Client) Networks(ctx context.Context) []Network {
	out := []Network{}
	for _, r := range Table(c.R.DockanOrEmpty(ctx, "network", "ls")) {
		out = append(out, Network{Name: r["NAME"], Driver: r["DRIVER"], Subnet: r["SUBNET"], Bridge: r["BRIDGE"]})
	}
	return out
}

// Inspect returns the key=value metadata of a container, sorted by key.
func (c Client) Inspect(ctx context.Context, store, name string) ([][2]string, error) {
	out, err := c.R.DockanText(ctx, StoreHome(store), "inspect", name)
	if err != nil {
		return nil, err
	}
	var pairs [][2]string
	for _, line := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && k != "" {
			pairs = append(pairs, [2]string{k, v})
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i][0] < pairs[j][0] })
	return pairs, nil
}

// Logs returns the container log, or the error text.
func (c Client) Logs(ctx context.Context, store, name string) string {
	res, err := c.R.Dockan(ctx, StoreHome(store), "logs", name)
	if err != nil {
		return err.Error()
	}
	return res.Text()
}

func nonEmptyLines(text string) []string {
	var out []string
	for _, l := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, strings.TrimRight(l, " \t"))
		}
	}
	return out
}

var imageRefRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/:@-]{0,254}$`)

// CleanImageRef validates an image reference, including pinned digests
// (name@sha256:...).
func CleanImageRef(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if !imageRefRe.MatchString(ref) || strings.Contains(ref, "..") {
		return "", fmt.Errorf("invalid image reference: %s", ref)
	}
	return ref, nil
}
