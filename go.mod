module github.com/hyperledger/firefly-fabconnect

go 1.16

require (
	github.com/Shopify/sarama v1.29.1
	github.com/frankban/quicktest v1.14.0 // indirect
	github.com/globalsign/mgo v0.0.0-20181015135952-eeefdecb41b8
	github.com/golang/protobuf v1.5.2
	github.com/google/certificate-transparency-go v1.1.1 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/gorilla/websocket v1.4.2
	github.com/hashicorp/golang-lru v0.5.4
	github.com/hyperledger/fabric-config v0.0.7 // indirect
	github.com/hyperledger/fabric-protos-go v0.0.0-20201028172056-a3136dde2354
	github.com/hyperledger/fabric-sdk-go v1.0.1-0.20210729165856-3be4ed253dcf
	github.com/julienschmidt/httprouter v1.3.0
	github.com/klauspost/compress v1.13.5 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/oklog/ulid/v2 v2.0.2
	github.com/olekukonko/tablewriter v0.0.5 // indirect
	github.com/onsi/gomega v1.17.0 // indirect
	github.com/otiai10/copy v1.6.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/procfs v0.6.0 // indirect
	github.com/rogpeppe/go-internal v1.8.1 // indirect
	github.com/rs/cors v1.8.2
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.3.0
	github.com/spf13/viper v1.10.1
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/stretchr/testify v1.7.1-0.20210116013205-6990a05d54c2
	github.com/syndtr/goleveldb v1.0.1-0.20210305035536-64b5b1c73954
	github.com/urfave/cli v1.22.2 // indirect
	github.com/x-cray/logrus-prefixed-formatter v0.5.2
	github.com/xeipuuv/gojsonschema v1.2.0
	go.etcd.io/bbolt v1.3.5 // indirect
	go.etcd.io/etcd v0.5.0-alpha.5.0.20200910180754-dd1b699fc489 // indirect
	golang.org/x/crypto v0.0.0-20220112180741-5e0467b6c7ce // indirect
	golang.org/x/sys v0.0.0-20211216021012-1d35b9e2eb4e // indirect
	golang.org/x/term v0.0.0-20210615171337-6886f2dfbf5b // indirect
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e // indirect
	golang.org/x/tools v0.1.9-0.20211228192929-ee1ca4ffc4da // indirect
	gopkg.in/yaml.v2 v2.4.0
	sigs.k8s.io/yaml v1.2.0 // indirect
)

replace google.golang.org/grpc => google.golang.org/grpc v1.29.0

replace github.com/hyperledger/fabric-sdk-go => github.com/kaleido-io/fabric-sdk-go v1.0.1-0.20210805185536-5908d6234be8
