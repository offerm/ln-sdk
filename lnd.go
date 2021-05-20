package lnsdk

import (
	"fmt"
	"github.com/lightningnetwork/lnd/macaroons"
	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/macaroon.v2"
	"io/ioutil"
	"github.com/offerm/lnsdk/utils"
	"os"
	"path/filepath"
)

const (
	defaultTLSCertFilename  = "tls.cert"
	defaultMacaroonFilename = "admin.macaroon"
	defaultRpcPort          = "1231"
	defaultRpcHostPort      = "localhost:" + defaultRpcPort
)

var (
	//tls        = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
	//certFile   = flag.String("cert_file", "", "The TLS cert file")
	//keyFile    = flag.String("key_file", "", "The TLS key file")
	//port       = flag.Int("port", 10000, "The server port")

	//Commit stores the current commit hash of this build. This should be
	//set using -ldflags during compilation.
	//defaultLndDir       = btcutil.AppDataDir("lnd", false)
	defaultLndDir       = "/Users/offerm/lnd/lndx"
	defaultTLSCertPath  = filepath.Join(defaultLndDir, defaultTLSCertFilename)
	defaultMacaroonPath = filepath.Join(defaultLndDir, "data/chain/bitcoin/testnet", defaultMacaroonFilename)
)

//type Lnd struct {
//
//}

//func NewLnd(host string, port int, network string, implementation string,
//		agent utils.Agent, wg *sync.WaitGroup) (*Lnd, error){
//	if network != utils.MainNet && network != utils.SimNet{
//		return nil, fmt.Errorf("network %v is not supported",network)
//	}
//	if implementation != utils.LND{
//		return nil, fmt.Errorf("implementation %v is not supported",implementation)
//	}
//	return &Lnd{
//		host: host,
//		port: port,
//		network: network,
//		implementation: implementation,
//		callBack: agent,
//		wg: wg,
//	}, nil
//}

func GetLNDClientConn(ctx *cli.Context) *grpc.ClientConn {

	defaultLndDir, _ = homedir.Expand(ctx.String(utils.LndDirFlag.Name))
	RpcHostPort := fmt.Sprintf("%v:%v", ctx.String(utils.LnHostFlag.Name), ctx.Int(utils.LnPortFlag.Name))

	defaultTLSCertPath = filepath.Join(defaultLndDir, defaultTLSCertFilename)
	defaultMacaroonPath = filepath.Join(defaultLndDir,
		"data/chain/bitcoin",
		ctx.String(utils.NetworkFlag.Name),
		defaultMacaroonFilename,
	)

	creds, err := credentials.NewClientTLSFromFile(defaultTLSCertPath, "")
	if err != nil {
		fatal(err)
	}

	macaroonBytes, err := ioutil.ReadFile(defaultMacaroonPath)
	if err != nil {
		log.Fatal("Cannot read macaroon file", err)
	}

	mac := &macaroon.Macaroon{}
	if err = mac.UnmarshalBinary(macaroonBytes); err != nil {
		log.Fatal("Cannot unmarshal macaroon", err)
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(100 * 1024 * 1024)),
		grpc.WithPerRPCCredentials(macaroons.NewMacaroonCredential(mac)),
	}

	conn, err := grpc.Dial(RpcHostPort, opts...)
	if err != nil {
		fatal(err)
	}

	return conn
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "[lncli] %v\n", err)
	os.Exit(1)
}
