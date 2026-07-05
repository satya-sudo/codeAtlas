package httpapi

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
)

type Route struct {
	Prefix string
	Target *url.URL
}

type ProxyHandler struct {
	logger *slog.Logger
	routes []Route
}

func NewProxyHandler(logger *slog.Logger, routes []Route) *ProxyHandler {
	return &ProxyHandler{
		logger: logger,
		routes: routes,
	}
}

func (h *ProxyHandler) Register(mux *http.ServeMux) error {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	for _, route := range h.routes {
		target := route.Target
		prefix := route.Prefix

		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.ModifyResponse = func(resp *http.Response) error {
			resp.Header.Del("Access-Control-Allow-Origin")
			resp.Header.Del("Access-Control-Allow-Methods")
			resp.Header.Del("Access-Control-Allow-Headers")
			resp.Header.Del("Access-Control-Expose-Headers")
			return nil
		}
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			h.logger.Error("proxy request failed", "target", target.String(), "path", r.URL.Path, "error", err)
			http.Error(w, "upstream service unavailable", http.StatusBadGateway)
		}

		mux.Handle(prefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.logger.Debug("proxy request", "prefix", prefix, "target", target.String(), "path", r.URL.Path, "method", r.Method)
			proxy.ServeHTTP(w, r)
		}))
	}

	return nil
}
