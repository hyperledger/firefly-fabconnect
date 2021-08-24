package kafka

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type Config struct {
	Kafka struct {
		Broker        string `yaml:"broker"`
		Topic         string `yaml:"topic"`
		GroupID       string `yaml:"groupid"`
		NumPartitions int    `yaml:"partitions"`
		ReplFactor    int    `yaml:"replfactor"`
	} `yaml:"kafka"`
}

func createTopic(ts *testing.T, broker string, topic string, numPartitions int, replFactor int) {

	t, err := kafka.NewAdminClient(&kafka.ConfigMap{"bootstrap.servers": broker})
	if err != nil {
		ts.Logf("Failed to create Admin client: %s\n", err)
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
		ts.Logf("Failed to create topic: %v\n", err)
		os.Exit(1)
	}
	for _, result := range results {
		ts.Logf("Topic %s created.\n", result)
	}
}

func kafkaProducer(ts *testing.T, broker string, topic string) {
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
					ts.Logf("Delivery failed: %v\n", ev.TopicPartition)
				} else {
					ts.Logf("Produced message to %v\n", ev.TopicPartition)
				}
			}
		}
	}()

	err = p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          []byte("test"),
	}, nil)
	if err != nil {
		ts.Logf("Producer error: %v\n", err)
	}

	p.Flush(1 * 100)
}

func kafkaConsumer(ts *testing.T, broker string, topic string, groupid string) (res string) {

	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": broker,
		"group.id":          "ff",
		"auto.offset.reset": "earliest",
	})
	if err != nil {
		panic(err)
	}
	defer c.Close()

	err = c.SubscribeTopics([]string{topic}, nil)
	if err != nil {
		ts.Logf("Subscription error: %v\n", err)
	}

	for {
		msg, err := c.ReadMessage(-1)
		if err == nil {
			res = string(msg.Value)
			ts.Logf("Consumed message from %s: %s\n", msg.TopicPartition, res)
			break
		} else {
			ts.Logf("Consumer error: %v (%v)\n", err, msg)
		}
	}

	return res
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

	var match []string

	ctx := context.Background()

	t.Log("Creating Kafka standalone cluster...")
	cmd := exec.Command("docker-compose", "up", "-d")
	_, err = cmd.Output()
	if err != nil {
		t.Log(err.Error())
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
		if string(container.Names[0]) == "/broker" || string(container.Names[0]) == "/zookeeper" || string(container.Names[0]) == "/schema-registry" {
			r, _ := regexp.Compile("Up")
			f := r.FindString(container.Status)
			match = append(match, f)
		}
	}

	if len(match) == 3 {
		t.Log("Waiting for Kafka cluster to start services (60 seconds)...")
		time.Sleep(60 * time.Second)
		topics := strings.Split(cfg.Kafka.Topic, ",")
		for _, topicName := range topics {
			createTopic(t, cfg.Kafka.Broker, topicName, cfg.Kafka.NumPartitions, cfg.Kafka.ReplFactor)
			time.Sleep(10 * time.Second)
			kafkaProducer(t, cfg.Kafka.Broker, topicName)
			time.Sleep(10 * time.Second)
			res := kafkaConsumer(t, cfg.Kafka.Broker, topicName, cfg.Kafka.GroupID)
			assert.Equal(t, res, "test")
		}

		t.Log("Removing Kafka standalone cluster...")
		cmd := exec.Command("docker-compose", "down")
		_, err := cmd.Output()
		if err != nil {
			t.Log(err.Error())
			return
		}

		t.Log("All done!")
	}
}
