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

package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/hyperledger-labs/firefly-fabconnect/internal/conf"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/rest"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"

	_ "net/http/pprof"
)

func initLogging(debugLevel int) {
	log.SetFormatter(&prefixed.TextFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		DisableSorting:  true,
		ForceFormatting: true,
		FullTimestamp:   true,
	})
	switch debugLevel {
	case 0:
		log.SetLevel(log.ErrorLevel)
	case 1:
		log.SetLevel(log.InfoLevel)
	case 2:
		log.SetLevel(log.DebugLevel)
	case 3:
		log.SetLevel(log.TraceLevel)
	default:
		log.SetLevel(log.DebugLevel)
	}
	log.Debugf("Log level set to %d", debugLevel)
}

type cmdConfig struct {
	DebugLevel int
	DebugPort  int
	PrintYAML  bool
	Filename   string
}

var rootConfig = cmdConfig{}
var restGatewayConf = conf.RESTGatewayConf{}
var restGateway *rest.RESTGateway

var rootCmd = &cobra.Command{
	Use:   "fabconnect",
	Short: "Connectivity Bridge for Hyperledger Fabric permissioned chains",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		err := viper.Unmarshal(&restGatewayConf)
		if err != nil {
			return err
		}

		// allow tests to assign a mock
		if restGateway == nil {
			restGateway = rest.NewRESTGateway(&restGatewayConf)
		}
		err = restGateway.ValidateConf()
		if err != nil {
			return err
		}

		initLogging(rootConfig.DebugLevel)

		if rootConfig.DebugPort > 0 {
			go func() {
				log.Debugf("Debug HTTP endpoint listening on localhost:%d: %s", rootConfig.DebugPort, http.ListenAndServe(fmt.Sprintf("localhost:%d", rootConfig.DebugPort), nil))
			}()
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		err := startServer()
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	cobra.OnInitialize(initConfig)
	// all environment variables for the "fabconnect" command will have the "FC" prefix
	// e.g "FC_CONFIGFILE"
	viper.SetEnvPrefix("FC")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	rootCmd.Flags().IntVarP(&rootConfig.DebugLevel, "debug", "d", 1, "0=error, 1=info, 2=debug")
	rootCmd.Flags().IntVarP(&rootConfig.DebugPort, "debugPort", "Z", 6060, "Port for pprof HTTP endpoints (localhost only)")
	rootCmd.Flags().BoolVarP(&rootConfig.PrintYAML, "print-yaml-confg", "Y", false, "Print YAML config snippet and exit")
	rootCmd.Flags().StringVarP(&rootConfig.Filename, "configfile", "f", "", "Configuration file, must be one of .yml, .yaml, or .json")
	conf.CobraInit(rootCmd, &restGatewayConf)
}

func initConfig() {
	if rootConfig.Filename != "" {
		viper.SetConfigFile(rootConfig.Filename)
		if err := viper.ReadInConfig(); err != nil {
			log.Infof("Can't read config: %s, will rely on environment variables and command line arguments for required configurations\n", err)
		} else {
			log.Infof("Using config file: %s\n", viper.ConfigFileUsed())
		}
	} else {
		log.Info("No config file specified, will rely on environment variables and command line arguments for required configurations")
	}
}

func startServer() error {

	if rootConfig.PrintYAML {
		a, err := marshalToYAML(rootConfig)
		if err != nil {
			return err
		}
		b, err := marshalToYAML(restGatewayConf)
		if err != nil {
			return err
		}
		print(fmt.Sprintf("# Full YAML configuration processed from supplied file\n%s\n%s\n", string(a), string(b)))
	}

	err := restGateway.Init()
	if err != nil {
		return err
	}
	serverDone := make(chan bool)
	go func(done chan bool) {
		log.Info("Starting REST gateway")
		if err := restGateway.Start(); err != nil {
			log.Errorf("REST gateway failed: %s", err)
		}
		done <- true
	}(serverDone)

	// Terminate when ANY routine fails (do not wait for them all to complete)
	<-serverDone

	return nil
}

// MarshalToYAML marshals a JSON annotated structure into YAML, by first going to JSON
func marshalToYAML(conf interface{}) (yamlBytes []byte, err error) {
	var jsonBytes []byte
	if jsonBytes, err = json.Marshal(conf); err != nil {
		return
	}
	jsonAsMap := make(map[string]interface{})
	if err = json.Unmarshal(jsonBytes, &jsonAsMap); err != nil {
		return
	}
	yamlBytes, err = yaml.Marshal(&jsonAsMap)
	return
}

// Execute is called by the main method of the package
func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		return 1
	}
	return 0
}
