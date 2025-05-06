package config

import (
	"path/filepath"
)

const (
	DefaultLogLevel       = "info"
	DefaultAltunDir       = ".altun"
	DefaultConfigDir      = "config"
	DefaultDataDir        = "data"
	DefaultConfigFileName = "config.toml"
	DefaultInitVersion    = "0"
)

var (
	defaultConfigFilePath = filepath.Join(DefaultConfigDir, DefaultConfigFileName)
)

type Config struct {
	BaseConfig `mapstructure:",squash"`
	// Consensus *ConsensusConfig `mapstructure:"consensus"`
	// Storage   *StorageConfig   `mapstructure:"storage"`
}

type BaseConfig struct {
	Version   string `mapstructure:"version"`
	RootDir   string `mapstructure:"home"`
	DBPath    string `mapstructure:"db_dir"`
	DBBackend string `mapstructure:"db_backend"` // Database backend: goleveldb | rocksdb | badgerdb | pebbledb
}

func DefaultBaseConfig() BaseConfig {
	return BaseConfig{
		Version:   DefaultInitVersion,
		DBBackend: "goleveldb",
		DBPath:    DefaultDataDir,
	}
}

func (cfg BaseConfig) DBDir() string {
	return rootify(cfg.DBPath, cfg.RootDir)
}

func rootify(path, root string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}
