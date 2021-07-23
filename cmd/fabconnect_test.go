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
	"os"
	"path"
	"testing"

	"github.com/hyperledger-labs/firefly-fabconnect/internal/conf"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/rest/test"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/stretchr/testify/assert"
)

var tmpdir string
var testConfig *conf.RESTGatewayConf

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	teardown()
	os.Exit(code)
}

func setup() {
	tmpdir, testConfig = test.Setup()
	os.Setenv("FC_HTTP_PORT", "8002")
	os.Setenv("FC_EVENTS_POLLINGINTERVAL", "60")
}

func teardown() {
	test.Teardown(tmpdir)
}

func runNothing(cmd *cobra.Command, args []string) error {
	return nil
}

func TestMissingConfigFile(t *testing.T) {
	assert := assert.New(t)

	restGateway = nil
	rootCmd.RunE = runNothing
	args := []string{}
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	assert.EqualError(err, "Must provide REST Gateway client configuration path")
}

func TestMaxWaitTimeTooSmallWarns(t *testing.T) {
	assert := assert.New(t)

	restGateway = nil
	restGatewayConf.MaxTXWaitTime = 0
	rootCmd.RunE = runNothing
	args := []string{
		"-f", path.Join(tmpdir, "config.json"),
		"-x", "1",
	}
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	assert.NoError(err)
	assert.Equal(10, restGatewayConf.MaxTXWaitTime)
}

func TestEnvVarOverride(t *testing.T) {
	assert := assert.New(t)

	restGateway = nil
	rootCmd.RunE = runNothing
	_ = rootCmd.Execute()
	assert.Equal(8002, restGatewayConf.HTTP.Port)
	assert.Equal(uint64(60), restGatewayConf.Events.PollingIntervalSec)
}

func TestCmdArgsOverride(t *testing.T) {
	assert := assert.New(t)

	restGateway = nil
	rootCmd.RunE = runNothing
	args := []string{
		"-l", "8001",
	}
	rootCmd.SetArgs(args)
	_ = rootCmd.Execute()
	assert.Equal(8001, restGatewayConf.HTTP.Port)
}

func TestDefaultsInConfigFile(t *testing.T) {
	assert := assert.New(t)

	restGateway = nil
	restGatewayConf.HTTP.Port = 0
	rootCmd.RunE = runNothing
	viper.Reset()
	args := []string{
		"-f", path.Join(tmpdir, "config.json"),
	}
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	assert.NoError(err)
	assert.Equal(3000, restGatewayConf.HTTP.Port)
}

func TestMissingKafkaTopic(t *testing.T) {
	assert := assert.New(t)

	restGateway = nil
	rootCmd.RunE = runNothing
	args := []string{
		"-l", "8001",
		"-f", path.Join(tmpdir, "config.json"),
		"-b", "broker1",
		"-b", "broker2",
	}
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	assert.EqualError(err, "No output topic specified for bridge to send events to")
}

func TestCmdLaunch(t *testing.T) {
	assert := assert.New(t)

	restGateway = nil
	restGatewayConf.Kafka.Brokers = []string{}
	rootCmd.RunE = runNothing
	args := []string{
		"-l", "8001",
		"-f", path.Join(tmpdir, "config.json"),
	}
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	assert.Nil(err)
}

func TestKafkaSuccess(t *testing.T) {
	assert := assert.New(t)

	restGateway = nil
	rootCmd.RunE = runNothing
	args := []string{
		"-l", "8001",
		"-f", path.Join(tmpdir, "config.json"),
		"-b", "broker1", "-b", "broker2",
		"-t", "topic1", "-T", "topic2",
		"-g", "group1",
	}
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	assert.Nil(err)
	assert.Equal([]string{"broker1", "broker2"}, restGatewayConf.Kafka.Brokers)
}
