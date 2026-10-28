package httptransport

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

func NewReverseProxy(target string) http.Handler {
	targetURL, err := url.Parse(target)
	if err != nil || targetURL.Host == "" || (targetURL.Scheme != "http" && targetURL.Scheme != "https") {
		panic("invalid proxy target URL")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 10 * time.Second
	proxy := &httputil.ReverseProxy{
		Transport: transport,
		Rewrite: func(req *httputil.ProxyRequest) {
			info := requestForwarding(req.In)
			req.SetURL(targetURL)
			req.Out.Host = targetURL.Host
			req.Out.Header.Del("Forwarded")
			req.Out.Header.Del("X-Real-IP")
			req.Out.Header.Set("X-Forwarded-For", info.ip)
			req.Out.Header.Set("X-Forwarded-Host", info.host)
			req.Out.Header.Set("X-Forwarded-Proto", info.proto)
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			// Do not log URLs, authorization headers or request bodies.
			log.Print("proxy request failed")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":{"code":"bad_gateway","message":"service unavailable"}}`))
		},
	}
	return proxy
}
