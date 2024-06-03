package main

import (
	"errors"
	"flag"
	"github.com/ginchuco/ginchu/lib"
	"github.com/ginchuco/ginchu/lib/crypto"
	"os"
	"path/filepath"
)

var dataDir = flag.String("data-dir", lib.DefaultConfig().DataDirPath, "")

type Config struct {
	RPCUrl                     string                       `json:"rpc_url"`
	RPCPort                    string                       `json:"rpc_port"`
	AdminRPCPort               string                       `json:"admin_rpc_port"`
	PrivateKeys                []*crypto.BLS12381PrivateKey `json:"private_keys"`
	PercentInvalidTransactions int                          `json:"percent_invalid_transactions"`
}

func DefaultConfig() *Config {
	configBz, err := os.ReadFile(filepath.Join(*dataDir, lib.ConfigFilePath))
	if err != nil {
		panic(err)
	}
	config := new(lib.Config)
	if err = lib.UnmarshalJSON(configBz, config); err != nil {
		if err != nil {
			panic(err)
		}
	}
	valKey, err := crypto.NewBLSPrivateKeyFromFile(filepath.Join(*dataDir, lib.ValKeyPath))
	if err != nil {
		panic(err)
	}
	privateKeys := make([]*crypto.BLS12381PrivateKey, 5)
	for i := 0; i < 5; i++ {
		if i == 0 {
			privateKeys[i] = valKey.(*crypto.BLS12381PrivateKey)
			continue
		}
		pk, _ := crypto.NewBLSPrivateKey()
		privateKeys[i] = pk.(*crypto.BLS12381PrivateKey)
	}
	return &Config{
		RPCUrl:                     localhost,
		RPCPort:                    config.RPCPort,
		AdminRPCPort:               config.AdminPort,
		PrivateKeys:                privateKeys,
		PercentInvalidTransactions: 10,
	}
}

func (c *Config) FromFile(l lib.LoggerI) *Config {
	configFilePath := filepath.Join(*dataDir, configFileName)
	l.Infof("Reading data directory at %s", *dataDir)
	if err := os.MkdirAll(*dataDir, os.ModePerm); err != nil {
		l.Fatal(err.Error())
	}
	if _, err := os.Stat(configFilePath); errors.Is(err, os.ErrNotExist) {
		l.Infof("Creating %s file", configFilePath)
		if err = c.WriteToFile(configFilePath); err != nil {
			l.Fatal(err.Error())
		}
	}
	l.Infof("Reading config file at %s", configFilePath)
	bz, err := os.ReadFile(configFilePath)
	if err != nil {
		l.Fatal(err.Error())
	}
	if err = lib.UnmarshalJSON(bz, c); err != nil {
		l.Fatal(err.Error())
	}
	if len(c.PrivateKeys) == 0 {
		l.Fatalf("no private keys are in the config file: %s", configFilePath)
	}
	return c
}

func (c *Config) WriteToFile(filepath string) lib.ErrorI {
	bz, err := lib.MarshalJSONIndent(c)
	if err != nil {
		return err
	}
	if er := os.WriteFile(filepath, bz, os.ModePerm); er != nil {
		return lib.ErrWriteFile(er)
	}
	return nil
}
