package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/utils"
)

func main() {
	err := utils.SetDelay("/proc/220224/ns/net", "eth1", 50000000)
	if err != nil {
		log.Error(err)
	}
	err = utils.SetJitter("/proc/220224/ns/net", "eth1", 7000000)
	if err != nil {
		log.Error(err)
	}
}
