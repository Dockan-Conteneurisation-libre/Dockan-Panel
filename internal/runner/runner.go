// Package runner executes host commands for the panel: dockan itself, shell
// scripts, and privileged commands through sudo when the panel is not root.
package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Result is the outcome of one command.
type Result struct {
	Code   int
	Stdout string
	Stderr string
}

// Text returns stdout and stderr joined and trimmed.
func (r Result) Text() string {
	return strings.TrimSpace(r.Stdout + "\n" + r.Stderr)
}

// Runner runs commands with a PATH that includes the user's ~/.local/bin.
type Runner struct {
	Bin        string
	PathPrefix string
	Timeout    time.Duration
}

// ErrCommand wraps a command that exited with a non-zero status; its message
// is the command output.
type ErrCommand struct{ Output string }

func (e *ErrCommand) Error() string {
	if e.Output == "" {
		return "command failed"
	}
	return e.Output
}

// Run executes argv and returns its result. A non-zero exit is not an error
// here: callers decide with Check.
func (r *Runner) Run(ctx context.Context, env []string, argv ...string) (Result, error) {
	if len(argv) == 0 {
		return Result{}, errors.New("empty command")
	}
	timeout := r.Timeout
	if timeout == 0 {
		timeout = 30 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = append(r.Env(), env...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	res := Result{Stdout: out.String(), Stderr: errb.String()}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		res.Code = exitErr.ExitCode()
	default:
		return res, fmt.Errorf("%s: %w", argv[0], err)
	}
	return res, nil
}

// Env returns the process environment with ~/.local/bin first in PATH.
func (r *Runner) Env() []string {
	env := os.Environ()
	if r.PathPrefix == "" {
		return env
	}
	for i, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			env[i] = "PATH=" + r.PathPrefix + ":" + strings.TrimPrefix(kv, "PATH=")
			return env
		}
	}
	return append(env, "PATH="+r.PathPrefix+":/usr/local/bin:/usr/bin:/bin")
}

// SudoPath is the PATH handed to privileged commands.
func (r *Runner) SudoPath() string {
	path := os.Getenv("PATH")
	if path == "" {
		path = "/usr/local/bin:/usr/bin:/bin"
	}
	if r.PathPrefix != "" {
		return r.PathPrefix + ":" + path
	}
	return path
}

// Check turns a failed result into an error carrying its output.
func Check(res Result, err error) (string, error) {
	if err != nil {
		return "", err
	}
	if res.Code != 0 {
		return "", &ErrCommand{Output: res.Text()}
	}
	return res.Text(), nil
}

// Dockan runs the dockan CLI, optionally against a given store home.
func (r *Runner) Dockan(ctx context.Context, home string, args ...string) (Result, error) {
	var env []string
	if home != "" {
		env = append(env, "DOCKAN_HOME="+home)
	}
	return r.Run(ctx, env, append([]string{r.Bin}, args...)...)
}

// DockanText runs dockan and returns its output, or an error on failure.
func (r *Runner) DockanText(ctx context.Context, home string, args ...string) (string, error) {
	return Check(r.Dockan(ctx, home, args...))
}

// DockanOrEmpty returns the output, or "" when the command fails.
func (r *Runner) DockanOrEmpty(ctx context.Context, args ...string) string {
	out, err := r.DockanText(ctx, "", args...)
	if err != nil {
		return ""
	}
	return out
}

// Shell runs a script with sh -lc.
func (r *Runner) Shell(ctx context.Context, script string) (Result, error) {
	return r.Run(ctx, nil, "sh", "-lc", script)
}

// IsRoot reports whether the panel runs as root.
func IsRoot() bool { return os.Geteuid() == 0 }

// SystemShell runs a script as root: directly when the panel is root, else
// through "sudo -n", which fails instead of prompting for a password.
func (r *Runner) SystemShell(ctx context.Context, script string) (Result, error) {
	if IsRoot() {
		return r.Shell(ctx, script)
	}
	return r.Run(ctx, nil, "sudo", "-n", "sh", "-lc", script)
}

// SystemDockan runs dockan as root (directly or through sudo -n).
func (r *Runner) SystemDockan(ctx context.Context, args ...string) (Result, error) {
	if IsRoot() {
		return r.Dockan(ctx, "", args...)
	}
	argv := append([]string{"sudo", "-n", "env", "PATH=" + r.SudoPath(), r.Bin}, args...)
	return r.Run(ctx, nil, argv...)
}

// ErrNoSudo explains why a privileged action cannot run.
var ErrNoSudo = errors.New("system actions are disabled: the panel is not running as root and sudo needs a password. " +
	"Run the production panel as a system service, or allow passwordless sudo for Dockan")

// SystemText checks a privileged result and recognises a sudo password prompt.
func SystemText(res Result, err error) (string, error) {
	if err != nil {
		return "", err
	}
	text := res.Text()
	if res.Code != 0 {
		low := strings.ToLower(text)
		if strings.Contains(low, "sudo:") && (strings.Contains(low, "password") || strings.Contains(low, "mot de passe") || strings.Contains(low, "terminal")) {
			return "", ErrNoSudo
		}
		return "", &ErrCommand{Output: text}
	}
	if text == "" {
		text = "Command completed."
	}
	return text, nil
}

// Quote returns argv as one shell command line, each argument single-quoted.
func Quote(argv ...string) string {
	parts := make([]string, len(argv))
	for i, a := range argv {
		parts[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
	}
	return strings.Join(parts, " ")
}

// SystemCommandLine is the command a person would type for a privileged
// dockan action ("Show command" buttons).
func (r *Runner) SystemCommandLine(args ...string) string {
	if IsRoot() {
		return Quote(append([]string{r.Bin}, args...)...)
	}
	return `sudo env "PATH=$HOME/.local/bin:$PATH" ` + Quote(append([]string{r.Bin}, args...)...)
}

// SystemScriptLine is the command line for a privileged script.
func SystemScriptLine(script string) string {
	if IsRoot() {
		return Quote("sh", "-lc", script)
	}
	return "sudo " + Quote("sh", "-lc", script)
}
