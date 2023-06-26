package utils

import (
	"regexp"

	"github.com/containernetworking/plugins/pkg/ns"
)

func foo(nsPath string) error {
	netNamespace, err := ns.GetNS(nsPath)
	if err != nil {
		return err
	}
	_ = netNamespace
	return nil
}

func pidFromNSPath(ns string) int {
	re := regexp.MustCompile(`.*/(?P<pid>\d+)/ns/net$`)
	matches := re.FindStringSubmatch(ns)
	result := make(map[string]string)
	for i, name := range re.SubexpNames() {
		if i != 0 && name != "" {
			result[name] = matches[i]
		}
	}
	return 0
}
