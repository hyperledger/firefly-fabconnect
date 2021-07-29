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

### Makefile

Firefly-fabconnect is equipped with a `Makefile` to simplify some tasks.
Here is the initial list of available commands:

- `make all`: code build, test, go-mod-tidy.
- `test`: execute the unit-tests.
- `make clean`: clean the docker environment, useful for testing.
- `go-mod-tidy`: dependencies sync (pull, install).
- `build`: code compile to executable binary format.

Executes the above from `$GOPATH/github.com/hyperledger-labs/firefly-fabconnect`.

### License
This project is licensed under the Apache 2 License - see the [`LICENSE`](LICENSE) file for details.
