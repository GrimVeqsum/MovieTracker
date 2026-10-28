package httptransport

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestForwardedIPCannotBypassRateLimit(t *testing.T) {
	policy, _ := NewForwardingPolicy(nil)
	limiter := NewIPRateLimiter(1, time.Minute)
	handler := policy.Middleware(limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })))
	for i, want := range []int{204, 429, 429} {
		r := httptest.NewRequest("POST", "/auth/login", nil)
		r.RemoteAddr = "203.0.113.10:12345"
		r.Header.Set("X-Forwarded-For", fmt.Sprintf("198.51.100.%d", i+1))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("request %d: got %d want %d", i, w.Code, want)
		}
	}
}

func TestTrustedCaddySeparatesClientsAndIgnoresForgedPrefix(t *testing.T) {
	policy, err := NewForwardingPolicy([]string{"172.30.253.0/29"})
	if err != nil {
		t.Fatal(err)
	}
	limiter := NewIPRateLimiter(1, time.Minute)
	handler := policy.Middleware(limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })))
	for i, tc := range []struct {
		chain string
		want  int
	}{
		{"198.51.100.1", 204}, {"198.51.100.2", 204},
		{"192.0.2.200, 198.51.100.1", 429}, {"192.0.2.201, 198.51.100.1", 429},
	} {
		r := httptest.NewRequest("POST", "/auth/login", nil)
		r.RemoteAddr = "172.30.253.2:1000"
		r.Header.Set("X-Forwarded-For", tc.chain)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("request %d got %d want %d", i, w.Code, tc.want)
		}
	}
}

func TestProxySanitizesForwardingHeaders(t *testing.T) {
	for _, trusted := range []bool{false, true} {
		t.Run(fmt.Sprint(trusted), func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, "%s|%s|%s|%s|%s", r.Header.Get("X-Forwarded-For"), r.Header.Get("X-Forwarded-Proto"), r.Header.Get("X-Forwarded-Host"), r.Header.Get("Forwarded"), r.Header.Get("X-Real-IP"))
			}))
			defer upstream.Close()
			cidrs := []string(nil)
			if trusted {
				cidrs = []string{"10.0.0.1/32"}
			}
			policy, _ := NewForwardingPolicy(cidrs)
			r := httptest.NewRequest("GET", "http://movies.example/movies", nil)
			r.RemoteAddr = "10.0.0.1:5000"
			r.Header.Set("X-Forwarded-For", "192.0.2.200, 198.51.100.1")
			r.Header.Set("X-Forwarded-Proto", "https")
			r.Header.Set("X-Forwarded-Host", "evil.example")
			r.Header.Set("Forwarded", "for=evil")
			r.Header.Set("X-Real-IP", "192.0.2.200")
			w := httptest.NewRecorder()
			policy.Middleware(NewReverseProxy(upstream.URL)).ServeHTTP(w, r)
			want := "10.0.0.1|http|movies.example||"
			if trusted {
				want = "198.51.100.1|https|movies.example||"
			}
			body, _ := io.ReadAll(w.Result().Body)
			if w.Code != 200 || string(body) != want {
				t.Fatalf("got %d %s want %s", w.Code, body, want)
			}
		})
	}
}

func TestForwardingPolicyRejectsInvalidConfiguration(t *testing.T) {
	if _, err := NewForwardingPolicy([]string{"trust-everyone"}); err == nil {
		t.Fatal("expected invalid CIDR error")
	}
}

func TestMalformedForwardingFallsBackToPeer(t *testing.T) {
	policy, _ := NewForwardingPolicy([]string{"10.0.0.1/32"})
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:20"
	r.Header.Set("X-Forwarded-For", "bad, 198.51.100.1")
	r.Header.Set("X-Forwarded-Proto", "https,http")
	info := policy.resolve(r)
	if info.ip != "10.0.0.1" || info.proto != "http" {
		t.Fatalf("unsafe forwarding: %+v", info)
	}
}
