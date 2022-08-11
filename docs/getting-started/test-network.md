# firefly-fabconnect with Fabric test-network
Sometimes a developer may want to connect the firefly-fabconnect component to a Fabric network that already exists on their machine or elsewhere, `ex: The Fabric test-network`.

This guide describes several ways to connect firefly-fabconnect to an external Hyperledger Fabric blockchain network to interact with network and stream events:

- Using __fabconnect__ and the [__Fabric test-network__](https://github.com/hyperledger/fabric-samples/tree/main/test-network) in the samples repository.
  > Using a docker container managed by docker-compose.
- Using __fabconnect__ and the [__Fabric test-network-nano-bash__](https://github.com/hyperledger/fabric-samples/tree/main/test-network-nano-bash) in the samples repository.
  > Using an instance of fabconnect compiled from source code.

This guide assumes:
- Working directory is `$HOME` directory
- The chaincode `asset-transfer-basic` is installed

## Table of Contents

- [Using fabconnect and the Fabric test-network](#fabconnect_testnetwork)
  * [Download fabric-samples, docker images, and fabric binaries](#fabconnect_testnetwork_prerequisites)
  * [Bring up the test network](#fabconnect_testnetwork_bringup_testnetwork)
  * [Configure the fabconnect](#configure_fabconnect_testnetwork)
    * [Create and prepare your fabconnect service configuration file (fabconnect.yaml)](#create_prepare_fabconnect_testnetwork)
    * [Create and prepare your fabric connection profile (ccp.yaml)](#create_prepare_cpp_fabconnect_testnetwork)
  * [Bring up fabconnect](#fabconnect_testnetwork_bringup_fabconnect)
- [Using fabconnect and the Fabric test-network-nano-bash](#fabconnect_testnetworknanobash)
  * [Bring up test-network-nano-bash](#fabconnect_testnetwork_bringup_testnetworknanobash)
  * [Download and build fabconnect](#download_fabconnect_build)
  * [Configure the fabconnect](#configure_fabconnect_testnetworknanobash)
    * [Create and prepare your fabconnect service configuration file (fabconnect.yaml)](#create_prepare_fabconnect_testnetworknanobash)
    * [Create and prepare your fabric connection profile (ccp.yaml)](#create_prepare_cpp_fabconnect_testnetworknanobash)
  * [Configure the identity (signer)](#fabconnect_configure_signer)
  * [Launch the connector](#fabconnect_testnetworknanobash_launch_connector)
- [Interacting with the asset-transfer-basic chaincode](#fabconnect_testnetwork_interacting_cc)
- [Documentation](#doc)
- [Troubleshooting](#troubleshooting)

## Using fabconnect and the Fabric test-network <a name="fabconnect_testnetwork"></a>

This mode runs a `fabconnect` instance using a docker container managed by docker-compose that connects to the [Fabric test-network](https://github.com/hyperledger/fabric-samples/tree/main/test-network) using __Certificate Authorities (CAs)__.

### Download fabric-samples, docker images, and fabric binaries<a name="fabconnect_testnetwork_download_prerequisites"></a>

> **NOTE:** Please refer to the instructions for [Install Fabric and Fabric Samples](https://hyperledger-fabric.readthedocs.io/en/latest/install.html) 
 

### Bring up test-network <a name="fabconnect_testnetwork_bringup_testnetwork"></a>

```bash
cd fabric-samples/test-network
```
Run the following command to start the network, create a channel with  `mychannel` default name, and generate the cryptographic artifacts with the Fabric CA:

```bash
./network.sh up createChannel -ca -c mychannel
```

After you have used the network.sh to create a channel, you can start the `asset-transfer-basic` chaincode on the channel using the following command:
```bash
./network.sh deployCC -ccn basic -ccp ../asset-transfer-basic/chaincode-go -ccl go
```

Refer to the instructions for more details [Using the Fabric test-network](https://hyperledger-fabric.readthedocs.io/en/latest/test_network.html#using-the-fabric-test-network)

### Configure the fabconnect<a name="configure_fabconnect_testnetwork"></a>
```bash
cd $HOME
```
Create the 'ff-test' directory and the necessary folder structure
```bash
mkdir -p ff-test/events  ff-test/receipts
cd ff-test
```

In $HOME/ff-test, you need to add a compose file.
```bash
touch docker-compose.yaml
```
Your docker-compose.yaml file after editing should look something like:
```yaml
version: "2.1"

networks:
    default:
        external:
            name: fabric_test

services:
    fabconnect_0:
        container_name: firefly_fabconnect_0
        image: ghcr.io/hyperledger/firefly-fabconnect:latest
        command: -f /fabconnect/fabconnect.yaml
        volumes:
            - fabconnect_receipts_0:/fabconnect/receipts
            - fabconnect_events_0:/fabconnect/events
            - ./fabconnect.yaml:/fabconnect/fabconnect.yaml
            - ./ccp.yaml:/fabconnect/ccp.yaml
            - $HOME/fabric-samples/test-network/organizations:/etc/firefly/organizations

        # If your services are listening on “localhost” and not on the Docker network (172.x.x.x),
        # you need to run your container on the host network (network_mode)
        # network_mode: host

        # In Mac to connect to the Docker host from inside a Docker container, use the "host.docker.internal" hostname
        # should work without additional configuration, but if not, you can set the extra host this way.
        # With "host-gateway" instead of an IP address.
        # extra_hosts:
        #     - "host.docker.internal:host-gateway"
        ports:
            - "3000:3000"
        healthcheck:
            test:
                - CMD
                - wget
                - -O
                - '-'
                - http://localhost:3000/status
        logging:
            driver: json-file
            options:
                max-file: "1"
                max-size: 10m
volumes:
    fabconnect_events_0: {}
    fabconnect_receipts_0: {}
```
> **NOTE:** Note that the `fabconnect_0` container will join the pre-existing network **"fabric_test"**, created by the Fabric test-network in the [Bring up the test network](#fabconnect_testnetwork_bringup_testnetwork) step.

#### Create and prepare your fabconnect service configuration file (fabconnect.yaml)<a name="create_prepare_fabconnect_testnetwork"></a>
Let's create in $HOME/ff-test the fabconnect service configuration file (must be one of .yml, .yaml, or .json):

```bash
touch $HOME/ff-test/fabconnect.yaml
```
Your `fabconnect.yaml` file after editing should look something like:
```yaml
maxinflight: 10
maxtxwaittime: 60
sendconcurrency: 25
receipts:
  maxdocs: 1000
  querylimit: 100
  retryinitialdelay: 5
  retrytimeout: 30
  leveldb:
    path: /fabconnect/receipts
events:
  webhooksAllowPrivateIPs: true
  leveldb:
    path: /fabconnect/events
http:
  port: 3000
rpc:
  useGatewayClient: true
  configpath: /fabconnect/ccp.yaml
```

#### Create and prepare your fabric connection profile (ccp.yaml)<a name="create_prepare_cpp_fabconnect_testnetwork"></a>
Let's create in $HOME/ff-test the fabric connection profile (ccp.yaml):

```bash
touch $HOME/ff-test/ccp.yaml
```
Your `ccp.yaml` file after editing should look something like:

```yaml
certificateAuthorities:
  ca.org1.example.com:
    tlsCACerts:
      path: /etc/firefly/organizations/peerOrganizations/org1.example.com/ca/ca.org1.example.com-cert.pem
    url: https://ca_org1:7054
    registrar:
      enrollId: admin
      enrollSecret: adminpw
    httpOptions:
      verify: false
channels:
  mychannel:
    peers:
      peer1.org1.com:
        chaincodeQuery: true
        endorsingPeer: true
        eventSource: true
        ledgerQuery: true
client:
  BCCSP:
    security:
      default:
        provider: SW
      enabled: true
      hashAlgorithm: SHA2
      level: 256
      softVerify: true
  credentialStore:
    cryptoStore:
      path: /etc/firefly/organizations/peerOrganizations/org1.example.com/users
    path: /etc/firefly/organizations/peerOrganizations/org1.example.com/users
  cryptoconfig:
    path: /etc/firefly/organizations/peerOrganizations/org1.example.com/users
  logging:
    level: info
  organization: org1
orderers:
  orderer1:
    tlsCACerts:
      path: /etc/firefly/organizations/ordererOrganizations/example.com/tlsca/tlsca.example.com-cert.pem
    url: orderer.example.com:7050
organizations:
  org1:
    certificateAuthorities:
      - ca.org1.example.com
    cryptoPath: /etc/firefly/organizations/fabric-ca/org1/msp
    mspid: Org1MSP
    peers:
      - peer1.org1.com
peers:
  peer1.org1.com:
    tlsCACerts:
      path: /etc/firefly/organizations/peerOrganizations/org1.example.com/tlsca/tlsca.org1.example.com-cert.pem
    url: peer0.org1.example.com:7051
    grpcOptions:
      ssl-target-name-override: peer0.org1.example.com
      hostnameOverride: peer0.org1.example.com
version: 1.1.0%

entityMatchers:
  peer:
    - pattern: peer0.org1.example.(\w+)
      urlSubstitutionExp: peer0.org1.example.com:7051
      sslTargetOverrideUrlSubstitutionExp: peer0.org1.example.com
      mappedHost: peer0.org1.example.com

    - pattern: (\w+).org1.example.(\w+):(\d+)
      urlSubstitutionExp: peer0.org1.example.com:${2}
      sslTargetOverrideUrlSubstitutionExp: ${1}.org1.example.com
      mappedHost: ${1}.org1.example.com

    - pattern: (\w+):7051
      urlSubstitutionExp: peer0.org1.example.com:7051
      sslTargetOverrideUrlSubstitutionExp: peer0.org1.example.com
      mappedHost: peer0.org1.example.com

  orderer:

    - pattern: (\w+).example.(\w+)
      urlSubstitutionExp: orderer.example.com:7050
      sslTargetOverrideUrlSubstitutionExp: orderer.example.com
      mappedHost: orderer.example.com

    - pattern: (\w+).example.(\w+):(\d+)
      urlSubstitutionExp: orderer.example.com:7050
      sslTargetOverrideUrlSubstitutionExp: orderer.example.com
      mappedHost: orderer.example.com

  certificateAuthority:
    - pattern: (\w+).org1.example.(\w+)
      urlSubstitutionExp: https://ca_org1:7054
      sslTargetOverrideUrlSubstitutionExp: org1.example.com
      mappedHost: ca.org1.example.com
```
### Bring up fabconect<a name="fabconnect_testnetwork_bringup_fabconnect"></a>

```bash
docker-compose up -d
```

Visit the url: http://__ip_address_here__:3000/api

## Using fabconnect and the Fabric test-network-nano-bash <a name="fabconnect_testnetworknanobash"></a>

This mode runs a `fabconnect` instance that we build from source code that connects to the [Fabric test-network-nano-bash](https://github.com/hyperledger/fabric-samples/tree/main/test-network-nano-bash) and unlike the previous mode, does not use docker containers or Certificate Authorities (CAs).

### Bring up `"test-network-nano-bash"`<a name="fabconnect_testnetwork_bringup_testnetworknanobash"></a>

👉🏾 [Follow the instructions to bring up network and install the chaincode.](https://github.com/hyperledger/fabric-samples/tree/main/test-network-nano-bash#test-network---nano-bash)

### Download and compile fabconnect<a name="download_fabconnect_build"></a>

```bash
cd $HOME
```

Download the fabconnect repository
```bash
git clone https://github.com/hyperledger/firefly-fabconnect.git
```

Compile fabconnect
```bash
go mod vendor
go build -o fabconnect
```

Move fabconnect binary to /usr/bin/local
```bash
 chmod +x fabconnect && sudo cp fabconnect /usr/local/bin/
```

### Configure the fabconnect<a name="configure_fabconnect_testnetworknanobash"></a>
```bash
cd $HOME
```
Create the 'ff-test' directory and the necessary folder structure
```bash
mkdir -p ff-test/events  ff-test/receipts
cd ff-test
```

#### Create and prepare your fabconnect service configuration file (fabconnect.yaml)<a name="create_prepare_fabconnect_testnetworknanobash"></a>
Let's create in $HOME/ff-test the fabconnect service configuration file (must be one of .yml, .yaml, or .json):

```bash
touch $HOME/ff-test/fabconnect.yaml
```
Your `fabconnect.yaml` file after editing should look something like:
```yaml
maxinflight: 10
maxtxwaittime: 60
sendconcurrency: 25
receipts:
  maxdocs: 1000
  querylimit: 100
  retryinitialdelay: 5
  retrytimeout: 30
  leveldb:
    path: /Users/me/ff-test/receipts
events:
  webhooksAllowPrivateIPs: true
  leveldb:
    path: /Users/me/ff-test/events
http:
  port: 3000
rpc:
  useGatewayClient: true
  configpath: /Users/me/ff-test/ccp.yaml
```

#### Create and prepare your fabric connection profile (ccp.yaml)<a name="create_prepare_cpp_fabconnect_testnetworknanobash"></a>
Let's create in $HOME/ff-test the fabric connection profile (ccp.yaml):

```bash
touch $HOME/ff-test/ccp.yaml
```
Your `ccp.yaml` file after editing should look something like:

```yaml
# ***** certificateAuthorities section *****
# The test-network-nano-bash does not start the Fabric-CA node, but we must keep the 
# certificateAuthorities section because otherwise the fabconnect does not start
certificateAuthorities:
  ca.org1.example.com:
    tlsCACerts:
      path: /Users/me/fabric-samples/test-network-nano-bash/crypto-config/peerOrganizations/org1.example.com/ca/ca.org1.example.com-cert.pem
    url: https://org1.example.com:7054
    registrar:
      enrollId: admin
      enrollSecret: adminpw
    httpOptions:
      verify: false
channels:
  mychannel:
    peers:
      peer1.org1.com:
        chaincodeQuery: true
        endorsingPeer: true
        eventSource: true
        ledgerQuery: true
client:
  BCCSP:
    security:
      default:
        provider: SW
      enabled: true
      hashAlgorithm: SHA2
      level: 256
      softVerify: true
  credentialStore:
    cryptoStore:
      path: /Users/me/fabric-samples/test-network-nano-bash/crypto-config/peerOrganizations/org1.example.com/users
    path: /Users/me/fabric-samples/test-network-nano-bash/crypto-config/peerOrganizations/org1.example.com/users
  cryptoconfig:
    path: /Users/me/fabric-samples/test-network-nano-bash/crypto-config/peerOrganizations/org1.example.com/users
  logging:
    level: info
  organization: org1
orderers:
  orderer1:
    tlsCACerts:
      path: /Users/me/fabric-samples/test-network-nano-bash/crypto-config/ordererOrganizations/example.com/tlsca/tlsca.example.com-cert.pem
    url: orderer.example.com:6050
  orderer2:
    tlsCACerts:
      path: /Users/me/fabric-samples/test-network-nano-bash/crypto-config/ordererOrganizations/example.com/tlsca/tlsca.example.com-cert.pem
    url: orderer.example.com:6051
organizations:
  org1:
    certificateAuthorities:
    - ca.org1.example.com
    cryptoPath:  /Users/me/fabric-samples/test-network-nano-bash/crypto-config/peerOrganizations/org1.example.com/msp
    mspid: Org1MSP
    peers:
    - peer1.org1.com
peers:
  peer1.org1.com:
    tlsCACerts:
      path: /Users/me/fabric-samples/test-network-nano-bash/crypto-config/peerOrganizations/org1.example.com/tlsca/tlsca.org1.example.com-cert.pem
    url: peer0.org1.example.com:7051
version: 1.1.0%

entityMatchers:
  peer:
    - pattern: peer0.org1.example.(\w+)
      urlSubstitutionExp: localhost:7051
      sslTargetOverrideUrlSubstitutionExp: peer0.org1.example.com
      mappedHost: peer0.org1.example.com

    - pattern: (\w+).org1.example.(\w+):(\d+)
      urlSubstitutionExp: localhost:${2}
      sslTargetOverrideUrlSubstitutionExp: ${1}.org1.example.com
      mappedHost: ${1}.org1.example.com

    - pattern: (\w+):7051
      urlSubstitutionExp: localhost:7051
      sslTargetOverrideUrlSubstitutionExp: peer0.org1.example.com
      mappedHost: peer0.org1.example.com

  orderer:
    - pattern: (\w+).example.(\w+)
      urlSubstitutionExp: localhost:6050
      sslTargetOverrideUrlSubstitutionExp: orderer.example.com
      mappedHost: orderer.example.com

    - pattern: (\w+).example.(\w+):(\d+)
      urlSubstitutionExp: localhost:6050
      sslTargetOverrideUrlSubstitutionExp: orderer.example.com
      mappedHost: orderer.example.com

  certificateAuthority:
    - pattern: (\w+).org1.example.(\w+)
      urlSubstitutionExp: https://localhost:7054
      sslTargetOverrideUrlSubstitutionExp: org1.example.com
      mappedHost: ca.org1.example.com
```

###  Configure the identity (signer) <a name="fabconnect_configure_signer"></a>
Before connect fabconnect to test-network-nano-bash we must configure the identity (signer) that will be used to establish a connection with the blockchain network. The `test-network-nano-bash` does not bring up Fabric-CA nodes, so you have to use the `admin` or `user1` identity generated by the `generate_artifacts.sh` script.

We move to the path where the credentials are stored. The path is defined in the CCP file `"cpp.yaml"`, in the `client.credentialStore` section.
```bash
cd ~/fabric-samples/test-network-nano-bash/crypto-config/peerOrganizations/org1.example.com/users/
```

Copy the `Admin@org1.example.com-cert.pem` to the root directory of `/users` with the following format `user + @ + MSPID + "-cert.pem"`. It is the format used by fabric-sdk-go to store a user:
```bash
cp Admin@org1.example.com/msp/signcerts/Admin@org1.example.com-cert.pem ./admin@Org1MSP-cert.pem
```
Copy the `User1@org1.example.com-cert.pem` to the root directory of `/users` with the following format `user + @ + MSPID + "-cert.pem"` :
```bash
cp User1@org1.example.com/msp/signcerts/User1@org1.example.com-cert.pem ./user1@Org1MSP-cert.pem
```

Copy and rename each user's `priv_sk` private key to the `/users/keystore/` directory:

Admin private key:
```bash
mkdir -p ./keystore && cp Admin@org1.example.com/msp/keystore/priv_sk ./keystore/admin_sk
```

Private key of user1:
```bash
mkdir -p ./keystore && cp User1@org1.example.com/msp/keystore/priv_sk ./keystore/user1_sk
```

The structure should look like this:
```bash
me@208996:~/fabric-samples/test-network-nano-bash/crypto-config/peerOrganizations/org1.example.com/users$ ls -l

drwxr-xr-x 4 me me 4096 Jul 19 09:59 Admin@org1.example.com
-rw-rw-r-- 1 me me 810  Jul 19 10:22 admin@Org1MSP-cert.pem
drwxrwxr-x 2 me me 4096 Jul 19 10:28 keystore
drwxr-xr-x 4 me me 4096 Jul 19 09:59 User1@org1.example.com
-rw-rw-r-- 1 me me 810  Jul 19 10:23 user1@Org1MSP-cert.pem
```
### Launch the connector<a name="fabconnect_testnetworknanobash_launch_connector"></a>

Use the following command to launch the connector:
```bash
fabconnect -f "/Users/me/ff-test/fabconnect.yaml"
```
## Interacting with the asset-transfer-basic chaincode<a name="fabconnect_testnetwork_interacting_cc"></a>
- If you operate with the __test-network__ you can install the __asset-transfer-basic__ chaincode with the steps: [Install the asset-transfer-basic on test-network](https://hyperledger-fabric.readthedocs.io/en/latest/test_network.html#starting-a-chaincode-on-the-channel)
- If you operate with __test-network-nano-bash__ you can install the __asset-transfer-basic__ chaincode with the steps: [Install the asset-transfer-basic on test-network-nano-bash](https://github.com/hyperledger/fabric-samples/tree/main/test-network-nano-bash#instructions-for-deploying-and-running-the-basic-asset-transfer-sample-chaincode)

> **NOTE**: To interact with the fabconnect endpoints you can access the link http://__ip_address__:3000/api in your browser.

### InitLedger
Invoke the POST /transactions endpoint, with the following data:
```json
{
  "headers": {
    "type": "SendTransaction",
    "signer": "admin",
    "channel": "mychannel",
    "chaincode": "basic"
  },
  "func": "InitLedger",
  "args": [],
  "init": false
}
```
As a result, the client should respond with a JSON similar to this:
```json
{
  "headers": {
    "id": "cee5a0b8-e207-49c9-76a2-ca0d106fa139",
    "type": "TransactionSuccess",
    "timeReceived": "2022-07-18T06:00:37.323214822Z",
    "timeElapsed": 2.217562622,
    "requestOffset": "",
    "requestId": ""
  },
  "blockNumber": 6,
  "signerMSP": "Org1MSP",
  "signer": "admin",
  "transactionID": "44574737473df89f0183827f10ede4b5e99563ba44df6b7d48a49763d9179228",
  "status": "VALID"
}
```

### QueryAllAssets
Invokes the POST /query endpoint, with the following data:
```json
{
  "headers": {
    "signer": "admin",
    "channel": "mychannel",
    "chaincode": "basic"
  },
  "func": "GetAllAssets",
  "args": [],
  "strongread": true
}
```
As a result, the client should respond with a JSON similar to this:
```json
{
  "headers": {
    "channel": "mychannel",
    "timeReceived": "",
    "timeElapsed": 0,
    "requestOffset": "",
    "requestId": ""
  },
  "result": [
    {
      "AppraisedValue": 300,
      "Color": "blue",
      "ID": "asset1",
      "Owner": "Tomoko",
      "Size": 5
    },
    {
      "AppraisedValue": 400,
      "Color": "red",
      "ID": "asset2",
      "Owner": "Brad",
      "Size": 5
    },
    ...
  ]
}
 ```
## Documentation <a name="doc"></a>

- [FireFly Fabconnect - Github](https://github.com/hyperledger/firefly-fabconnect)
- [FireFly Cli - Github](https://github.com/hyperledger/firefly-cli/)
- [Firefly - Doc](https://hyperledger.github.io/firefly/)

## Troubleshooting <a name="troubleshooting"></a>
