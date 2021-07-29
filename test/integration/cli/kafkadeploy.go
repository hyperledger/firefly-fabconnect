package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/confluentinc/confluent-kafka-go/kafka"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"

	lr "github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

var log = &lr.Logger{
	Out:   os.Stdout,
	Level: lr.InfoLevel,
	Formatter: &prefixed.TextFormatter{
		DisableColors:   false,
		TimestampFormat: "2006-01-02 15:04:05",
		FullTimestamp:   true,
		ForceFormatting: true,
	},
}

type Config struct {
	Kafka struct {
		Broker        string `yaml:"broker"`
		Topic         string `yaml:"topic"`
		GroupID       string `yaml:"groupid"`
		NumPartitions int    `yaml:"partitions"`
		ReplFactor    int    `yaml:"replfactor"`
	} `yaml:"kafka"`
}

func getConfig() Config {

	var cfg Config

	pwd, _ := os.Getwd()
	f, err := os.Open(pwd + "/config.yml")
	if err != nil {
		panic(err)
	}

	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		panic(err)
	}

	return cfg
}

func createTopic() {

	cfg := getConfig()

	t, err := kafka.NewAdminClient(&kafka.ConfigMap{"bootstrap.servers": cfg.Kafka.Broker})
	if err != nil {
		log.Errorf("Failed to create Admin client: %s\n", err)
		os.Exit(1)
	}
	defer t.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	maxDur, err := time.ParseDuration("60s")
	if err != nil {
		panic("ParseDuration(60s)")
	}
	results, err := t.CreateTopics(
		ctx,
		[]kafka.TopicSpecification{{
			Topic:             cfg.Kafka.Topic,
			NumPartitions:     cfg.Kafka.NumPartitions,
			ReplicationFactor: cfg.Kafka.ReplFactor}},
		kafka.SetAdminOperationTimeout(maxDur))
	if err != nil {
		log.Errorf("Failed to create topic: %v", err)
		os.Exit(1)
	}

	for _, result := range results {
		log.Infof("Topic has been %s created.", result)
	}
}

func check(containers []types.Container) int {

	var c []string
	for _, container := range containers {
		if string(container.Names[0]) == "/broker" || string(container.Names[0]) == "/zookeeper" || string(container.Names[0]) == "/control-center" {
			r, _ := regexp.Compile("Up")
			match := r.MatchString(container.Status)
			if match {
				conc := fmt.Sprintf("%v", match)
				c = append(c, conc)
			}
		}
	}

	return len(c)
}

func main() {

	ctx := context.Background()

	log.Info("Creating Kafka standalone server...")
	cmd := exec.Command("docker-compose", "up", "-d")
	_, err := cmd.Output()
	if err != nil {
		log.Errorf(err.Error())
		return
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		panic(err)
	}

	delay := 60
	timeDelay := time.Duration(delay) * time.Second
	var endTime <-chan time.Time

checkloop:
	for {
		if endTime == nil {
			endTime = time.After(timeDelay)
		}
		select {
		case <-endTime:
			if check(containers) == 3 {
				log.Infof("Kafka server is running now (UI port: 9021).")
				createTopic()
				break checkloop
			}
		default:
			time.Sleep(5 * time.Second)
			continue
		}
	}
}
