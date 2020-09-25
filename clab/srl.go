package clab

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
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

	x := strconv.FormatInt(int64(index), 16)
	d2 := fmt.Sprintf("%02s", x)
	m := "00:01:" + strings.ToUpper(d2) + ":00:00:00"
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
