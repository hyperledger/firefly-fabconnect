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

### API Specification

The API spec can be accessed at the endpoint `/api`, which is presented in the Swagger UI.

![swagger ui](/images/swagger-ui.png)

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

### Support for both Static and Dynamic Network Topology

There is support for using a full connection profile that describes the entire network, without relying on the peer's discovery service to discover the list of peers to send transaction proposals to. A sample connection profile can be seen in the folder [test/fixture/ccp.yml](/test/fixture/ccp.yml). This mode will be running if both `rpc.useGatewayClient` and `rpc.useGatewayServer` are missing or set to `false`.

There is also support for using the dynamic gateway client by relying on the peer's discovery service with a minimal connection profile. A sample connection profile can be seen in the folder [test/fixture/ccp-short.yml](/test/fixture/ccp-short.yml). This mode will be running if `rpc.useGatewayClient` is set to `true`.

Support for server-based gateway support, available in Fabric 2.4, is coming soon.

### Structured Data Support for Transaction Input with Schema Validation

When calling the `POST /transactions` endpoint, input data can be provided in any of the following formats:

- in the "traditional" array of strings corresponding to the target function's list of input parameters:

```json
POST http://localhost:3000/transactions?fly-sync=true&fly-signer=user001&fly-channel=default-channel&fly-chaincode=asset_transfer
{
    "headers": {
        "type": "SendTransaction"
    },
    "func": "CreateAsset",
    "args": ["asset204", "red", "10", "Tom", "123000"]
}
```

- provide a `payloadSchema` property in the input payload `headers`, using [JSON Schema](https://json-schema.org/) to define the list of parameters. The root type must be an `array`, with `prefixItems` to define the sequence of parameters:

```json
POST http://localhost:3000/transactions?fly-sync=true&fly-signer=user001&fly-channel=default-channel&fly-chaincode=asset_transfer
{
    "headers": {
        "type": "SendTransaction",
        "payloadSchema": {
            "type": "array",
            "prefixItems": [{
                "name": "id", "type": "string"
            }, {
                "name": "color", "type": "string"
            }, {
                "name": "size", "type": "integer"
            }, {
                "name": "owner", "type": "string"
            }, {
                "name": "value", "type": "string"
            }]
        }
    },
    "func": "CreateAsset",
    "args": {
        "owner": "Tom",
        "value": "123000",
        "size": 10,
        "id": "asset203",
        "color": "red"
    }
}
```

- when using `payloadSchema`, complex parameter structures are supported. Suppose the `CreateAsset` function has the following signature:

```golang
type Asset struct {
	ID             string `json:"ID"`
	Color          string `json:"color"`
	Size           int    `json:"size"`
	Owner          string `json:"owner"`
	Appraisal *Appraisal `json:"appraisal"`
}

type Appraisal struct {
	AppraisedValue int  `json:"appraisedValue"`
	Inspected      bool `json:"inspected"`
}

// CreateAsset issues a new asset to the world state with given details.
func (s *SmartContract) CreateAsset(ctx contractapi.TransactionContextInterface, id string, color string, size int, owner string, appraisal Appraisal) error {
  // implementation...
}
```

Note that the `appraisal` parameter is a complex type, the transaction input data can be specified as follows:

```json
POST http://localhost:3000/transactions?fly-sync=true&fly-signer=user001&fly-channel=default-channel&fly-chaincode=asset_transfer
{
    "headers": {
        "type": "SendTransaction",
        "payloadSchema": {
            "type": "array",
            "prefixItems": [{
                "name": "id", "type": "string"
            }, {
                "name": "color", "type": "string"
            }, {
                "name": "size", "type": "integer"
            }, {
                "name": "owner", "type": "string"
            }, {
                "name": "appraisal", "type": "object",
                "properties": {
                    "appraisedValue": {
                        "type": "integer"
                    },
                    "inspected": {
                        "type": "boolean"
                    }
                }
            }]
        }
    },
    "func": "CreateAsset",
    "args": {
        "owner": "Tom",
        "appraisal": {
            "appraisedValue": 123000,
            "inspected": true
        },
        "size": 10,
        "id": "asset205",
        "color": "red"
    }
}
```

### JSON Data Support in Events

If a chaincode publishes events with string or JSON data, fabconnect can be instructed to decode them from the byte array before sending the event to the listening client application. The decoding instructions can be provided during subscription.

For example, the following chaincode publishes an event containing a JSON structure in the payload:

```golang
	asset := Asset{
		ID:             id,
		Color:          color,
		Size:           size,
		Owner:          owner,
		Appraisal: &Appraisal{
			AppraisedValue: appraisal.AppraisedValue,
			Inspected: appraisal.inspected,
		},
	}
	assetJSON, _ := json.Marshal(asset)
	ctx.GetStub().SetEvent("AssetCreated", assetJSON)
```

An event subscription can be created as follows which contains instructions to decode the payload bytes:

```json
{
  "stream": "es-31e85b01-6440-4cc3-63e9-2aafc0d06466",
  "channel": "default-channel",
  "name": "sub-1",
  "signer": "user001",
  "fromBlock": "100",
  "filter": {
    "chaincodeId": "assettransfercomplex"
  },
  "payloadType": "stringifiedJSON"
}
```

Notice the `payloadType` property, which instructs fabconnect to decode the payload bytes into a JSON structure. As a result the client will receive the event JSON as follows:

```json
[
  {
    "chaincodeId": "assettransfercomplex",
    "blockNumber": 151,
    "transactionId": "8692254ea13e9f5cb021b613e722ce4610daa5c4529e1a9161497308b0278ca0",
    "eventName": "AssetCreated",
    "payload": {
      "ID": "asset204",
      "appraisal": {
        "appraisedValue": 123000,
        "inspected": true
      },
      "color": "red",
      "owner": "Tom",
      "size": 10
    },
    "subId": "sb-6859f687-61dd-44e8-6e8b-ddcf3b95b840"
  }
]
```

Besides `stringifiedJSON`, `string` is also supported as the payload type which represents UTF-8 encoded strings.

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
