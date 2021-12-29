// Copyright 2021 Kaleido
//
// SPDX-License-Identifier: Apache-2.0
//
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

	"github.com/hyperledger/firefly-fabconnect/internal/auth"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	"github.com/hyperledger/firefly-fabconnect/internal/events"
	"github.com/hyperledger/firefly-fabconnect/internal/messages"
	restasync "github.com/hyperledger/firefly-fabconnect/internal/rest/async"
	"github.com/hyperledger/firefly-fabconnect/internal/rest/identity"
	restsync "github.com/hyperledger/firefly-fabconnect/internal/rest/sync"
	restutil "github.com/hyperledger/firefly-fabconnect/internal/rest/utils"
	"github.com/hyperledger/firefly-fabconnect/internal/ws"
	"github.com/julienschmidt/httprouter"

	log "github.com/sirupsen/logrus"
)

const (
	errEventSupportMissing = "Event support is not configured on this gateway"
)

type router struct {
	syncDispatcher  restsync.SyncDispatcher
	asyncDispatcher restasync.AsyncDispatcher
	identityClient  identity.IdentityClient
	subManager      events.SubscriptionManager
	ws              ws.WebSocketServer
	httpRouter      *httprouter.Router
}

func newRouter(syncDispatcher restsync.SyncDispatcher, asyncDispatcher restasync.AsyncDispatcher, idClient identity.IdentityClient, sm events.SubscriptionManager, ws ws.WebSocketServer) *router {
	return &router{
		syncDispatcher:  syncDispatcher,
		asyncDispatcher: asyncDispatcher,
		identityClient:  idClient,
		subManager:      sm,
		ws:              ws,
		httpRouter:      httprouter.New(),
	}
}

func (r *router) addRoutes() {
	r.httpRouter.POST("/identities", r.registerUser)
	r.httpRouter.POST("/identities/:username/enroll", r.enrollUser)
	r.httpRouter.GET("/identities", r.listUsers)
	r.httpRouter.GET("/identities/:username", r.getUser)

	r.httpRouter.POST("/query", r.queryChaincode)
	r.httpRouter.POST("/transactions", r.sendTransaction)
	r.httpRouter.GET("/transactions/:txId", r.getTransaction)
	r.httpRouter.GET("/receipts", r.handleReceipts)
	r.httpRouter.GET("/receipts/:id", r.handleReceipts)

	r.httpRouter.POST("/eventstreams", r.createStream)
	r.httpRouter.PATCH("/eventstreams/:streamId", r.updateStream)
	r.httpRouter.GET("/eventstreams", r.listStreams)
	r.httpRouter.GET("/eventstreams/:streamId", r.getStream)
	r.httpRouter.DELETE("/eventstreams/:streamId", r.deleteStream)
	r.httpRouter.POST("/eventstreams/:streamId/suspend", r.suspendStream)
	r.httpRouter.POST("/eventstreams/:streamId/resume", r.resumeStream)
	r.httpRouter.POST("/subscriptions", r.createSubscription)
	r.httpRouter.GET("/subscriptions", r.listSubscription)
	r.httpRouter.GET("/subscriptions/:subscriptionId", r.getSubscription)
	r.httpRouter.DELETE("/subscriptions/:subscriptionId", r.deleteSubscription)
	r.httpRouter.POST("/subscriptions/:subscriptionId/reset", r.resetSubscription)

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

func (r *router) queryChaincode(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)

	msg, err := restutil.BuildQueryMessage(res, req, params)
	if err != nil {
		errors.RestErrReply(res, req, err.Error, err.StatusCode)
		return
	}
	// query requests are always synchronous
	r.syncDispatcher.DispatchMsgSync(req.Context(), res, req, msg)
}

func (r *router) getTransaction(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)

	msg, err := restutil.BuildTxByIdMessage(res, req, params)
	if err != nil {
		errors.RestErrReply(res, req, err.Error, err.StatusCode)
		return
	}
	// query requests are always synchronous
	r.syncDispatcher.DispatchMsgSync(req.Context(), res, req, msg)
}

func (r *router) sendTransaction(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)

	msg, opts, err := restutil.BuildTxMessage(res, req, params)
	if err != nil {
		errors.RestErrReply(res, req, err.Error, err.StatusCode)
		return
	}
	if opts.Sync {
		r.syncDispatcher.DispatchMsgSync(req.Context(), res, req, msg)
	} else {
		if asyncResponse, err := r.asyncDispatcher.DispatchMsgAsync(req.Context(), msg, opts.Ack); err != nil {
			errors.RestErrReply(res, req, err, 500)
		} else if opts.Ack {
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

func (r *router) createStream(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)
	if r.subManager == nil {
		errors.RestErrReply(res, req, errors.Errorf(errEventSupportMissing), 405)
		return
	}

	result, err := r.subManager.AddStream(res, req, params)
	if err != nil {
		errors.RestErrReply(res, req, err.Error, err.StatusCode)
		return
	}
	marshalAndReply(res, req, result)
}

func (r *router) updateStream(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)
	if r.subManager == nil {
		errors.RestErrReply(res, req, errors.Errorf(errEventSupportMissing), 405)
		return
	}

	result, err := r.subManager.UpdateStream(res, req, params)
	if err != nil {
		errors.RestErrReply(res, req, err.Error, err.StatusCode)
		return
	}
	marshalAndReply(res, req, result)
}

func (r *router) listStreams(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)
	if r.subManager == nil {
		errors.RestErrReply(res, req, errors.Errorf(errEventSupportMissing), 405)
		return
	}

	result := r.subManager.Streams(res, req, params)
	marshalAndReply(res, req, result)
}

func (r *router) getStream(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)
	if r.subManager == nil {
		errors.RestErrReply(res, req, errors.Errorf(errEventSupportMissing), 405)
		return
	}

	result, err := r.subManager.StreamByID(res, req, params)
	if err != nil {
		errors.RestErrReply(res, req, err.Error, err.StatusCode)
		return
	}
	marshalAndReply(res, req, result)
}

func (r *router) deleteStream(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)
	if r.subManager == nil {
		errors.RestErrReply(res, req, errors.Errorf(errEventSupportMissing), 405)
		return
	}

	result, err := r.subManager.DeleteStream(res, req, params)
	if err != nil {
		errors.RestErrReply(res, req, err.Error, err.StatusCode)
		return
	}
	marshalAndReply(res, req, result)
}

func (r *router) suspendStream(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)
	if r.subManager == nil {
		errors.RestErrReply(res, req, errors.Errorf(errEventSupportMissing), 405)
		return
	}

	result, err := r.subManager.SuspendStream(res, req, params)
	if err != nil {
		errors.RestErrReply(res, req, err.Error, err.StatusCode)
		return
	}
	marshalAndReply(res, req, result)
}

func (r *router) resumeStream(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)
	if r.subManager == nil {
		errors.RestErrReply(res, req, errors.Errorf(errEventSupportMissing), 405)
		return
	}

	result, err := r.subManager.ResumeStream(res, req, params)
	if err != nil {
		errors.RestErrReply(res, req, err.Error, err.StatusCode)
		return
	}
	marshalAndReply(res, req, result)
}

func (r *router) createSubscription(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)
	if r.subManager == nil {
		errors.RestErrReply(res, req, errors.Errorf(errEventSupportMissing), 405)
		return
	}

	result, err := r.subManager.AddSubscription(res, req, params)
	if err != nil {
		errors.RestErrReply(res, req, err.Error, err.StatusCode)
		return
	}
	marshalAndReply(res, req, result)
}

func (r *router) listSubscription(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)
	if r.subManager == nil {
		errors.RestErrReply(res, req, errors.Errorf(errEventSupportMissing), 405)
		return
	}

	result := r.subManager.Subscriptions(res, req, params)
	marshalAndReply(res, req, result)
}

func (r *router) getSubscription(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)
	if r.subManager == nil {
		errors.RestErrReply(res, req, errors.Errorf(errEventSupportMissing), 405)
		return
	}

	result, err := r.subManager.SubscriptionByID(res, req, params)
	if err != nil {
		errors.RestErrReply(res, req, err.Error, err.StatusCode)
		return
	}
	marshalAndReply(res, req, result)
}

func (r *router) deleteSubscription(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)
	if r.subManager == nil {
		errors.RestErrReply(res, req, errors.Errorf(errEventSupportMissing), 405)
		return
	}

	result, err := r.subManager.DeleteSubscription(res, req, params)
	if err != nil {
		errors.RestErrReply(res, req, err.Error, err.StatusCode)
		return
	}
	marshalAndReply(res, req, result)
}

func (r *router) resetSubscription(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)
	if r.subManager == nil {
		errors.RestErrReply(res, req, errors.Errorf(errEventSupportMissing), 405)
		return
	}

	result, err := r.subManager.ResetSubscription(res, req, params)
	if err != nil {
		errors.RestErrReply(res, req, err.Error, err.StatusCode)
		return
	}
	marshalAndReply(res, req, result)
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
