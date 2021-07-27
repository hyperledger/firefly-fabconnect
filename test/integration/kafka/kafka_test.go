package kafka

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"testing"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"gopkg.in/yaml.v2"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"

	lr "github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

var logr = &lr.Logger{
	Out:   os.Stdout,
	Level: lr.DebugLevel,
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
		GroupID	      string `yaml:"groupid"`
		NumPartitions int    `yaml:"partitions"`
		ReplFactor    int    `yaml:"replfactor"`
	} `yaml:"kafka"`
}

func createTopic(broker string, topic string, numPartitions int, replFactor int) {

	t, err := kafka.NewAdminClient(&kafka.ConfigMap{"bootstrap.servers": broker})
	if err != nil {
		logr.Errorf("Failed to create Admin client: %s\n", err)
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
			Topic:             topic,
			NumPartitions:     numPartitions,
			ReplicationFactor: replFactor}},
		kafka.SetAdminOperationTimeout(maxDur))
	if err != nil {
		logr.Errorf("Failed to create topic: %v", err)
		os.Exit(1)
	}

	for _, result := range results {
		logr.Debugf("Topic %s created.", result)
	}

}

func kafkaProducer(broker string, topic string) {
	p, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": broker})
	if err != nil {
		panic(err)
	}
	defer p.Close()

	go func() {
		for e := range p.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					logr.Errorf("Delivery failed: %v", ev.TopicPartition)
				} else {
					logr.Debugf("Produced message to %v", ev.TopicPartition)
				}
			}
		}
	}()

	p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          []byte("test"),
	}, nil)

	p.Flush(1 * 100)
}

func kafkaConsumer(broker string, topic string, groupid string) {

	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": broker,
		"group.id":          "ff",
		"auto.offset.reset": "earliest",
	})
	if err != nil {
		panic(err)
	}
	defer c.Close()

	c.SubscribeTopics([]string{topic}, nil)

	for {
		msg, err := c.ReadMessage(-1)
		if err == nil {
			logr.Debugf("Consumed message from %s: %s", msg.TopicPartition, string(msg.Value))
			break
		} else {
			logr.Errorf("Consumer error: %v (%v)", err, msg)
		}
	}
}

func TestIntegrationKafka(t *testing.T) {

	pwd, _ := os.Getwd()
	f, err := os.Open(string(pwd + "/config.yaml"))
	if err != nil {
		panic(err)
	}
	defer f.Close()

	var cfg Config
	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		panic(err)
	}

	var c []string

	ctx := context.Background()

	logr.Debug("Creating Kafka standalone cluster...")
	cmd := exec.Command("docker-compose", "up", "-d")
	_, err = cmd.Output()
	if err != nil {
		logr.Errorf(err.Error())
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

	for _, container := range containers {
		if string(container.Names[0]) == "/broker" || string(container.Names[0]) == "/zookeeper" || string(container.Names[0]) == "/control-center" {
			match, _ := regexp.MatchString("Up", string(container.Status))
			if match == true {
				conc := fmt.Sprintf("%v", match)
				c = append(c, conc)
			}
		}
	}

	if len(c) == 3 {
		logr.Debug("Waiting for Kafka cluster to start services (60 seconds)...")
		time.Sleep(60 * time.Second)
		createTopic(cfg.Kafka.Broker, cfg.Kafka.Topic, cfg.Kafka.NumPartitions, cfg.Kafka.ReplFactor)
		time.Sleep(10 * time.Second)
		kafkaProducer(cfg.Kafka.Broker, cfg.Kafka.Topic)
		time.Sleep(10 * time.Second)
		kafkaConsumer(cfg.Kafka.Broker, cfg.Kafka.Topic, cfg.Kafka.GroupID)

		logr.Debug("Removing Kafka standalone cluster...")
		cmd := exec.Command("docker-compose", "down")
		_, err := cmd.Output()
		if err != nil {
			logr.Errorf(err.Error())
			return
		}
		logr.Debug("All done!\n")
	}
}