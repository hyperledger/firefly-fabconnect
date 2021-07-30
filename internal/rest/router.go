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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/hyperledger-labs/firefly-fabconnect/internal/auth"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/errors"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/messages"
	restasync "github.com/hyperledger-labs/firefly-fabconnect/internal/rest/async"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/rest/identity"
	restsync "github.com/hyperledger-labs/firefly-fabconnect/internal/rest/sync"
	restutil "github.com/hyperledger-labs/firefly-fabconnect/internal/rest/utils"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/ws"
	"github.com/julienschmidt/httprouter"

	log "github.com/sirupsen/logrus"
)

type router struct {
	syncDispatcher  restsync.SyncDispatcher
	asyncDispatcher restasync.AsyncDispatcher
	identityClient  identity.IdentityClient
	ws              ws.WebSocketServer
	httpRouter      *httprouter.Router
}

func newRouter(syncDispatcher restsync.SyncDispatcher, asyncDispatcher restasync.AsyncDispatcher, idClient identity.IdentityClient, ws ws.WebSocketServer) *router {
	return &router{
		syncDispatcher:  syncDispatcher,
		asyncDispatcher: asyncDispatcher,
		identityClient:  idClient,
		ws:              ws,
		httpRouter:      httprouter.New(),
	}
}

func (r *router) addRoutes() {
	r.httpRouter.POST("/identities", r.registerUser)
	r.httpRouter.POST("/identities/:username/enroll", r.enrollUser)
	r.httpRouter.GET("/identities", r.listUsers)
	r.httpRouter.GET("/identities/:username", r.getUser)

	r.httpRouter.POST("/transactions", r.sendTransaction)
	r.httpRouter.GET("/receipts", r.handleReceipts)
	r.httpRouter.GET("/receipts/:id", r.handleReceipts)

	// router.POST("/eventstreams", r.createStream)
	// router.PATCH("/eventstreams/:streamId", r.updateStream)
	// router.GET("/eventstreams", r.listStreams)
	// router.GET("/eventstreams/:streamId", r.getStream)
	// router.DELETE("/eventstreams/:streamId", r.deleteStream)
	// router.POST("/eventstreams/:streamId/suspend", r.suspendStream)
	// router.POST("/eventstreams/:streamId/resume", r.resumeStream)
	// router.POST("/eventstreams/:streamId/subscriptions", r.createSubscription)
	// router.GET("/eventstreams/:streamId/subscriptions", r.listSubscription)
	// router.GET("/eventstreams/:streamId/subscriptions/:subscriptionId", r.getSubscription)
	// router.DELETE("/eventstreams/:streamId/subscriptions/:subscriptionId", r.deleteSubscription)
	// router.POST("/eventstreams/:streamId/subscriptions/:subscriptionId/reset", r.resetSubscription)

	r.httpRouter.GET("/ws", r.wsHandler)
	r.httpRouter.GET("/status", r.statusHandler)
}

func (r *router) newAccessTokenContextHandler() http.Handler {
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
			errors.RestErrReply(res, req, fmt.Errorf("Unauthorized"), 401)
			return
		}

		r.httpRouter.ServeHTTP(res, req.WithContext(authCtx))
	})
}

func (r *router) wsHandler(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	r.ws.NewConnection(res, req, params)
}

func (r *router) statusHandler(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	reply, _ := json.Marshal(&statusMsg{OK: true})
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(200)
	_, _ = res.Write(reply)
}

func (r *router) sendTransaction(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)

	c, err := restutil.ResolveParams(res, req, params)
	if err != nil {
		return
	}

	msg := &messages.SendTransaction{}
	msg.Headers.MsgType = messages.MsgTypeSendTransaction
	msg.Headers.ChannelID = c["headers"].(map[string]interface{})["channel"].(string)
	msg.Headers.Signer = c["headers"].(map[string]interface{})["signer"].(string)
	msg.ChaincodeName = c["chaincode"].(string)
	msg.Function = c["func"].(string)
	args := make([]string, len(c["args"].([]interface{})))
	for i, v := range c["args"].([]interface{}) {
		args[i] = v.(string)
	}
	msg.Args = args

	if strings.ToLower(restutil.GetFlyParam("sync", req, true)) == "true" {
		r.syncDispatcher.DispatchMsgSync(req.Context(), res, req, msg)
	} else {
		ack := (restutil.GetFlyParam("noack", req, true) != "true") // turn on ack's by default

		if asyncResponse, err := r.asyncDispatcher.DispatchMsgAsync(req.Context(), msg, ack); err != nil {
			errors.RestErrReply(res, req, err, 500)
		} else {
			restAsyncReply(res, req, asyncResponse)
		}
	}
}

func (r *router) registerUser(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)

	result, err := r.identityClient.Register(res, req, params)
	if err != nil {
		errors.RestErrReply(res, req, err.Error, err.StatusCode)
		return
	}
	marshalAndReply(res, req, result)
}

func (r *router) enrollUser(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)

	result, err := r.identityClient.Enroll(res, req, params)
	if err != nil {
		errors.RestErrReply(res, req, err.Error, err.StatusCode)
		return
	}
	marshalAndReply(res, req, result)
}

func (r *router) listUsers(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)
	result, err := r.identityClient.List(res, req, params)
	if err != nil {
		errors.RestErrReply(res, req, err.Error, err.StatusCode)
		return
	}
	marshalAndReply(res, req, result)
}

func (r *router) getUser(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)
	result, err := r.identityClient.Get(res, req, params)
	if err != nil {
		errors.RestErrReply(res, req, err.Error, err.StatusCode)
		return
	}
	marshalAndReply(res, req, result)
}

func (r *router) handleReceipts(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	r.asyncDispatcher.HandleReceipts(res, req, params)
}

func restAsyncReply(res http.ResponseWriter, req *http.Request, asyncResponse *messages.AsyncSentMsg) {
	resBytes, _ := json.Marshal(asyncResponse)
	status := 202 // accepted
	log.Infof("<-- %s %s [%d]:\n%s", req.Method, req.URL, status, string(resBytes))
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(status)
	_, _ = res.Write(resBytes)
}

func marshalAndReply(res http.ResponseWriter, req *http.Request, result interface{}) {
	resBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Errorf("Error serializing receipts: %s", err)
		errors.RestErrReply(res, req, errors.Errorf(errors.ReceiptStoreSerializeResponse), 500)
		return
	}
	status := 200
	log.Infof("<-- %s %s [%d]", req.Method, req.URL, status)
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(status)
	_, _ = res.Write(resBytes)
}
