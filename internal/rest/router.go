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
	restsync "github.com/hyperledger-labs/firefly-fabconnect/internal/rest/sync"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/utils"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/ws"
	"github.com/julienschmidt/httprouter"

	log "github.com/sirupsen/logrus"
)

type router struct {
	syncDispatcher  restsync.SyncDispatcher
	asyncDispatcher restasync.AsyncDispatcher
	ws              ws.WebSocketServer
	httpRouter      *httprouter.Router
}

func newRouter(syncDispatcher restsync.SyncDispatcher, asyncDispatcher restasync.AsyncDispatcher, ws ws.WebSocketServer) *router {
	return &router{
		syncDispatcher:  syncDispatcher,
		asyncDispatcher: asyncDispatcher,
		ws:              ws,
		httpRouter:      httprouter.New(),
	}
}

func (r *router) addRoutes() {
	// router.POST("/identities", r.restHandler)
	// router.GET("/identities", r.restHandler)
	// router.GET("/identities/:username", r.restHandler)

	r.httpRouter.POST("/transactions", r.sendTransaction)
	r.httpRouter.GET("/receipts", r.handleReceipts)
	r.httpRouter.GET("/receipts/:receiptId", r.handleReceipts)

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

	c, err := r.resolveParams(res, req, params)
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

	if strings.ToLower(getFlyParam("sync", req, true)) == "true" {
		r.syncDispatcher.DispatchMsgSync(req.Context(), res, req, msg)
	} else {
		ack := (getFlyParam("noack", req, true) != "true") // turn on ack's by default

		if asyncResponse, err := r.asyncDispatcher.DispatchMsgAsync(req.Context(), msg, ack); err != nil {
			errors.RestErrReply(res, req, err, 500)
		} else {
			restAsyncReply(res, req, asyncResponse)
		}
	}
}

func (r *router) handleReceipts(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	r.asyncDispatcher.HandleReceipts(res, req, params)
}

func (r *router) resolveParams(res http.ResponseWriter, req *http.Request, params httprouter.Params) (c map[string]interface{}, err error) {
	// if query parameters are specified, they take precedence over body properties
	channelParam := params.ByName("channel")
	methodParam := params.ByName("func")
	signer := getFlyParam("signer", req, false)
	blocknumber := getFlyParam("blocknumber", req, false)

	body, err := utils.ParseJSONPayload(req)
	if err != nil {
		errors.RestErrReply(res, req, err, 400)
		return nil, err
	}

	// consolidate inidividual parameters with the body parameters
	if channelParam != "" {
		body["headers"].(map[string]string)["channel"] = channelParam
	}
	if methodParam != "" {
		body["func"] = methodParam
	}
	if signer != "" {
		body["headers"].(map[string]string)["signer"] = signer
	}
	if blocknumber != "" {
		body["headers"].(map[string]string)["blocknumber"] = blocknumber
	}

	return body, nil
}

func getQueryParamNoCase(name string, req *http.Request) []string {
	name = strings.ToLower(name)
	_ = req.ParseForm()
	for k, vs := range req.Form {
		if strings.ToLower(k) == name {
			return vs
		}
	}
	return nil
}

// getFlyParam standardizes how special 'fly' params are specified, in query params, or headers
func getFlyParam(name string, req *http.Request, isBool bool) string {
	valStr := ""
	vs := getQueryParamNoCase(utils.GetenvOrDefaultLowerCase("PREFIX_SHORT", "fly")+"-"+name, req)
	if len(vs) > 0 {
		valStr = vs[0]
	}
	if isBool && valStr == "" && len(vs) > 0 {
		valStr = "true"
	}
	if valStr == "" {
		valStr = req.Header.Get("x-" + utils.GetenvOrDefaultLowerCase("PREFIX_LONG", "firefly") + "-" + name)
	}
	return valStr
}

func restAsyncReply(res http.ResponseWriter, req *http.Request, asyncResponse *messages.AsyncSentMsg) {
	resBytes, _ := json.Marshal(asyncResponse)
	status := 202 // accepted
	log.Infof("<-- %s %s [%d]:\n%s", req.Method, req.URL, status, string(resBytes))
	log.Debugf("<-- %s", resBytes)
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(status)
	_, _ = res.Write(resBytes)
}
