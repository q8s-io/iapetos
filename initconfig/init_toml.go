package initconfig

import (
	"log"

	"github.com/BurntSushi/toml"
)

var StatefulPodResourceCfg Config

type Config struct {
	Node NodeConfig `toml:"nodeConfig"`
	Pod  PodConfig  `toml:"podConfig"`
}

type PodConfig struct {
	Timeout int `toml:"timeout"`
}

type NodeConfig struct {
	Timeout int `toml:"timeout"`
}

func init() {
	filePath := "./config/node_lost_connection/config.toml"
	if _, err := toml.DecodeFile(filePath, &StatefulPodResourceCfg); err != nil {
		log.Fatal("initconfig config error: ", err)
	}
}
