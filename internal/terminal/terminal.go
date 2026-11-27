// Package terminal runs interactive shells in containers ("dockan exec")
// behind a pseudo-terminal, with output buffered for polling clients.
package terminal

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	maxBuffer = 1 << 20 // output kept per session
	maxInput  = 8192
	idleLimit = 30 * time.Minute
)

// Session is one live shell.
type Session struct {
	ID    string
	Owner string
	pty   *os.File
	cmd   *exec.Cmd

	mu     sync.Mutex
	cond   *sync.Cond
	buf    []byte
	start  int64 // absolute offset of buf[0]
	alive  bool
	seen   time.Time
	exited chan struct{}
}

// Manager keeps the live sessions.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

// NewManager returns an empty manager and starts the idle reaper.
func NewManager() *Manager {
	m := &Manager{sessions: map[string]*Session{}}
	go m.reap()
	return m
}

// Start runs argv under a new pseudo-terminal.
func (m *Manager) Start(id, owner string, env []string, argv ...string) (*Session, error) {
	if err := m.checkLimits(owner); err != nil {
		return nil, err
	}
	ptmx, tty, err := openPTY()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = append(env, "TERM=xterm-256color")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = tty, tty, tty
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		ptmx.Close()
		tty.Close()
		return nil, fmt.Errorf("start shell: %w", err)
	}
	tty.Close()
	s := &Session{ID: id, Owner: owner, pty: ptmx, cmd: cmd, alive: true, seen: time.Now(), exited: make(chan struct{})}
	s.cond = sync.NewCond(&s.mu)
	go s.pump()
	go func() {
		_ = cmd.Wait() // exit status is shown through the terminal output
		s.mu.Lock()
		s.alive = false
		s.cond.Broadcast()
		s.mu.Unlock()
		close(s.exited)
	}()
	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()
	return s, nil
}

// Get returns a session owned by owner.
func (m *Manager) Get(id, owner string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok || s.Owner != owner {
		return nil, errors.New("terminal session not found")
	}
	return s, nil
}

// Stop ends a session.
func (m *Manager) Stop(id, owner string) error {
	s, err := m.Get(id, owner)
	if err != nil {
		return err
	}
	s.kill()
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
	return nil
}

// Limits of open shells, per account and in total.
const (
	MaxPerOwner = 8
	MaxTotal    = 64
)

// ErrTooManyShells is returned when the shell limits are reached.
var ErrTooManyShells = errors.New("too many open terminals: close one first")

func (m *Manager) checkLimits(owner string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	mine := 0
	for _, s := range m.sessions {
		if s.Owner == owner {
			mine++
		}
	}
	if mine >= MaxPerOwner || len(m.sessions) >= MaxTotal {
		return ErrTooManyShells
	}
	return nil
}

// StopUser kills every shell of an account (account deleted).
func (m *Manager) StopUser(owner string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.sessions {
		if s.Owner == owner {
			s.kill()
			delete(m.sessions, id)
		}
	}
}

func (m *Manager) reap() {
	for range time.Tick(time.Minute) {
		m.mu.Lock()
		for id, s := range m.sessions {
			s.mu.Lock()
			idle := time.Since(s.seen) > idleLimit
			dead := !s.alive && time.Since(s.seen) > time.Minute
			s.mu.Unlock()
			if idle || dead {
				s.kill()
				delete(m.sessions, id)
			}
		}
		m.mu.Unlock()
	}
}

func (s *Session) pump() {
	chunk := make([]byte, 32*1024)
	for {
		n, err := s.pty.Read(chunk)
		if n > 0 {
			s.mu.Lock()
			s.buf = append(s.buf, chunk[:n]...)
			if over := len(s.buf) - maxBuffer; over > 0 {
				s.buf = s.buf[over:]
				s.start += int64(over)
			}
			s.cond.Broadcast()
			s.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

func (s *Session) kill() {
	if s.cmd.Process != nil {
		_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGHUP) // session leader's group
		_ = s.cmd.Process.Kill()
	}
	_ = s.pty.Close()
}

// Read returns output after offset, waiting up to wait for new data. It
// returns the data, the next offset and whether the shell still runs.
func (s *Session) Read(offset int64, wait time.Duration) ([]byte, int64, bool) {
	deadline := time.Now().Add(wait)
	timer := time.AfterFunc(wait, func() {
		s.mu.Lock()
		s.cond.Broadcast()
		s.mu.Unlock()
	})
	defer timer.Stop()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seen = time.Now()
	end := s.start + int64(len(s.buf))
	for offset >= end && s.alive && time.Now().Before(deadline) {
		s.cond.Wait()
		end = s.start + int64(len(s.buf))
	}
	if offset < s.start || offset > end {
		offset = s.start
	}
	out := append([]byte{}, s.buf[offset-s.start:]...)
	return out, end, s.alive
}

// Write sends keystrokes to the shell.
func (s *Session) Write(data []byte) error {
	if len(data) > maxInput {
		return errors.New("terminal input is too large")
	}
	s.mu.Lock()
	alive := s.alive
	s.seen = time.Now()
	s.mu.Unlock()
	if !alive {
		return errors.New("the shell has exited")
	}
	_, err := s.pty.Write(data)
	return err
}

// Resize sets the terminal size.
func (s *Session) Resize(cols, rows int) error {
	if cols < 2 || rows < 2 || cols > 1000 || rows > 1000 {
		return errors.New("invalid terminal size")
	}
	ws := struct{ Row, Col, X, Y uint16 }{uint16(rows), uint16(cols), 0, 0}
	return ioctl(s.pty.Fd(), syscall.TIOCSWINSZ, uintptr(unsafe.Pointer(&ws)))
}

func openPTY() (*os.File, *os.File, error) {
	ptmx, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("open /dev/ptmx: %w", err)
	}
	var unlock int32
	if err := ioctl(ptmx.Fd(), syscall.TIOCSPTLCK, uintptr(unsafe.Pointer(&unlock))); err != nil {
		ptmx.Close()
		return nil, nil, fmt.Errorf("unlock pty: %w", err)
	}
	var n uint32
	if err := ioctl(ptmx.Fd(), syscall.TIOCGPTN, uintptr(unsafe.Pointer(&n))); err != nil {
		ptmx.Close()
		return nil, nil, fmt.Errorf("pty number: %w", err)
	}
	tty, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		ptmx.Close()
		return nil, nil, fmt.Errorf("open pty: %w", err)
	}
	return ptmx, tty, nil
}

func ioctl(fd, req, arg uintptr) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, arg); errno != 0 {
		return errno
	}
	return nil
}
