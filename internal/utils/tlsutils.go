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

package utils

import (
	"crypto/tls"
	"crypto/x509"
	"os"

	"github.com/hyperledger/firefly-fabconnect/internal/conf"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	log "github.com/sirupsen/logrus"
)

// CreateTLSConfiguration creates a tls.Config structure based on parsing the configuration passed in via a TLSConfig structure
func CreateTLSConfiguration(tlsConfig *conf.TLSConfig) (t *tls.Config, err error) {

	if !AllOrNoneReqd(tlsConfig.ClientCertsFile, tlsConfig.ClientKeyFile) {
		err = errors.Errorf(errors.ConfigTLSCertOrKey)
		return nil, err
	}

	mutualAuth := tlsConfig.ClientCertsFile != "" && tlsConfig.ClientKeyFile != ""
	log.Debugf("Kafka TLS Enabled=%t Insecure=%t MutualAuth=%t ClientCertsFile=%s PrivateKeyFile=%s CACertsFile=%s",
		tlsConfig.Enabled, tlsConfig.InsecureSkipVerify, mutualAuth, tlsConfig.ClientCertsFile, tlsConfig.ClientKeyFile, tlsConfig.CACertsFile)
	if !tlsConfig.Enabled {
		return nil, nil
	}

	var clientCerts []tls.Certificate
	if mutualAuth {
		var cert tls.Certificate
		if cert, err = tls.LoadX509KeyPair(tlsConfig.ClientCertsFile, tlsConfig.ClientKeyFile); err != nil {
			log.Errorf("Unable to load client key/certificate: %s", err)
			return nil, nil
		}
		clientCerts = append(clientCerts, cert)
	}

	var caCertPool *x509.CertPool
	if tlsConfig.CACertsFile != "" {
		var caCert []byte
		if caCert, err = os.ReadFile(tlsConfig.CACertsFile); err != nil {
			log.Errorf("Unable to load CA certificates: %s", err)
			return nil, nil
		}
		caCertPool = x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
	}
	// TODO: Fix linting: G402: TLS InsecureSkipVerify may be true.
	// #nosec G402
	t = &tls.Config{
		Certificates:       clientCerts,
		RootCAs:            caCertPool,
		InsecureSkipVerify: tlsConfig.InsecureSkipVerify,
	}
	return t, nil
}
