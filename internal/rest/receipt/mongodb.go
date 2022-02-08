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

package receipt

import (
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/hyperledger/firefly-fabconnect/internal/conf"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	"github.com/hyperledger/firefly-fabconnect/internal/utils"
	log "github.com/sirupsen/logrus"
)

const (
	mongoConnectTimeout = 10 * 1000
)

// MongoDatabase is a subset of mgo that we use, allowing stubbing.
type MongoDatabase interface {
	Connect(url string, timeout time.Duration) error
	GetCollection(database string, collection string) MongoCollection
}

// MongoCollection is the subset of mgo that we use, allowing stubbing
type MongoCollection interface {
	Insert(...interface{}) error
	Create(info *mgo.CollectionInfo) error
	EnsureIndex(index mgo.Index) error
	Find(query interface{}) MongoQuery
}

// MongoQuery is the subset of mgo that we use, allowing stubbing
type MongoQuery interface {
	Limit(n int) *mgo.Query
	Skip(n int) *mgo.Query
	Sort(fields ...string) *mgo.Query
	All(result interface{}) error
	One(result interface{}) error
}

type mgoWrapper struct {
	session *mgo.Session
}

func (m *mgoWrapper) Connect(url string, timeout time.Duration) (err error) {
	m.session, err = mgo.DialWithTimeout(url, timeout)
	return
}

func (m *mgoWrapper) GetCollection(database string, collection string) MongoCollection {
	return &collWrapper{coll: m.session.DB(database).C(collection)}
}

type collWrapper struct {
	coll *mgo.Collection
}

func (m *collWrapper) Insert(docs ...interface{}) error {
	return m.coll.Insert(docs...)
}

func (m *collWrapper) Create(info *mgo.CollectionInfo) error {
	return m.coll.Create(info)
}

func (m *collWrapper) EnsureIndex(index mgo.Index) error {
	return m.coll.EnsureIndex(index)
}

func (m *collWrapper) Find(query interface{}) MongoQuery {
	return m.coll.Find(query)
}

type mongoReceipts struct {
	config     *conf.ReceiptsDBConf
	mgo        MongoDatabase
	collection MongoCollection
}

func newMongoReceipts(config *conf.ReceiptsDBConf) *mongoReceipts {
	return &mongoReceipts{
		config: config,
		mgo:    &mgoWrapper{},
	}
}

func (m *mongoReceipts) ValidateConf() (err error) {
	if !utils.AllOrNoneReqd(m.config.MongoDB.URL, m.config.MongoDB.Database, m.config.MongoDB.Collection) {
		err = errors.Errorf(errors.ConfigRESTGatewayRequiredReceiptStore)
		return
	}
	if m.config.QueryLimit < 1 {
		m.config.QueryLimit = 100
	}
	return
}

func (m *mongoReceipts) Init() (err error) {
	if m.config.MongoDB.ConnectTimeoutMS <= 0 {
		m.config.MongoDB.ConnectTimeoutMS = mongoConnectTimeout
	}
	err = m.mgo.Connect(m.config.MongoDB.URL, time.Duration(m.config.MongoDB.ConnectTimeoutMS)*time.Millisecond)
	if err != nil {
		err = errors.Errorf(errors.ReceiptStoreMongoDBConnect, err)
		return
	}
	m.collection = m.mgo.GetCollection(m.config.MongoDB.Database, m.config.MongoDB.Collection)
	if collErr := m.collection.Create(&mgo.CollectionInfo{
		Capped:  (m.config.MaxDocs > 0),
		MaxDocs: m.config.MaxDocs,
	}); collErr != nil {
		log.Infof("MongoDB collection exists: %s", err)
	}

	index := mgo.Index{
		Key:        []string{"receivedAt"},
		Unique:     false,
		DropDups:   false,
		Background: true,
		Sparse:     true,
	}
	if err = m.collection.EnsureIndex(index); err != nil {
		err = errors.Errorf(errors.ReceiptStoreMongoDBIndex, err)
		return
	}

	log.Infof("Connected to MongoDB on %s DB=%s Collection=%s", m.config.MongoDB.URL, m.config.MongoDB.Database, m.config.MongoDB.Collection)
	return
}

// AddReceipt processes an individual reply message, and contains all errors
// To account for any transitory failures writing to mongoDB, it retries adding receipt with a backoff
func (m *mongoReceipts) AddReceipt(requestID string, receipt *map[string]interface{}) (err error) {
	return m.collection.Insert(*receipt)
}

// GetReceipts Returns recent receipts with skip & limit
func (m *mongoReceipts) GetReceipts(skip, limit int, ids []string, sinceEpochMS int64, from, to, start string) (*[]map[string]interface{}, error) {
	filter := bson.M{}
	if len(ids) > 0 {
		filter["_id"] = bson.M{
			"$in": ids,
		}
	}
	if sinceEpochMS > 0 {
		filter["receivedAt"] = bson.M{
			"$gt": sinceEpochMS,
		}
	}
	if from != "" {
		filter["from"] = from
	}
	if to != "" {
		filter["to"] = to
	}
	query := m.collection.Find(filter)
	query.Sort("-receivedAt")
	if limit > 0 {
		query.Limit(limit)
	}
	if skip > 0 {
		query.Skip(skip)
	}
	// Perform the query
	var err error
	results := make([]map[string]interface{}, 0, limit)
	if err = query.All(&results); err != nil && err != mgo.ErrNotFound {
		return nil, err
	}
	return &results, nil
}

// getReply handles a HTTP request for an individual reply
func (m *mongoReceipts) GetReceipt(requestID string) (*map[string]interface{}, error) {
	query := m.collection.Find(bson.M{"_id": requestID})
	result := make(map[string]interface{})
	if err := query.One(&result); err == mgo.ErrNotFound {
		return nil, nil
	} else if err != nil {
		return nil, err
	} else {
		return &result, nil
	}
}

func (m *mongoReceipts) Close() {}
