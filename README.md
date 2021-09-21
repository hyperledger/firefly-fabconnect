# firefly-fabconnect
A reliable REST and websocket API to interact with a Fabric network and stream events.

## Architecture
### High Level Components
![high level architecture](/images/arch-1.jpg)

### Objects and Flows
![objects and flows architecture](/images/arch-2.png)
![kafkal handler architecture](/images/arch-3.png)


The component provides 3 high level sets of API endpoints:
- Client MSPs (aka the wallet): registering and enrolling identities to be used for signing transactions
- Transactions: submit transactions and query for transaction result/receipts
- Events: subscribe to events with regex based filter and stream to the client app via websocket

## Getting Started
After checking out the repo, simply run `make` to build and test.

To launch, first prepare the 2 configurations files:
- sample main config file:

```json
{
  "maxInFlight": 10,
  "maxTXWaitTime": 60,
  "sendConcurrency": 25,
  "receipts": {
    "maxDocs": 1000,
    "queryLimit": 100,
    "retryInitialDelay": 5,
    "retryTimeout": 30,
    "leveldb": {
      "path": "/Users/me/Documents/ff-test/receipts"
    }
  },
  "events": {
    "webhooksAllowPrivateIPs": true,
    "leveldb": {
      "path": "/Users/me/Documents/ff-test/events"
    }
  },
  "http": {
    "port": 3000
  },
  "rpc": {
    "useGatewayClient": true,
    "configPath": "/Users/me/Documents/ff-test/ccp.yml"
  }
}
```

- the standard Fabric common connection profile (CCP) file that describes the target Fabric network, at the location specified in the main config file above under `rpc.configPath`. For details on the CCP file, see [Fabric SDK documentation](https://hyperledger.github.io/fabric-sdk-node/release-1.4/tutorial-network-config.html). Note that the CCP file must contain the `client` section, which is required for the fabconnect to act as a client to Fabric networks.

Use the following command to launch the connector:
```
./fabconnect -f "/Users/me/Documents/ff-test/config.json"
```

### Hierarchical Configurations
Every configuration parameter can be specified in one of the following ways:
- configuration file that is specified with the `-f` command line parameter. this is overriden by...
- environment variables that follows the naming convention:
  - given a configuration property in the configuration JSON "prop1.prop2"
  - capitalized, exchanging `.` with `_`, then add the `FC_` prefix
  - becoming: `FC_PROP1_PROP2`
  - this is overriden by...
- command line parameter with a naming convention that follows the same dot-notaion of the property:
  - given "prop1.prop2"
  - the command line parameter should be `--prop1-prop2` or a shorthand variation

### Fixes Needed for multiple subscriptions under the same event stream
The current `fabric-sdk-go` uses an internal cache for event services, which builds keys only using the channel ID. This means if there are multiple subscriptions targeting the same channel, but specify different `fromBlock` parameters, only the first instance will be effective. All subsequent subscriptions will share the same event service, rendering their own `fromBlock` configuration ineffective.

A fix has been provided for this in the forked repository [https://github.com/kaleido-io/fabric-sdk-go](https://github.com/kaleido-io/fabric-sdk-go).

Follow these simple steps to integrate this fix (until it's contributed back to the official repo):
- clone the repository https://github.com/kaleido-io/fabric-sdk-go and place it peer to the `firefly-fabconnect` folder:
  ```
  workspace-root
     \_ fabric-sdk-go
     \_ firefly-fabconnect
  ```
- checkout branch `eventservice-cache-key`
- configure go to use it instead of the official package:
  ```
  go mod edit -replace=github.com/hyperledger/fabric-sdk-go=../fabric-sdk-go
  ```
- rebuild with `make`

### License
This project is licensed under the Apache 2 License - see the [`LICENSE`](LICENSE) file for details.
