package main

type Options struct {
	ClientAddr     string `mapstructure:"client-listen"`
	PeerAddr       string `mapstructure:"peer-listen"`
	ServerNodeAddr string `mapstructure:"self-node"`
	RedisAddr      string `mapstructure:"self-redis-address"`
	LedgerPath     string `mapstructure:"self-ledger-path"`
	SelfID         string `mapstructure:"self-id"`
	Replicas       []Replica
}

type Replica struct {
	ID         string
	PeerAddr   string `mapstructure:"peer-address"`
	ClientAddr string `mapstructure:"client-address"`
	RedisAddr  string `mapstructure:"redis-address"`
	LedgerPath string `mapstructure:"ledger-path"`
}
