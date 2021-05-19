package utils

import "github.com/urfave/cli/v2"

const LND = "lnd"
const MainNet = "mainnet"
const SimNet = "simnet"

var (
	LnHostFlag = &cli.StringFlag{
		Name:    "ln-host",
		Usage:   "host name/ip of the lightning node",
		Aliases: []string{"lnh"},
		Value:   "localhost",
	}
	LnPortFlag = &cli.IntFlag{
		Name:    "ln-port",
		Usage:   "port of the lightning node",
		Aliases: []string{"lnp"},
		Value:   10009,
	}
	NetworkFlag = &cli.StringFlag{
		Name:    "network",
		Usage:   "lightning environment (mainnet, testnet, simnet)",
		Aliases: []string{"n"},
		Value:   "mainnet",
	}
	LndDirFlag = &cli.StringFlag{
		Name:     "lnd-dir",
		Usage:    "path to lnd directory",
		Required: true,
	}
)
