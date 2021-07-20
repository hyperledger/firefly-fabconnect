// Copyright 2021 Kaleido

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hyperledger-labs/firefly-fabconnect/internal/auth"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/conf"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/errors"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/fabric"
	restasync "github.com/hyperledger-labs/firefly-fabconnect/internal/rest/async"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/rest/receipt"
	restsync "github.com/hyperledger-labs/firefly-fabconnect/internal/rest/sync"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/tx"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/utils"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/ws"

	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

const (
	// MaxHeaderSize max size of content
	MaxHeaderSize = 16 * 1024
)

// RESTGateway as the HTTP gateway interface for fabconnect
type RESTGateway struct {
	printYAML   *bool
	config      *conf.RESTGatewayConf
	srv         *http.Server
	sendCond    *sync.Cond
	pendingMsgs map[string]bool
	successMsgs map[string]interface{}
	failedMsgs  map[string]error
}

type statusMsg struct {
	OK bool `json:"ok"`
}

type errMsg struct {
	Message string `json:"error"`
}

// NewRESTGateway constructor
func NewRESTGateway(config *conf.RESTGatewayConf, printYAML *bool) (g *RESTGateway) {
	g = &RESTGateway{
		config:      config,
		printYAML:   printYAML,
		sendCond:    sync.NewCond(&sync.Mutex{}),
		pendingMsgs: make(map[string]bool),
		successMsgs: make(map[string]interface{}),
		failedMsgs:  make(map[string]error),
	}
	return
}

func (g *RESTGateway) ValidateConf() error {
	// HTTP and RPC configurations are mandatory
	if g.config.HTTP.Port == 0 {
		return errors.Errorf(errors.ConfigRESTGatewayRequiredHTTPPort)
	}
	if g.config.RPC.ConfigPath == "" {
		return errors.Errorf(errors.ConfigRESTGatewayRequiredRPCPath)
	}
	if g.config.HTTP.LocalAddr == "" {
		g.config.HTTP.LocalAddr = "0.0.0.0"
	}
	return nil
}

func (g *RESTGateway) newAccessTokenContextHandler(parent http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {

		// Extract an access token from bearer token (only - no support for query params)
		accessToken := ""
		hSplit := strings.SplitN(req.Header.Get("Authorization"), " ", 2)
		if len(hSplit) == 2 && strings.ToLower(hSplit[0]) == "bearer" {
			accessToken = hSplit[1]
		}
		authCtx, err := auth.WithAuthContext(req.Context(), accessToken)
		if err != nil {
			log.Errorf("Error getting auth context: %s", err)
			g.sendError(res, "Unauthorized", 401)
			return
		}

		parent.ServeHTTP(res, req.WithContext(authCtx))
	})
}

func (g *RESTGateway) sendError(res http.ResponseWriter, msg string, code int) {
	reply, _ := json.Marshal(&errMsg{Message: msg})
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(code)
	_, _ = res.Write(reply)
}

func fabricRPCConnect(config conf.RPCConf) (fabric.RPCClient, error) {
	return fabric.RPCConnect(config)
}

// Start kicks off the HTTP listener and router
func (g *RESTGateway) Start() (err error) {

	if *g.printYAML {
		b, err := utils.MarshalToYAML(&g.config)
		print("# YAML Configuration snippet for REST Gateway\n" + string(b))
		return err
	}

	tlsConfig, err := utils.CreateTLSConfiguration(&g.config.HTTP.TLS)
	if err != nil {
		return
	}

	var processor tx.TxProcessor
	var rpcClient fabric.RPCClient
	if g.config.RPC.ConfigPath != "" {
		rpcClient, err = fabricRPCConnect(g.config.RPC)
		if err != nil {
			return err
		}
		processor = tx.NewTxProcessor(g.config)
		processor.Init(rpcClient)
	}
	syncDispatcher := restsync.NewSyncDispatcher(processor)
	receiptStore, err := receipt.NewReceiptStore(g.config)
	asyncDispatcher := restasync.NewAsyncDispatcher(g.config, processor, receiptStore)
	ws := ws.NewWebSocketServer()

	router := httprouter.New()
	r := newRouter(syncDispatcher, asyncDispatcher, ws)
	r.addRoutes(router)

	// if conf.EventLevelDBPath != "" {
	// 	sm := events.NewSubscriptionManager(&conf.SubscriptionManagerConf, rpc, ws)
	// 	err = sm.Init()
	// 	if err != nil {
	// 		return nil, ethconnecterrors.Errorf(ethconnecterrors.RESTGatewayEventManagerInitFailed, err)
	// 	}
	// }

	g.srv = &http.Server{
		Addr:           fmt.Sprintf("%s:%d", g.config.HTTP.LocalAddr, g.config.HTTP.Port),
		TLSConfig:      tlsConfig,
		Handler:        g.newAccessTokenContextHandler(router),
		MaxHeaderBytes: MaxHeaderSize,
	}

	readyToListen := make(chan bool)
	gwDone := make(chan error)
	svrDone := make(chan error)

	go func() {
		<-readyToListen
		log.Printf("HTTP server listening on %s", g.srv.Addr)
		err := g.srv.ListenAndServe()
		if err != nil {
			log.Errorf("Listening ended with: %s", err)
		}
		svrDone <- err
	}()
	go func() {
		err := asyncDispatcher.Run()
		if err != nil {
			log.Errorf("Webhooks Kafka bridge ended with: %s", err)
		}
		gwDone <- err
	}()
	for !asyncDispatcher.IsInitialized() {
		time.Sleep(250 * time.Millisecond)
	}
	readyToListen <- true

	// Clean up on SIGINT
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	// Complete the main routine if any child ends, or SIGINT
	select {
	case err = <-gwDone:
		break
	case err = <-svrDone:
		break
	case <-signals:
		break
	}

	// Ensure we shutdown the server
	// if sm != nil {
	// 	sm.Close()
	// }
	log.Infof("Shutting down HTTP server")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_ = g.srv.Shutdown(ctx)
	defer cancel()

	return
}
