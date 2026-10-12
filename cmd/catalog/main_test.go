package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSIGHUPReloadsRunningProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and starts the service binary")
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "catalog")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build service: %v\n%s", err, output)
	}
	configPath := filepath.Join(dir, "config.json")
	writeConfig := func(contents string) {
		t.Helper()
		if err := os.WriteFile(configPath, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeConfig(`{"catalog_label":"before"}`)
	httpAddr := unusedAddress(t)
	grpcAddr := unusedAddress(t)
	logPath := filepath.Join(dir, "service.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	cmd := exec.Command(binary, "-config", configPath, "-http-addr", httpAddr, "-grpc-addr", grpcAddr)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		_ = cmd.Wait()
	}()

	url := "http://" + httpAddr + "/books"
	waitForLabel(t, url, "before")
	pid := cmd.Process.Pid
	writeConfig(`{"catalog_label":"after"}`)
	if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	waitForLabel(t, url, "after")
	writeConfig(`{"catalog_label":""}`)
	if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	waitForLog(t, logPath, "config reload failed")
	waitForLabel(t, url, "after")
	if cmd.Process.Pid != pid {
		t.Fatal("service process changed during reload")
	}
}

func unusedAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()
	return addr
}

func waitForLabel(t *testing.T, url, want string) {
	t.Helper()
	client := http.Client{Timeout: 200 * time.Millisecond}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get(url)
		if err == nil {
			var body struct {
				CatalogLabel string `json:"catalog_label"`
			}
			decodeErr := json.NewDecoder(response.Body).Decode(&body)
			response.Body.Close()
			if decodeErr == nil && response.StatusCode == http.StatusOK && body.CatalogLabel == want {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("HTTP response did not reach label %q at %s", want, url)
}

func waitForLog(t *testing.T, path, phrase string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(content), phrase) {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal(fmt.Errorf("log did not contain %q: %s", phrase, content))
		case <-time.After(20 * time.Millisecond):
		}
	}
}
