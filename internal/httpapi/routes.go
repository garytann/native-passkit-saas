package httpapi

import "net/http"

const APIV1Prefix = "/api/v1"

type routeGroup struct {
	mux    *http.ServeMux
	prefix string
}

func newRouteGroup(mux *http.ServeMux, prefix string) routeGroup {
	return routeGroup{
		mux:    mux,
		prefix: prefix,
	}
}

func (g routeGroup) HandleFunc(pattern string, handler http.HandlerFunc) {
	g.mux.HandleFunc(g.prefix+pattern, handler)
}

func (g routeGroup) Path(pattern string) string {
	return g.prefix + pattern
}
