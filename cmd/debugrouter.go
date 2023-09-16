package cmd

import (
	"net/http"
	"net/http/pprof"
	runtimepprof "runtime/pprof"

	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
	log "github.com/sirupsen/logrus"
)

type debugRouter struct {
	router *httprouter.Router
}

func NewDebugRouter() *debugRouter {
	r := httprouter.New()
	cors.Default().Handler(r)
	return &debugRouter{
		router: r,
	}
}

func (d *debugRouter) addRoutes() {
	d.router.GET("/debug/pprof/cmdline", d.cmdline)
	d.router.GET("/debug/pprof/profile", d.profile)
	d.router.GET("/debug/pprof/symbol", d.symbol)
	d.router.GET("/debug/pprof/trace", d.trace)
	d.router.GET("/debug/pprof/goroutines", d.goroutines)
	d.router.GET("/debug/pprof/", d.index)
}

func (d *debugRouter) cmdline(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)
	pprof.Cmdline(res, req)
}

func (d *debugRouter) profile(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)
	pprof.Profile(res, req)
}

func (d *debugRouter) symbol(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)

	pprof.Symbol(res, req)
}

func (d *debugRouter) trace(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)

	pprof.Trace(res, req)
}

func (d *debugRouter) index(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)

	pprof.Index(res, req)
}

func (d *debugRouter) goroutines(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)

	_ = runtimepprof.Lookup("goroutine").WriteTo(res, 1)
}
