package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dockan-panel/internal/auth"
	"dockan-panel/internal/dockan"
	"dockan-panel/internal/runner"
	"dockan-panel/internal/store"
)

type storeAppView struct {
	store.App
	Target    string `json:"target"`
	Installed bool   `json:"installed"`
	Autostart bool   `json:"autostart"`
	Images    string `json:"images"` // current, available, unknown
	Config    string `json:"config"`
}

func (s *Server) getStore(_ http.ResponseWriter, _ *http.Request, _ *auth.Session) (any, error) {
	_, err := os.Stat(filepath.Join(s.Store.Dir, "dockan-store"))
	ready := err == nil
	apps := []storeAppView{}
	for _, a := range s.Store.Catalog() {
		target := s.Store.Target(a.ID)
		_, err := os.Stat(filepath.Join(target, "dockan.yml"))
		installed := err == nil
		v := storeAppView{App: a, Target: target, Installed: installed, Autostart: store.ServiceEnabled(a.ID),
			Images: "unknown", Config: s.Store.ConfigYAML(a.ID, target)}
		if v.Requires == nil {
			v.Requires = []string{}
		}
		if installed && ready {
			v.Images = s.Store.ImageStatus(a.Requires)
		}
		apps = append(apps, v)
	}
	return map[string]any{
		"ready": ready, "apps": apps, "path": s.Store.Dir, "base": s.Store.DefaultBase(),
		"catalog": s.Store.CatalogSource(), "root": runner.IsRoot(),
	}, nil
}

func (s *Server) postStore(_ http.ResponseWriter, r *http.Request, _ *auth.Session) (any, error) {
	if r.PathValue("action") == "update" {
		return done(s.Store.Update(r.Context()))
	}
	var b struct{ App, Target, Config string }
	if err := decode(r, &b); err != nil {
		return nil, err
	}
	req := store.Request{App: b.App, Target: b.Target, Config: b.Config}
	ctx := r.Context()
	switch r.PathValue("action") {
	case "install":
		return done(s.Store.Install(ctx, req, false))
	case "deploy":
		return done(s.Store.Install(ctx, req, true))
	case "install-autostart":
		return done(s.Store.Autostart(ctx, req, true))
	case "autostart":
		return done(s.Store.Autostart(ctx, req, false))
	case "disable-autostart":
		return done(s.Store.DisableAutostart(ctx, req))
	case "launch":
		return done(s.Store.Launch(ctx, req))
	case "update-images":
		return done(s.Store.UpdateImages(ctx, req, false))
	case "redeploy":
		return done(s.Store.UpdateImages(ctx, req, true))
	case "save-config":
		return done(s.Store.Configure(ctx, req, false))
	case "save-redeploy":
		return done(s.Store.Configure(ctx, req, true))
	}
	return nil, apiError{http.StatusNotFound, "unknown action"}
}

// postStoreUpload receives website files for the Static Site app.
func (s *Server) postStoreUpload(_ http.ResponseWriter, r *http.Request, _ *auth.Session) (any, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, 2<<30)
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		return nil, badRequest("upload failed: the files are too large or the request is invalid")
	}
	defer func() {
		_ = r.MultipartForm.RemoveAll() // temporary upload files
	}()
	if r.FormValue("app") != "static-site" {
		return nil, badRequest("website uploads are only available for the Static Site app")
	}
	target, err := store.CleanTarget(r.FormValue("target"))
	if err != nil {
		return nil, err
	}
	public, publicPath, err := store.OpenSite(target)
	if err != nil {
		return nil, err
	}
	defer public.Close()
	if r.FormValue("replace") == "1" {
		if err := store.EmptyDir(public); err != nil {
			return nil, fmt.Errorf("empty the website folder: %w", err)
		}
	}
	count := 0
	if file, header, err := r.FormFile("zip"); err == nil {
		defer file.Close()
		if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
			return nil, badRequest("the archive must be a .zip file")
		}
		n, err := store.ExtractZip(public, file, header.Size)
		count += n
		if err != nil {
			return nil, err
		}
	}
	if r.MultipartForm != nil {
		n, err := store.SaveFiles(public, r.MultipartForm.File["files"])
		count += n
		if err != nil {
			return nil, err
		}
	}
	if count == 0 {
		return nil, badRequest("choose a .zip file or at least one file")
	}
	return done(fmt.Sprintf("%d file(s) copied to %s", count, publicPath), nil)
}

// Terminal.

func (s *Server) postTerminal(_ http.ResponseWriter, r *http.Request, sess *auth.Session) (any, error) {
	var b struct {
		ID, Name, Store, Data string
		Offset                int64
		Cols, Rows            int
	}
	if err := decode(r, &b); err != nil {
		return nil, err
	}
	if r.PathValue("action") == "start" {
		name, err := dockan.CleanName(b.Name, "container name")
		if err != nil {
			return nil, err
		}
		id, err := auth.RandomToken(16)
		if err != nil {
			return nil, err
		}
		env := s.R.Env()
		if home := dockan.StoreHome(dockan.CleanStore(b.Store)); home != "" {
			env = append(env, "DOCKAN_HOME="+home)
		}
		if _, err := s.Terms.Start(id, sess.User, env, s.R.Bin, "exec", name, "sh", "-li"); err != nil {
			return nil, err
		}
		return map[string]string{"id": id}, nil
	}
	if r.PathValue("action") == "stop" {
		return done("Terminal closed.", s.Terms.Stop(b.ID, sess.User))
	}
	t, err := s.Terms.Get(b.ID, sess.User)
	if err != nil {
		return nil, apiError{http.StatusNotFound, err.Error()}
	}
	switch r.PathValue("action") {
	case "read":
		data, next, alive := t.Read(b.Offset, 20*time.Second)
		return map[string]any{"data": string(data), "offset": next, "alive": alive}, nil
	case "input":
		return done("ok", t.Write([]byte(b.Data)))
	case "resize":
		return done("ok", t.Resize(b.Cols, b.Rows))
	}
	return nil, errors.New("unknown terminal action")
}
