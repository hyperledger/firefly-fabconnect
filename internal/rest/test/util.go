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

package test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/hyperledger/firefly-fabconnect/internal/conf"
)

func Setup() (string, *conf.RESTGatewayConf) {
	var testConfigJSON = `{
  "maxInFlight": 10,
  "maxTXWaitTime": 60,
  "sendConcurrency": 25,
  "receipts": {
    "maxDocs": 1000,
    "queryLimit": 100,
    "retryInitialDelay": 5,
    "retryTimeout": 30,
    "leveldb": {
      "path": "/test-receipt-path"
    }
  },
  "http": {
    "port": 3000,
    "localAddr": "192.168.0.100"
  },
  "rpc": {
    "configPath": "/test-config-path"
  }
}`
	var testConfigJSONBad = `{
  "maxInFlight": "abc",
  "maxTXWaitTime": 60,
  "sendConcurrency": 25,
  "receipts": {
    "maxDocs": 1000,
    "queryLimit": 100,
    "retryInitialDelay": 5,
    "retryTimeout": 30,
    "leveldb": {
      "path": "/test-receipt-path"
    }
  },
  "http": {
    "port": 3000
  },
  "rpc": {
    "configPath": "/test-config-path"
  }
}`
	var testRPCConfig = `name: "test profile"
client:
  organization: org1
  credentialStore:
    path: "/tmp-client-creds-path"
organizations:
  org1:
    mspid: "org1MSP"
    cryptoPath: "/tmp-crypto-path-org1"
    peers:
      - peer1.org1.com
    certificateAuthorities:
      - ca-org1
  org2:
    mspid: "org2MSP"
    cryptoPath: "/tmp-crypto-path-org2"
    peers:
      - peer1.org2.com
    certificateAuthorities:
      - ca-org2
orderers:
  orderer1.org1.com:
    url: grpc://orderer1.org1.com:7050
peers:	
  peer1.org1.com:
    url: grpc://peer1.org1.com:7051
  peer1.org2.com:
    url: grpc://peer1.org2.com:7051
certificateAuthorities:
  ca-org1:
    url: https://ca.org1.com
    httpOptions:
      verify: false
    tlsCACerts:
      path: /tmp-cert
    caName: ca-org1
`
	var testCert = `-----BEGIN CERTIFICATE-----
MIIGZzCCBU+gAwIBAgIQA6UoMhKxyLpFA0uSvxPu6zANBgkqhkiG9w0BAQsFADBG
MQswCQYDVQQGEwJVUzEPMA0GA1UEChMGQW1hem9uMRUwEwYDVQQLEwxTZXJ2ZXIg
Q0EgMUIxDzANBgNVBAMTBkFtYXpvbjAeFw0yMTA2MTgwMDAwMDBaFw0yMjA3MTcy
MzU5NTlaMBQxEjAQBgNVBAMTCXBob3RpYy5pbzCCASIwDQYJKoZIhvcNAQEBBQAD
ggEPADCCAQoCggEBAI/Wt601Lqf7TQmKdNNri1B39t1hVknpCLLWlp1yUHlQi/Os
DrAUTaOks8AZYx3Vsfz9ZoXbscVgVYVzNHApHNQRzs5Tih6ylEJ7RdaXzkh6k2F/
+7wW9EWx+ZLoatv/Pex0ZikEMLVralISUxJ4QmzO8Mp5XOLctXI6hYFn1Qh/yMoo
cXBVryfDd6MchSsfMaDui+pbDE12E0qZHDZpmdR5XPVU4z69rlcd46Udp8O+CMx8
Umhgloz8EDLX39A3pgG8zWT4v4nykfLjWC6vbjSG3cXvB1m4qowVn3KuZZUCqNZG
CAiNBlZRBmy5Hp9m28sbGvI12EtFKCqIvOziZLkCAwEAAaOCA4EwggN9MB8GA1Ud
IwQYMBaAFFmkZgZSoHuVkjyjlAcnlnRb+T3QMB0GA1UdDgQWBBRpdXSfcJi2w22t
l1R9np4ziyk2WDCBswYDVR0RBIGrMIGogglwaG90aWMuaW+CCyoucGhvdGljLmlv
ghMqLml0LXN2Y3MucGhvdGljLmlvghYqLm5ldDItc3RhZ2UucGhvdGljLmlvghQq
LmRldi1zdmNzLnBob3RpYy5pb4IOKi5pdC5waG90aWMuaW+CFiouc3RhZ2Utc3Zj
cy5waG90aWMuaW+CESouc3RhZ2UucGhvdGljLmlvghAqLmRldjIucGhvdGljLmlv
MA4GA1UdDwEB/wQEAwIFoDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIw
OwYDVR0fBDQwMjAwoC6gLIYqaHR0cDovL2NybC5zY2ExYi5hbWF6b250cnVzdC5j
b20vc2NhMWIuY3JsMBMGA1UdIAQMMAowCAYGZ4EMAQIBMHUGCCsGAQUFBwEBBGkw
ZzAtBggrBgEFBQcwAYYhaHR0cDovL29jc3Auc2NhMWIuYW1hem9udHJ1c3QuY29t
MDYGCCsGAQUFBzAChipodHRwOi8vY3J0LnNjYTFiLmFtYXpvbnRydXN0LmNvbS9z
Y2ExYi5jcnQwDAYDVR0TAQH/BAIwADCCAX0GCisGAQQB1nkCBAIEggFtBIIBaQFn
AHUAKXm+8J45OSHwVnOfY6V35b5XfZxgCvj5TV0mXCVdx4QAAAF6HHUecgAABAMA
RjBEAiAvOlrqIjkKZzDh15KniTldUtU/TVrGs6BuH6+UWEaajgIgS/gS/hFSV7nx
2I58SQ/1SUoUIbCgNw8G5QJ3BHplY3MAdQAiRUUHWVUkVpY/oS/x922G4CMmY63A
S39dxoNcbuIPAgAAAXocdR6bAAAEAwBGMEQCIAct7dfvNHvls6pI62t2cIeormJz
Djt8ojdjFs6b192uAiApQt1Wxf5t93gjqwKL6yr7qt5+46nGlWlgwgSFmNnNcQB3
AFGjsPX9AXmcVm24N3iPDKR6zBsny/eeiEKaDf7UiwXlAAABehx1HsEAAAQDAEgw
RgIhANQ6Ykzq5l17md5IX1b7nsvAxeXKk5g/pXLhZ80os+RhAiEA8PnzkyqKz3Rg
4zvD48NknBAOsqVFBWSknZV0apTkoBkwDQYJKoZIhvcNAQELBQADggEBAKptxXbv
uCCh9cZYfSHnxyTPfdip0KwGcpEkwHR9q1fsUEbJk44YWePoYGhZ/lOBELwi/1ox
Z/v9LX21c1mxJ9lBnK1YqnZR3RqzFq78jSeqyp+ONmPJU6QfsqShifcYy2Y3fMs2
mHg9pLUS4SDnBvYJ5D59u/yOLDvRaX5GLAwUKMu/m+01Dte9+MxmAXt4edCFc6Hx
UgHqCW0DMhbhPaFxyOZsHG9X9uyJ+c7txjoRSmo0sc1TJ1VWuDzw39XM4PZvkGdi
9M/zq6m1ZyOTpfw7RTGle5Ph0XeE1Z0U62RsF1CKsjQ5em7+oVpFTKJ/1wEqQCDR
J4OVv51lNtDLT9k=
-----END CERTIFICATE-----
`

	tmpdir, _ := ioutil.TempDir("", "restgateway_test")

	// set up CA certs
	certPath := path.Join(tmpdir, "test.crt")
	_ = ioutil.WriteFile(certPath, []byte(testCert), 0644)
	// modify ca cert path
	ccp := strings.Replace(testRPCConfig, "/tmp-cert", certPath, 1)
	// set up crypto path for each org
	clientCredsPath := path.Join(tmpdir, "client")
	ccp = strings.Replace(ccp, "/tmp-client-creds-path", clientCredsPath, 1)
	org1MSPPath := path.Join(tmpdir, "org1")
	ccp = strings.Replace(ccp, "/tmp-crypto-path-org1", org1MSPPath, 1)
	org2MSPPath := path.Join(tmpdir, "org2")
	ccp = strings.Replace(ccp, "/tmp-crypto-path-org2", org2MSPPath, 1)
	// set up CCP
	ccpPath := path.Join(tmpdir, "ccp.yml")
	_ = ioutil.WriteFile(ccpPath, []byte(ccp), 0644)
	testConfigJSON = strings.Replace(testConfigJSON, "/test-config-path", ccpPath, 1)
	// setup receipt store
	receiptStorePath := path.Join(tmpdir, "receipts")
	if _, err := os.Stat(receiptStorePath); os.IsNotExist(err) {
		_ = os.Mkdir(receiptStorePath, 0777)
	}
	testConfigJSON = strings.Replace(testConfigJSON, "/test-receipt-path", receiptStorePath, 1)
	// write the config file
	_ = ioutil.WriteFile(path.Join(tmpdir, "config.json"), []byte(testConfigJSON), 0644)
	_ = ioutil.WriteFile(path.Join(tmpdir, "config-bad.json"), []byte(testConfigJSONBad), 0644)

	// init the default config
	testConfig := &conf.RESTGatewayConf{}
	err := json.Unmarshal([]byte(testConfigJSON), testConfig)
	if err != nil {
		fmt.Printf("Failed to unmarshal config file from JSON: %s. %s", testConfigJSON, err)
	}

	return tmpdir, testConfig
}

func Teardown(tmpdir string) {
	os.RemoveAll(tmpdir)
}
