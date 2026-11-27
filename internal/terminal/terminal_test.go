package terminal

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestShellRoundTrip(t *testing.T) {
	m := NewManager()
	s, err := m.Start("t1", "admin", os.Environ(), "sh")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Stop("t1", "admin") }()
	if err := s.Resize(100, 30); err != nil {
		t.Fatal(err)
	}
	if err := s.Write([]byte("echo pty-$((40+2)); stty size\n")); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	var off int64
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		data, next, _ := s.Read(off, 500*time.Millisecond)
		out.Write(data)
		off = next
		if strings.Contains(out.String(), "pty-42") && strings.Contains(out.String(), "30 100") {
			break
		}
	}
	if !strings.Contains(out.String(), "pty-42") || !strings.Contains(out.String(), "30 100") {
		t.Fatalf("output = %q", out.String())
	}
	if _, err := m.Get("t1", "intruder"); err == nil {
		t.Fatal("another user can reach the session")
	}
	if err := s.Write([]byte("exit\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-s.exited:
	case <-time.After(3 * time.Second):
		t.Fatal("shell did not exit")
	}
	if _, _, alive := s.Read(off, 10*time.Millisecond); alive {
		t.Fatal("session still reported alive")
	}
}
