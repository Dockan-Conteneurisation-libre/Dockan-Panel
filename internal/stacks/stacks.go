// Package stacks stores compose projects edited in the panel, in the same
// layout as the PHP panel: stacks/<name>/{dockan.yml,required-images.txt,
// registry-dir.txt}.
package stacks

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const maxYAML = 512 * 1024

var (
	nameRe  = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)
	imageRe = regexp.MustCompile(`(?m)^\s*image:\s*["']?([^"'\s#]+)["']?`)
	refRe   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/:@-]*$`)
)

// Stack is a saved project.
type Stack struct {
	Name           string   `json:"name"`
	File           string   `json:"file"`
	YAML           string   `json:"yaml"`
	RequiredImages []string `json:"requiredImages"`
	RegistryDir    string   `json:"registryDir"`
}

// Store manages stacks under dir.
type Store struct{ Dir string }

// CleanName validates a stack name.
func CleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !nameRe.MatchString(name) || name == "." || name == ".." {
		return "", errors.New("invalid stack name: use letters, digits, dot, dash or underscore")
	}
	return name, nil
}

func (s Store) dir(name string) string { return filepath.Join(s.Dir, name) }

// File is the compose file of a stack.
func (s Store) File(name string) string { return filepath.Join(s.dir(name), "dockan.yml") }

// Names lists saved stacks.
func (s Store) Names() []string {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return []string{}
	}
	names := []string{}
	for _, e := range entries {
		if _, err := CleanName(e.Name()); err != nil || !e.IsDir() {
			continue
		}
		if _, err := os.Stat(s.File(e.Name())); err == nil {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// Load reads one stack.
func (s Store) Load(name string) (Stack, error) {
	name, err := CleanName(name)
	if err != nil {
		return Stack{}, err
	}
	raw, err := os.ReadFile(s.File(name))
	if err != nil {
		return Stack{}, errors.New("stack not found")
	}
	st := Stack{Name: name, File: s.File(name), YAML: string(raw)}
	if req, err := os.ReadFile(filepath.Join(s.dir(name), "required-images.txt")); err == nil {
		st.RequiredImages, _ = ParseImages(string(req))
	}
	if len(st.RequiredImages) == 0 {
		st.RequiredImages = DetectImages(st.YAML)
	}
	if reg, err := os.ReadFile(filepath.Join(s.dir(name), "registry-dir.txt")); err == nil {
		st.RegistryDir = strings.TrimSpace(string(reg))
	}
	return st, nil
}

// Save writes a stack.
func (s Store) Save(st Stack) error {
	name, err := CleanName(st.Name)
	if err != nil {
		return err
	}
	yaml := strings.TrimSpace(strings.ReplaceAll(st.YAML, "\r\n", "\n"))
	if yaml == "" {
		return errors.New("the stack file is empty")
	}
	if len(yaml) > maxYAML {
		return errors.New("the stack file is too large")
	}
	if reg := strings.TrimSpace(st.RegistryDir); strings.ContainsAny(reg, "\n\r\x00") || (reg != "" && !filepath.IsAbs(reg)) {
		return errors.New("invalid registry folder: use an absolute path")
	}
	if len(st.RequiredImages) == 0 {
		st.RequiredImages = DetectImages(yaml)
	}
	if err := os.MkdirAll(s.dir(name), 0o755); err != nil {
		return fmt.Errorf("stack folder: %w", err)
	}
	files := map[string]string{
		"dockan.yml":          yaml + "\n",
		"required-images.txt": strings.Join(st.RequiredImages, "\n") + "\n",
		"registry-dir.txt":    strings.TrimSpace(st.RegistryDir) + "\n",
	}
	for f, content := range files {
		if err := os.WriteFile(filepath.Join(s.dir(name), f), []byte(content), 0o644); err != nil {
			return fmt.Errorf("save %s: %w", f, err)
		}
	}
	return nil
}

// Delete removes a stack.
func (s Store) Delete(name string) error {
	name, err := CleanName(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(s.File(name)); err != nil {
		return errors.New("stack not found")
	}
	for _, f := range []string{"dockan.yml", "required-images.txt", "registry-dir.txt"} {
		if err := os.Remove(filepath.Join(s.dir(name), f)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("delete stack: %w", err)
		}
	}
	return os.Remove(s.dir(name))
}

// DetectImages lists the "image:" references of a compose file.
func DetectImages(yaml string) []string {
	var refs []string
	for _, m := range imageRe.FindAllStringSubmatch(yaml, -1) {
		refs = append(refs, m[1])
	}
	out, _ := ParseImages(strings.Join(refs, "\n"))
	return out
}

// ParseImages reads an image list separated by spaces, commas or lines.
func ParseImages(text string) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	for _, f := range strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\r' || r == '\t' }) {
		if strings.HasPrefix(f, "#") || seen[f] {
			continue
		}
		if !refRe.MatchString(f) {
			return nil, fmt.Errorf("invalid image reference: %s", f)
		}
		seen[f] = true
		out = append(out, f)
	}
	return out, nil
}

// DefaultYAML is the template of a new stack.
const DefaultYAML = `name: my-stack
services:
  web:
    image: myapp:latest
    ports:
      - 8080:8080
    env:
      - PORT=8080
    restart: always
    healthcheck: CMD-SHELL curl -f http://127.0.0.1:8080/`
