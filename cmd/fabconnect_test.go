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
}

func teardown() {
	test.Teardown(tmpdir)
}

func runNothing(cmd *cobra.Command, args []string) error {
	return nil
}

func TestCmdLaunch(t *testing.T) {
	assert := assert.New(t)

	restGateway = nil
	cmd, _ := newRootCmd()
	cmd.RunE = runNothing
	cmd.SetArgs([]string{"-l", "8001", "-f", path.Join(tmpdir, "config.json")})
	err := cmd.Execute()
	assert.Nil(err)
	restGateway.Shutdown()
}

func TestMissingConfigFile(t *testing.T) {
	assert := assert.New(t)

	restGateway = nil
	cmd, _ := newRootCmd()
	cmd.RunE = runNothing
	cmd.SetArgs([]string{"-l", "8001"})
	err := cmd.Execute()
	assert.EqualError(err, "No configuration filename specified")
}

func TestMaxWaitTimeTooSmallWarns(t *testing.T) {
	assert := assert.New(t)

	restGateway = nil
	cmd, _ := newRootCmd()
	cmd.RunE = runNothing
	cmd.SetArgs([]string{"-l", "8001", "-f", path.Join(tmpdir, "config.json"), "-x", "1"})
	err := cmd.Execute()
	assert.NoError(err)
	restGateway.Shutdown()
}

func TestKafkaCobraInitSuccess(t *testing.T) {
	assert := assert.New(t)

	restGateway = nil
	cmd, _ := newRootCmd()
	cmd.RunE = runNothing
	args := []string{
		"-l", "8001",
		"-f", path.Join(tmpdir, "config.json"),
		"-b", "broker1", "-b", "broker2",
		"-t", "topic1", "-T", "topic2",
		"-g", "group1",
	}
	cmd.SetArgs(args)
	err := cmd.Execute()
	assert.Nil(err)
	assert.Equal([]string{"broker1", "broker2"}, restGatewayConf.Kafka.Brokers)
	restGateway.Shutdown()
}

func TestKafkaCobraInitFailure(t *testing.T) {
	assert := assert.New(t)

	restGateway = nil
	cmd, _ := newRootCmd()
	cmd.RunE = runNothing
	args := []string{
		"-l", "8001",
		"-f", path.Join(tmpdir, "config.json"),
		"-b", "broker1",
		"-b", "broker2",
	}
	cmd.SetArgs(args)
	err := cmd.Execute()
	assert.EqualError(err, "No output topic specified for bridge to send events to")
	assert.Equal([]string{"broker1", "broker2"}, restGatewayConf.Kafka.Brokers)
	restGateway.Shutdown()
}
