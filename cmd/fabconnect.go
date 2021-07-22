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

package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/hyperledger-labs/firefly-fabconnect/internal/conf"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/errors"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/rest"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

var cmdConfig struct {
	DebugLevel int
	DebugPort  int
	PrintYAML  bool
	Filename   string
	Type       string
}

var restGatewayConf conf.RESTGatewayConf
var restGateway *rest.RESTGateway

func newRootCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "fabconnect",
		Short: "Connectivity Bridge for Hyperledger Fabric permissioned chains",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if cmdConfig.Filename == "" {
				err := errors.Errorf(errors.ConfigFileMissing)
				return err
			}

			viper.SetConfigFile(cmdConfig.Filename)
			if err := viper.ReadInConfig(); err == nil {
				log.Infof("Using config file: %s", viper.ConfigFileUsed())
			}

			err := readServerConfig()
			if err != nil {
				return err
			}

			// allow tests to assign a mock
			if restGateway == nil {
				restGateway = rest.NewRESTGateway(&restGatewayConf)
				err = restGateway.Init()
				if err != nil {
					return err
				}
			}
			err = restGateway.ValidateConf()
			if err != nil {
				return err
			}

			initLogging(cmdConfig.DebugLevel)

			if cmdConfig.DebugPort > 0 {
				go func() {
					log.Debugf("Debug HTTP endpoint listening on localhost:%d: %s", cmdConfig.DebugPort, http.ListenAndServe(fmt.Sprintf("localhost:%d", cmdConfig.DebugPort), nil))
				}()
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := startServer()
			return err
		},
	}

	cmd.Flags().IntVarP(&cmdConfig.DebugLevel, "debug", "d", 1, "0=error, 1=info, 2=debug")
	cmd.Flags().IntVarP(&cmdConfig.DebugPort, "debugPort", "Z", 6060, "Port for pprof HTTP endpoints (localhost only)")
	cmd.Flags().BoolVarP(&cmdConfig.PrintYAML, "print-yaml-confg", "Y", false, "Print YAML config snippet and exit")
	cmd.Flags().StringVarP(&cmdConfig.Filename, "filename", "f", os.Getenv("FABCONNECT_CONFIGFILE"), "Configuration file, must be one of .yml, .yaml, or .json")

	restGatewayConf = conf.RESTGatewayConf{}
	conf.CobraInit(cmd, &restGatewayConf)

	pflag.Parse()
	err := viper.BindPFlags(pflag.CommandLine)

	return cmd, err
}

func init() {
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	viper.AutomaticEnv()
}

func readServerConfig() error {
	err := viper.Unmarshal(&restGatewayConf)
	if err != nil {
		return err
	}
	return nil
}

func startServer() error {

	if cmdConfig.PrintYAML {
		b, err := marshalToYAML(&cmdConfig)
		print("# Full YAML configuration processed from supplied file\n" + string(b))
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
	rootCmd, err := newRootCmd()
	if err != nil {
		fmt.Println(err)
		return 1
	}
	if err = rootCmd.Execute(); err != nil {
		fmt.Println(err)
		return 1
	}
	return 0
}
