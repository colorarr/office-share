package main

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestServerHelperProcess(t *testing.T) {
	if os.Getenv("OFFICE_SHARE_TEST_SERVER") == "1" {
		main()
	}
}

func TestShutdownWaitsForActiveUpload(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires SIGTERM")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()
	data := t.TempDir()
	const password = "Shutdown-Test-Password-2026"
	cmd := exec.Command(os.Args[0], "-test.run=^TestServerHelperProcess$")
	cmd.Env = append(os.Environ(), "OFFICE_SHARE_TEST_SERVER=1", "LISTEN_ADDR="+addr,
		"DATA_DIR="+data, "WEB_DIR="+data, "BASE_URL=http://"+addr, "MYSQL_DSN=", "ADMIN_PASSWORD="+password)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	waited := false
	t.Cleanup(func() {
		if !waited {
			cmd.Process.Kill()
			<-done
		}
	})
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(8 * time.Second)
	for {
		res, err := client.Get("http://" + addr + "/api/health")
		if err == nil {
			res.Body.Close()
			if res.StatusCode == 200 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("test server did not start")
		}
		time.Sleep(20 * time.Millisecond)
	}
	credentials, _ := json.Marshal(map[string]string{"username": "admin", "password": password})
	res, err := client.Post("http://"+addr+"/api/login", "application/json", bytes.NewReader(credentials))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || len(res.Cookies()) == 0 {
		t.Fatalf("login failed: %d", res.StatusCode)
	}
	cookie := res.Cookies()[0]
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(8 * time.Second))
	prefix := "--shutdown-boundary\r\nContent-Disposition: form-data; name=\"file\"; filename=\"slow.txt\"\r\n\r\n"
	content := strings.Repeat("a", 16384)
	suffix := "end\r\n--shutdown-boundary--\r\n"
	length := len(prefix) + len(content) + len(suffix)
	_, err = fmt.Fprintf(conn, "POST /api/upload HTTP/1.1\r\nHost: %s\r\nCookie: %s=%s\r\nContent-Type: multipart/form-data; boundary=shutdown-boundary\r\nContent-Length: %d\r\n\r\n%s%s", addr, cookie.Name, cookie.Value, length, prefix, content)
	if err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for {
		files, err := os.ReadDir(filepath.Join(data, "files"))
		if err == nil && len(files) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("upload handler did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err = cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		waited = true
		t.Fatalf("server exited before active upload completed: %v; output=%s", err, output.String())
	case <-time.After(100 * time.Millisecond):
	}
	if _, err = io.WriteString(conn, suffix); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "POST"})
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if response.StatusCode != 201 {
		t.Fatalf("active upload failed during shutdown: %d", response.StatusCode)
	}
	select {
	case err = <-done:
		waited = true
		if err != nil {
			t.Fatalf("shutdown failed: %v; output=%s", err, output.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not exit after upload completed")
	}
	db, err := sql.Open("sqlite", filepath.Join(data, "office.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var id string
	var size int
	if err = db.QueryRow("SELECT id,size FROM items WHERE kind='file'").Scan(&id, &size); err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(filepath.Join(data, "files", id))
	if err != nil || string(stored) != content+"end" || size != len(stored) {
		t.Fatalf("upload was not durably saved: size=%d, stored=%d, err=%v", size, len(stored), err)
	}
}
