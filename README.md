# firefly-fabconnect
A reliable REST and websocket API to interact with a Fabric network and stream events.

## Architecture

![high level architecture](/images/arch-1.jpg)

The component provides 3 high level sets of API endpoints:
- Client MSPs (aka the wallet): registering and enrolling identities to be used for signing transactions
- Transactions: submit transactions and query for transaction result/receipts
- Events: subscribe to events with regex based filter and stream to the client app via websocket

