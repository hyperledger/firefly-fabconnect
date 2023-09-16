// Copyright © 2023 Kaleido, Inc.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"net/http"
	"net/http/pprof"
	runtimepprof "runtime/pprof"

	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
	log "github.com/sirupsen/logrus"
)

type DebugRouter struct {
	router *httprouter.Router
}

func NewDebugRouter() *DebugRouter {
	r := httprouter.New()
	cors.Default().Handler(r)
	return &DebugRouter{
		router: r,
	}
}

func (d *DebugRouter) addRoutes() {
	d.router.GET("/debug/pprof/cmdline", d.cmdline)
	d.router.GET("/debug/pprof/profile", d.profile)
	d.router.GET("/debug/pprof/symbol", d.symbol)
	d.router.GET("/debug/pprof/trace", d.trace)
	d.router.GET("/debug/pprof/goroutines", d.goroutines)
	d.router.GET("/debug/pprof/", d.index)
}

func (d *DebugRouter) cmdline(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)
	pprof.Cmdline(res, req)
}

func (d *DebugRouter) profile(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)
	pprof.Profile(res, req)
}

func (d *DebugRouter) symbol(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)

	pprof.Symbol(res, req)
}

func (d *DebugRouter) trace(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)

	pprof.Trace(res, req)
}

func (d *DebugRouter) index(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)

	pprof.Index(res, req)
}

func (d *DebugRouter) goroutines(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)

	_ = runtimepprof.Lookup("goroutine").WriteTo(res, 1)
}
