# Kafka Integration Test

Test version: 0.1.0 (2021/08/09)\
Kafka version: 6.2.0 (Confluent)

## Description
Integration test for Kafka messaging system.\
- Official Confluent docker-compose.yml
- Golang Confluent Kafka client based on https://github.com/edenhill/librdkafka

## Dependencies

- `docker-engine` - 20.10.7+
- `docker-compose` - 1.29.2+\
or
- `docker desktop` - 3.5.2+ (MacOS, Windows or Linux)

## Ports

- `confluentinc/cp-zookeeper` - 2181 (default)
- `confluentinc/cp-server` - 9092 (default), 9101 (JMX)
- `confluentinc/cp-schema-registry` - 8081 (default)

Detail in docker-compose.yml

## Configuration (optional)

Automated integration test requires default parameters. However, custom or enterprise environments may require an adjustemnt. 

```yaml
kafka:
  broker: "localhost:9092"
  topic: "org1requests,org1replies"
  groupid: "ff"
  partitions: 2
  replfactor: 1
```

## Test run 

Process
1) `run docker-compose` > pull required images, and start them in Docker
2) `wait for services start`
3) `create Kafka topics`\
for each topic:
4) `create Kafka producer` and publish test message
5) `create Kafka consumer` and consumer test message
6) `evaluate` test results

Command
```sh
# -v verbose (to show test logs)
go test -v -run TestIntegrationKafka ./test/integration/kafka
```

## Improvements
- Apache Kafka or Confluent Kafka version as optional parameter
- Optional parameter to leave Kafka docker environment running to speed up re-test

## Change Log
- 0.1.0 (2021/08/09) Initial Version