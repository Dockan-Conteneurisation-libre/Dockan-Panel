package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"dockan-panel/internal/auth"
	"dockan-panel/internal/dockan"
	"dockan-panel/internal/runner"
	"dockan-panel/internal/stacks"
	"dockan-panel/internal/store"
	"dockan-panel/internal/system"
)

func (s *Server) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/session", s.api(public, s.getSession))
	mux.HandleFunc("POST /api/setup", s.api(public, s.postSetup))
	mux.HandleFunc("POST /api/login", s.api(public, s.postLogin))
	mux.HandleFunc("POST /api/logout", s.api(public, s.postLogout))
	mux.HandleFunc("POST /api/passkeys/login-options", s.api(public, s.passkeyLoginOptions))
	mux.HandleFunc("POST /api/passkeys/login-verify", s.api(public, s.passkeyLoginVerify))
	mux.HandleFunc("POST /api/passkeys/register-options", s.api(signed, s.passkeyRegisterOptions))
	mux.HandleFunc("POST /api/passkeys/register-verify", s.api(signed, s.passkeyRegisterVerify))
	mux.HandleFunc("GET /api/users", s.api(signed, s.getUsers))
	mux.HandleFunc("POST /api/account/{action}", s.api(signed, s.postAccount))

	mux.HandleFunc("GET /api/overview", s.api(signed, s.getOverview))
	mux.HandleFunc("GET /api/containers", s.api(signed, s.getContainers))
	mux.HandleFunc("GET /api/containers/detail", s.api(signed, s.getContainer))
	mux.HandleFunc("POST /api/containers/action", s.api(signed, s.postContainerAction))
	mux.HandleFunc("POST /api/containers/run", s.api(signed, s.postRun))
	mux.HandleFunc("POST /api/containers/exec", s.api(signed, s.postExec))
	mux.HandleFunc("GET /api/images", s.api(signed, s.getImages))
	mux.HandleFunc("POST /api/images/{action}", s.api(signed, s.postImage))
	mux.HandleFunc("GET /api/volumes", s.api(signed, s.getVolumes))
	mux.HandleFunc("POST /api/volumes/{action}", s.api(signed, s.postVolume))
	mux.HandleFunc("GET /api/networks", s.api(signed, s.getNetworks))
	mux.HandleFunc("GET /api/stacks", s.api(signed, s.getStacks))
	mux.HandleFunc("POST /api/stacks/{action}", s.api(signed, s.postStack))
	mux.HandleFunc("POST /api/compose", s.api(signed, s.postCompose))
	mux.HandleFunc("GET /api/store", s.api(signed, s.getStore))
	mux.HandleFunc("POST /api/store/{action}", s.api(signed, s.postStore))
	mux.HandleFunc("POST /api/store-upload", s.api(signed, s.postStoreUpload))
	mux.HandleFunc("GET /api/system", s.api(signed, s.getSystem))
	mux.HandleFunc("POST /api/system/{action}", s.api(signed, s.postSystem))
	mux.HandleFunc("POST /api/terminal/{action}", s.api(signed, s.postTerminal))
}

func (s *Server) getOverview(_ http.ResponseWriter, r *http.Request, _ *auth.Session) (any, error) {
	ctx := r.Context()
	cs := s.D.Containers(ctx)
	running := 0
	for _, c := range cs {
		if c.Status == "running" {
			running++
		}
	}
	return map[string]any{
		"containers": len(cs), "running": running,
		"images": len(s.D.Images(ctx)), "volumes": len(s.D.Volumes(ctx)), "networks": len(s.D.Networks(ctx)),
		"stacks": len(s.Stacks.Names()), "recent": cs,
		"doctor": s.R.DockanOrEmpty(ctx, "doctor"), "dockanVersion": s.R.DockanOrEmpty(ctx, "version"),
	}, nil
}

type storeAppRow struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	Containers []string `json:"containers"`
	File       string   `json:"file"`
	Target     string   `json:"target"`
}

func (s *Server) getContainers(_ http.ResponseWriter, r *http.Request, _ *auth.Session) (any, error) {
	cs := s.D.Containers(r.Context())
	byName := map[string]dockan.Container{}
	for _, c := range cs {
		byName[c.Name] = c
	}
	apps := []storeAppRow{}
	for _, a := range s.Store.Catalog() {
		target := s.Store.Target(a.ID)
		file := filepath.Join(target, "dockan.yml")
		if _, err := os.Stat(file); err != nil {
			continue
		}
		row := storeAppRow{ID: a.ID, Name: a.Name, Status: "files", Containers: []string{}, File: file, Target: target}
		for _, name := range store.ComposeContainers(file, a.ID) {
			if c, ok := byName[name]; ok {
				row.Containers = append(row.Containers, name)
				if row.Status == "files" {
					row.Status = c.Status
				}
			}
		}
		apps = append(apps, row)
	}
	return map[string]any{"containers": cs, "apps": apps, "images": s.D.Images(r.Context())}, nil
}

func (s *Server) getContainer(_ http.ResponseWriter, r *http.Request, _ *auth.Session) (any, error) {
	name, err := dockan.CleanName(r.URL.Query().Get("name"), "container name")
	if err != nil {
		return nil, err
	}
	store := dockan.CleanStore(r.URL.Query().Get("store"))
	var cur *dockan.Container
	for _, c := range s.D.Containers(r.Context()) {
		if c.Name == name && (store == "" || c.Store == store) {
			c := c
			cur = &c
			break
		}
	}
	if cur == nil {
		return nil, apiError{http.StatusNotFound, "container not found: " + name}
	}
	inspect, err := s.D.Inspect(r.Context(), cur.Store, name)
	if err != nil {
		inspect = [][2]string{}
	}
	return map[string]any{
		"container": cur, "inspect": inspect, "logs": tail(s.D.Logs(r.Context(), cur.Store, name), 2000),
		"composeFile": s.composeFileFor(name),
	}, nil
}

func tail(text string, lines int) string {
	parts := strings.Split(text, "\n")
	if len(parts) > lines {
		parts = parts[len(parts)-lines:]
	}
	return strings.Join(parts, "\n")
}

// composeFileFor finds the dockan.yml of an app from a container name
// (<app>-<service>), as the PHP panel did.
func (s *Server) composeFileFor(name string) string {
	parts := strings.Split(name, "-")
	var ids []string
	for i := len(parts) - 1; i >= 1; i-- {
		ids = append(ids, strings.Join(parts[:i], "-"))
	}
	ids = append(ids, name)
	for _, id := range ids {
		for _, f := range []string{
			filepath.Join("/srv/dockan-apps", id, "dockan.yml"),
			filepath.Join("/srv", id, "dockan.yml"),
			filepath.Join(s.Store.Target(id), "dockan.yml"),
		} {
			if st, err := os.Stat(f); err == nil && !st.IsDir() {
				return f
			}
		}
	}
	return ""
}

type containerReq struct {
	Name   string `json:"name"`
	Store  string `json:"store"`
	Action string `json:"action"`
}

func (s *Server) postContainerAction(_ http.ResponseWriter, r *http.Request, _ *auth.Session) (any, error) {
	var b containerReq
	if err := decode(r, &b); err != nil {
		return nil, err
	}
	name, err := dockan.CleanName(b.Name, "container name")
	if err != nil {
		return nil, err
	}
	home := dockan.StoreHome(dockan.CleanStore(b.Store))
	ctx := r.Context()
	switch b.Action {
	case "stop", "health":
		return done(s.R.DockanText(ctx, home, b.Action, name))
	case "remove":
		return done(s.R.DockanText(ctx, home, "rm", name))
	case "start-app", "restart-app":
		file := s.composeFileFor(name)
		if file == "" {
			return nil, badRequest("no dockan.yml found for this container")
		}
		verb := map[string]string{"start-app": "up", "restart-app": "redeploy"}[b.Action]
		return done(runner.Check(s.R.Run(ctx, []string{"DOCKAN_PORT_BIND_ADDR=0.0.0.0"}, s.R.Bin, "compose", verb, "-f", file)))
	}
	return nil, badRequest("unknown container action")
}

type runReq struct {
	Name, Image, Ports, Env, Volumes, Aliases, Network string
	Entrypoint, Restart, Healthcheck, Memory, CPUs     string
	Isolation, Command                                 string
	GUI                                                bool
}

var isolations = map[string]bool{"": true, "auto": true, "none": true, "bubblewrap": true, "firejail": true, "systemd-nspawn": true, "chroot": true}

func (s *Server) postRun(_ http.ResponseWriter, r *http.Request, _ *auth.Session) (any, error) {
	var b runReq
	if err := decode(r, &b); err != nil {
		return nil, err
	}
	name, err := dockan.CleanName(b.Name, "container name")
	if err != nil {
		return nil, err
	}
	image, err := dockan.CleanImageRef(b.Image)
	if err != nil {
		return nil, err
	}
	if !isolations[b.Isolation] {
		return nil, badRequest("unknown isolation mode")
	}
	args := []string{"run", "-d", "--name", name}
	for _, opt := range []struct{ flag, text string }{
		{"-p", b.Ports}, {"-e", b.Env}, {"-v", b.Volumes}, {"--alias", b.Aliases},
	} {
		for _, v := range lines(opt.text) {
			if strings.HasPrefix(v, "-") {
				return nil, badRequest("invalid value for " + opt.flag + ": " + v)
			}
			args = append(args, opt.flag, v)
		}
	}
	for flag, v := range map[string]string{"--network": b.Network, "--entrypoint": b.Entrypoint, "--restart": b.Restart,
		"--healthcheck": b.Healthcheck, "--memory": b.Memory, "--cpus": b.CPUs} {
		if v = strings.TrimSpace(v); v != "" {
			if strings.HasPrefix(v, "-") {
				return nil, badRequest("invalid value for " + flag)
			}
			args = append(args, flag, v)
		}
	}
	if b.GUI {
		args = append(args, "--gui")
	}
	if b.Isolation != "" && b.Isolation != "auto" {
		args = append(args, "--isolation="+b.Isolation)
	}
	args = append(args, image)
	cmd, err := splitCommand(b.Command)
	if err != nil {
		return nil, err
	}
	args = append(args, cmd...)
	return done(s.R.DockanText(r.Context(), "", args...))
}

func lines(text string) []string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "#") {
			out = append(out, l)
		}
	}
	return out
}

// splitCommand splits a command line on spaces, honouring double quotes and
// backslash escapes (the PHP panel used str_getcsv the same way).
func splitCommand(s string) ([]string, error) {
	var out []string
	var cur strings.Builder
	inQuote, escaped, has := false, false, false
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped, has = true, true
		case r == '"':
			inQuote, has = !inQuote, true
		case r == ' ' && !inQuote:
			if has {
				out = append(out, cur.String())
				cur.Reset()
				has = false
			}
		default:
			cur.WriteRune(r)
			has = true
		}
	}
	if inQuote {
		return nil, errors.New("unclosed quote in the command")
	}
	if has {
		out = append(out, cur.String())
	}
	return out, nil
}

func (s *Server) postExec(_ http.ResponseWriter, r *http.Request, _ *auth.Session) (any, error) {
	var b struct{ Name, Store, Command string }
	if err := decode(r, &b); err != nil {
		return nil, err
	}
	name, err := dockan.CleanName(b.Name, "container name")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(b.Command) == "" || len(b.Command) > 8000 {
		return nil, badRequest("the command is empty or too long")
	}
	return done(s.R.DockanText(r.Context(), dockan.StoreHome(dockan.CleanStore(b.Store)), "exec", name, "sh", "-lc", b.Command))
}

func (s *Server) getImages(_ http.ResponseWriter, r *http.Request, _ *auth.Session) (any, error) {
	return s.D.Images(r.Context()), nil
}

func (s *Server) postImage(_ http.ResponseWriter, r *http.Request, _ *auth.Session) (any, error) {
	var b struct{ Ref string }
	if err := decode(r, &b); err != nil {
		return nil, err
	}
	ref, err := dockan.CleanImageRef(b.Ref)
	if err != nil {
		return nil, err
	}
	switch r.PathValue("action") {
	case "pull":
		return done(s.R.DockanText(r.Context(), "", "pull", ref))
	case "remove":
		return done(s.R.DockanText(r.Context(), "", "rmi", ref))
	}
	return nil, apiError{http.StatusNotFound, "unknown action"}
}

type backupInfo struct {
	File    string `json:"file"`
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Created string `json:"created"`
}

func (s *Server) backups() []backupInfo {
	matches, _ := filepath.Glob(filepath.Join(s.Cfg.BackupDir(), "*.tar.gz"))
	out := []backupInfo{}
	for _, m := range matches {
		if st, err := os.Stat(m); err == nil {
			out = append(out, backupInfo{File: m, Name: filepath.Base(m), Size: st.Size(), Created: st.ModTime().Format(time.RFC3339)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name > out[j].Name })
	return out
}

func (s *Server) getVolumes(_ http.ResponseWriter, r *http.Request, _ *auth.Session) (any, error) {
	return map[string]any{"volumes": s.D.Volumes(r.Context()), "backups": s.backups()}, nil
}

func (s *Server) postVolume(_ http.ResponseWriter, r *http.Request, _ *auth.Session) (any, error) {
	var b struct{ Name, Backup string }
	if err := decode(r, &b); err != nil {
		return nil, err
	}
	name, err := dockan.CleanName(b.Name, "volume name")
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	switch r.PathValue("action") {
	case "create":
		return done(s.R.DockanText(ctx, "", "volume", "create", name))
	case "remove":
		return done(s.R.DockanText(ctx, "", "volume", "rm", name))
	case "backup":
		file := filepath.Join(s.Cfg.BackupDir(), name+"-"+time.Now().Format("20060102-150405")+".tar.gz")
		out, err := s.R.DockanText(ctx, "", "volume", "backup", name, file)
		return done(strings.TrimSpace(out+"\nBackup: "+file), err)
	case "restore":
		file, err := s.backupFile(b.Backup)
		if err != nil {
			return nil, err
		}
		return done(s.R.DockanText(ctx, "", "volume", "restore", name, file))
	}
	return nil, apiError{http.StatusNotFound, "unknown action"}
}

// backupFile accepts only a backup file of the panel's backup folder.
func (s *Server) backupFile(name string) (string, error) {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "." || base == "/" || !strings.HasSuffix(base, ".tar.gz") {
		return "", badRequest("choose a backup")
	}
	dir, err := filepath.EvalSymlinks(s.Cfg.BackupDir())
	if err != nil {
		return "", err
	}
	file, err := filepath.EvalSymlinks(filepath.Join(dir, base))
	if err != nil || filepath.Dir(file) != dir {
		return "", badRequest("the backup is outside the panel backup folder")
	}
	return file, nil
}

func (s *Server) getNetworks(_ http.ResponseWriter, r *http.Request, _ *auth.Session) (any, error) {
	return s.D.Networks(r.Context()), nil
}

func (s *Server) getStacks(_ http.ResponseWriter, _ *http.Request, _ *auth.Session) (any, error) {
	list := []stacks.Stack{}
	for _, n := range s.Stacks.Names() {
		if st, err := s.Stacks.Load(n); err == nil {
			list = append(list, st)
		}
	}
	return map[string]any{"stacks": list, "template": stacks.DefaultYAML}, nil
}

type stackReq struct {
	Name           string `json:"name"`
	YAML           string `json:"yaml"`
	RequiredImages string `json:"requiredImages"`
	RegistryDir    string `json:"registryDir"`
}

func (s *Server) postStack(_ http.ResponseWriter, r *http.Request, _ *auth.Session) (any, error) {
	var b stackReq
	if err := decode(r, &b); err != nil {
		return nil, err
	}
	name, err := stacks.CleanName(b.Name)
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	switch action := r.PathValue("action"); action {
	case "save":
		imgs, err := stacks.ParseImages(b.RequiredImages)
		if err != nil {
			return nil, err
		}
		return done("Stack saved: "+name, s.Stacks.Save(stacks.Stack{Name: name, YAML: b.YAML, RequiredImages: imgs, RegistryDir: b.RegistryDir}))
	case "delete":
		return done("Stack deleted: "+name, s.Stacks.Delete(name))
	case "up", "down", "redeploy", "health":
		if _, err := s.Stacks.Load(name); err != nil {
			return nil, err
		}
		return done(s.R.DockanText(ctx, "", "compose", action, "-f", s.Stacks.File(name)))
	case "import":
		st, err := s.Stacks.Load(name)
		if err != nil {
			return nil, err
		}
		if len(st.RequiredImages) == 0 {
			return nil, badRequest("no required images")
		}
		var out []string
		for _, img := range st.RequiredImages {
			args := []string{"pull", img}
			if st.RegistryDir != "" {
				args = append(args, st.RegistryDir)
			}
			text, err := s.R.DockanText(ctx, "", args...)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", img, err)
			}
			out = append(out, "$ dockan "+strings.Join(args, " "), text)
		}
		return done(strings.Join(out, "\n"), nil)
	}
	return nil, apiError{http.StatusNotFound, "unknown action"}
}

func (s *Server) postCompose(_ http.ResponseWriter, r *http.Request, _ *auth.Session) (any, error) {
	var b struct{ File, Action string }
	if err := decode(r, &b); err != nil {
		return nil, err
	}
	switch b.Action {
	case "up", "down", "redeploy", "health":
	default:
		return nil, badRequest("unknown compose action")
	}
	file := filepath.Clean(strings.TrimSpace(b.File))
	if !filepath.IsAbs(file) {
		return nil, badRequest("use an absolute path to dockan.yml or compose.yaml")
	}
	if st, err := os.Stat(file); err != nil || st.IsDir() {
		return nil, badRequest("compose file not found")
	}
	return done(s.R.DockanText(r.Context(), "", "compose", b.Action, "-f", file))
}

func (s *Server) getSystem(_ http.ResponseWriter, r *http.Request, _ *auth.Session) (any, error) {
	return map[string]any{
		"info": system.Collect(r.Context(), s.R, s.Cfg), "profiles": system.Profiles, "runtimes": system.Runtimes,
	}, nil
}

func (s *Server) postSystem(_ http.ResponseWriter, r *http.Request, _ *auth.Session) (any, error) {
	var b struct {
		Kind, Value, Mode, Version, Ref string
		System                          bool
	}
	if err := decode(r, &b); err != nil {
		return nil, err
	}
	mode := system.Mode(b.Mode)
	switch mode {
	case system.Preview, system.Install, system.Command:
	default:
		return nil, badRequest("unknown mode")
	}
	ctx := r.Context()
	switch r.PathValue("action") {
	case "deps":
		return done(system.Deps(ctx, s.R, b.Kind, b.Value, mode))
	case "update-dockan":
		return done(system.UpdateDockan(ctx, s.R, b.Version, b.System, mode))
	case "update-panel":
		return done(system.UpdatePanel(ctx, s.R, b.Ref, s.Cfg.UpdateDir, mode))
	}
	return nil, apiError{http.StatusNotFound, "unknown action"}
}
