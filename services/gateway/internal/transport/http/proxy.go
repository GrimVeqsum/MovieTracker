package httptransport

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func NewReverseProxy(
	target string,
) http.Handler {
	targetURL, err :=
		url.Parse(
			target,
		)

	if err != nil {
		log.Fatalf(
			"invalid proxy target url %s: %v",
			target,
			err,
		)
	}

	proxy :=
		httputil.NewSingleHostReverseProxy(
			targetURL,
		)

	originalDirector :=
		proxy.Director

	proxy.Director =
		func(
			req *http.Request,
		) {
			externalHost :=
				req.Host

			forwardedProto :=
				strings.TrimSpace(
					req.Header.Get(
						"X-Forwarded-Proto",
					),
				)

			if forwardedProto == "" {
				if req.TLS != nil {
					forwardedProto =
						"https"
				} else {
					forwardedProto =
						"http"
				}
			}

			originalDirector(
				req,
			)

			req.Host =
				targetURL.Host

			req.Header.Set(
				"X-Forwarded-Host",
				externalHost,
			)

			req.Header.Set(
				"X-Forwarded-Proto",
				forwardedProto,
			)
		}

	proxy.ErrorHandler =
		func(
			w http.ResponseWriter,
			r *http.Request,
			err error,
		) {
			log.Printf(
				"proxy error: %v",
				err,
			)

			w.Header().Set(
				"Content-Type",
				"application/json",
			)

			w.WriteHeader(
				http.StatusBadGateway,
			)

			_, _ =
				w.Write(
					[]byte(
						`{"error":{"code":"bad_gateway","message":"service unavailable"}}`,
					),
				)
		}

	return proxy
}
