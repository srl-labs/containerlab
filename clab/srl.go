package clab

import (
	"crypto/rand"
	"fmt"
	"os"
	"path"
	"text/template"

	log "github.com/sirupsen/logrus"
)

type mac struct {
	MAC string
}

func generateSRLTopologyFile(src, labDir string, index int) error {
	dst := path.Join(labDir, "topology.yml")
	tpl, err := template.ParseFiles(src)
	if err != nil {
		return err
	}

	// generate random bytes to use in the 2-3rd bytes of a base mac
	// this ensures that different srl nodes will have different macs for their ports
	buf := make([]byte, 2)
	_, err = rand.Read(buf)
	if err != nil {
		return err
	}
	m := fmt.Sprintf("02:%02x:%02x:00:00:00", buf[0], buf[1])

	mac := mac{
		MAC: m,
	}
	log.Debug(mac, dst)
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	if err = tpl.Execute(f, mac); err != nil {
		panic(err)
	}
	log.Debugf("CopyFile GoTemplate src %s -> dat %s succeeded\n", src, dst)
	return nil
}
