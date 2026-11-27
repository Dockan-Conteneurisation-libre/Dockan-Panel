package dockan

import "testing"

func TestParseContainersScoped(t *testing.T) {
	out := `STORE      NAME               STATUS       PID      IMAGE                    PORTS
current    pweb               running      964698   alpine:3.20
user       dockan-panel-web   exited       784352   dockan-panel:latest      9090:9090`
	rows := ParseContainers(out)
	if len(rows) != 2 {
		t.Fatalf("rows = %d", len(rows))
	}
	if rows[0].Store != "current" || rows[0].Name != "pweb" || rows[0].Status != "running" || rows[0].Ports != "" {
		t.Fatalf("row 0 = %+v", rows[0])
	}
	if rows[1].Name != "dockan-panel-web" || rows[1].Ports != "9090:9090" {
		t.Fatalf("row 1 = %+v", rows[1])
	}
}

func TestParseContainersPlain(t *testing.T) {
	out := "NAME               STATUS       PID      IMAGE                    PORTS\nweb  running  12  nginx:alpine  8080:80"
	rows := ParseContainers(out)
	if len(rows) != 1 || rows[0].Store != "" || rows[0].Image != "nginx:alpine" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestTable(t *testing.T) {
	rows := Table("NAME                 DRIVER       SUBNET             BRIDGE\npnet                 host-shared  10.89.0.0/24       dockan-pnet")
	if len(rows) != 1 || rows[0]["SUBNET"] != "10.89.0.0/24" || rows[0]["BRIDGE"] != "dockan-pnet" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestCleanImageRef(t *testing.T) {
	for _, ok := range []string{"alpine", "nginx:alpine", "ghcr.io/org/app:1.0", "alpine@sha256:" + "ab12"} {
		if _, err := CleanImageRef(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "-rf", "a b", "../x", "x;rm"} {
		if _, err := CleanImageRef(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
