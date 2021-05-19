module github.com/offerm/ln-sdk

go 1.15

replace google.golang.org/grpc => google.golang.org/grpc v1.26.0

require (
	github.com/lightningnetwork/lnd v0.12.0-beta
	github.com/mitchellh/go-homedir v1.1.0
	github.com/sirupsen/logrus v1.8.1
	github.com/urfave/cli/v2 v2.3.0
	google.golang.org/grpc v1.24.0
	gopkg.in/macaroon.v2 v2.1.0
)
