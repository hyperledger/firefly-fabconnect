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
	"os"
	"path"
	"regexp"
	"testing"

	"github.com/hyperledger/firefly-fabconnect/internal/rest/test"
	"github.com/spf13/cobra"

	"github.com/stretchr/testify/assert"
)

var tmpdir string

func runNothing(cmd *cobra.Command, args []string) error {
	return nil
}

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	teardown()
	os.Exit(code)
}

func setup() {
	os.Setenv("FC_HTTP_PORT", "8002")
	os.Setenv("FC_MAXINFLIGHT", "60")
}

func teardown() {
	test.Teardown(tmpdir)
}

func TestMissingConfigFile(t *testing.T) {
	assert := assert.New(t)

	rootCmd, _ := newRootCmd()
	args := []string{}
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	assert.EqualError(err, "Must provide REST Gateway client configuration path")
}

func TestBadConfigFile(t *testing.T) {
	assert := assert.New(t)

	tmpdir, _ := test.Setup()
	rootCmd, _ := newRootCmd()
	args := []string{
		"-f", path.Join(tmpdir, "config-bad.json"),
	}
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	assert.Regexp(regexp.MustCompile(`User credentials store creation failed`), err)
	test.Teardown(tmpdir)
}

func TestStartServerError(t *testing.T) {
	assert := assert.New(t)

	tmpdir, _ := test.Setup()
	rootCmd, _ := newRootCmd()
	args := []string{
		"-Y",
		"-f", path.Join(tmpdir, "config.json"),
		"-r", "/bad-path",
	}
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	assert.Regexp(regexp.MustCompile(`User credentials store creation failed. User credentials store path is empty`), err)
	test.Teardown(tmpdir)
}

func TestMaxWaitTimeTooSmallWarns(t *testing.T) {
	assert := assert.New(t)

	tmpdir, _ := test.Setup()
	rootCmd, restGatewayConf := newRootCmd()
	rootCmd.RunE = runNothing
	args := []string{
		"-f", path.Join(tmpdir, "config.json"),
		"-t", "1",
	}
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	assert.NoError(err)
	assert.Equal(10, restGatewayConf.MaxTXWaitTime)
	assert.Equal(1, restGatewayConf.Events.PollingIntervalSec)

	// test that the environment variable FC_HTTP_PORT overrides the port setting in the config file
	assert.Equal(8002, restGatewayConf.HTTP.Port)
	// test that settings in the config file that are not overriden
	assert.Equal("192.168.0.100", restGatewayConf.HTTP.LocalAddr)
	test.Teardown(tmpdir)
}

func TestEnvVarOverride(t *testing.T) {
	assert := assert.New(t)

	rootCmd, restGatewayConf := newRootCmd()
	rootCmd.RunE = runNothing
	_ = rootCmd.Execute()
	assert.Equal(8002, restGatewayConf.HTTP.Port)
	assert.Equal(60, restGatewayConf.MaxInFlight)
}

func TestCmdArgsOverride(t *testing.T) {
	assert := assert.New(t)

	rootCmd, restGatewayConf := newRootCmd()
	rootCmd.RunE = runNothing
	args := []string{
		"-P", "8001",
		"--events-polling-int", "10",
	}
	rootCmd.SetArgs(args)
	_ = rootCmd.Execute()
	assert.Equal(8001, restGatewayConf.HTTP.Port)
	assert.Equal(10, restGatewayConf.Events.PollingIntervalSec)
}

func TestDefaultsInConfigFile(t *testing.T) {
	assert := assert.New(t)

	tmpdir, _ := test.Setup()
	rootCmd, restGatewayConf := newRootCmd()
	rootCmd.RunE = runNothing
	args := []string{
		"-f", path.Join(tmpdir, "config.json"),
	}
	rootCmd.SetArgs(args)
	os.Unsetenv("FC_HTTP_PORT")
	err := rootCmd.Execute()
	assert.NoError(err)
	assert.Equal(3000, restGatewayConf.HTTP.Port)
	test.Teardown(tmpdir)
}

func TestMissingKafkaTopic(t *testing.T) {
	assert := assert.New(t)

	tmpdir, _ := test.Setup()
	rootCmd, _ := newRootCmd()
	rootCmd.RunE = runNothing
	args := []string{
		"-P", "8001",
		"-f", path.Join(tmpdir, "config.json"),
		"-b", "broker1",
		"-b", "broker2",
	}
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	assert.EqualError(err, "No output topic specified for bridge to send events to")
	test.Teardown(tmpdir)
}

func TestCmdLaunch(t *testing.T) {
	assert := assert.New(t)

	tmpdir, _ := test.Setup()
	rootCmd, _ := newRootCmd()
	args := []string{
		"-P", "8001",
		"-f", path.Join(tmpdir, "config.json"),
		"-Y",
	}
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	assert.Nil(err)
	test.Teardown(tmpdir)
}

func TestKafkaSuccess(t *testing.T) {
	assert := assert.New(t)

	tmpdir, _ := test.Setup()
	rootCmd, restGatewayConf := newRootCmd()
	rootCmd.RunE = runNothing
	args := []string{
		"-P", "8001",
		"-f", path.Join(tmpdir, "config.json"),
		"-b", "broker1", "-b", "broker2",
		"-n", "topic1", "-o", "topic2",
		"-g", "group1",
	}
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	assert.Nil(err)
	assert.Equal([]string{"broker1", "broker2"}, restGatewayConf.Kafka.Brokers)
	test.Teardown(tmpdir)
}
