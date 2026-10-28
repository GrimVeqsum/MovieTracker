//go:build integration

package httptransport

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRealCaddyClientIPAndForwarding(t *testing.T) {
	project := os.Getenv("COMPOSE_PROJECT_NAME")
	if !strings.HasPrefix(project, "movietracker-") || os.Getenv("TEST_KAFKA_PORT") == "" {
		t.Fatal("use tests/integration/run.sh with an isolated Compose project")
	}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Forwarded-Proto") != "http" || r.Header.Get("X-Forwarded-Host") == "evil.example" {
			t.Error("forged forwarding reached backend")
		}
		w.WriteHeader(204)
	}))
	defer backend.Close()
	proxy := NewReverseProxy(backend.URL)
	policy, _ := NewForwardingPolicy([]string{"127.0.0.1/32"})
	gateway := httptest.NewServer(policy.Middleware(NewRouter(NewHandler(backend.URL, backend.URL), NewWebHandler("test_bot"), "test_bot", proxy, proxy)))
	defer gateway.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	composeFile, err := filepath.Abs("../../../../../tests/integration/compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("docker", "compose", "-p", project, "-f", composeFile, "up", "-d", "--no-deps", "caddy")
	cmd.Env = append(os.Environ(), "CADDY_TEST_LISTEN="+address, "CADDY_TEST_UPSTREAM="+strings.TrimPrefix(gateway.URL, "http://"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("start isolated Caddy: %v %s", err, out)
	}
	t.Cleanup(func() {
		cmd := exec.Command("docker", "compose", "-p", project, "-f", composeFile, "stop", "caddy")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("stop Caddy: %v %s", err, out)
		}
	})
	deadline := time.Now().Add(20 * time.Second)
	for {
		resp, err := http.Get("http://" + address + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("Caddy did not become ready")
		}
		time.Sleep(100 * time.Millisecond)
	}
	for _, ip := range []string{"127.0.0.2", "127.0.0.3"} {
		transport := &http.Transport{DialContext: (&net.Dialer{LocalAddr: &net.TCPAddr{IP: net.ParseIP(ip)}}).DialContext}
		client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
		defer transport.CloseIdleConnections()
		for n := 0; n < 6; n++ {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			req, _ := http.NewRequestWithContext(ctx, "POST", "http://"+address+"/auth/login", nil)
			req.Header.Set("X-Forwarded-For", fmt.Sprintf("192.0.2.%d", n+1))
			req.Header.Set("X-Forwarded-Proto", "https")
			req.Header.Set("X-Forwarded-Host", "evil.example")
			resp, err := client.Do(req)
			if err != nil {
				cancel()
				t.Fatal(err)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			cancel()
			want := 204
			if n == 5 {
				want = 429
			}
			if resp.StatusCode != want {
				t.Fatalf("real client %s request %d: got %d want %d", ip, n+1, resp.StatusCode, want)
			}
		}
	}
}
