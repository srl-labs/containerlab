package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	cniBin = "/opt/cni/bin"
)

// GetCanonicalImageName produces a canonical image name.
// if the input name did not specify a tag, the implicit "latest" tag is returned.
func GetCanonicalImageName(imageName string) string {
	// name transformation rules
	//    alpine == docker.io/library/alpine:latest
	//    foo/bar == docker.io/foo/bar:latest
	//    foo.bar/baz == foo.bar/bar:latest
	//    localhost/foo:bar == localhost/foo:bar
	//    docker.elastic.co/elasticsearch/elasticsearch == docker.elastic.co/elasticsearch/elasticsearch:latest
	canonicalImageName := imageName
	slashCount := strings.Count(imageName, "/")

	switch slashCount {
	case 0:
		canonicalImageName = "docker.io/library/" + imageName
	case 1:
		// split on slash to get first element of the name
		nameSplit := strings.Split(imageName, "/")
		// case of foo.bar/baz
		if strings.Contains(nameSplit[0], ".") {
			canonicalImageName = imageName
		} else if strings.Contains(nameSplit[0], "localhost") {
			// case of localhost/foo:bar - podman prefixes local images with "localhost"
			canonicalImageName = imageName
		} else {
			canonicalImageName = "docker.io/" + imageName
		}
	}
	// append latest tag if no tag was provided
	if !strings.Contains(canonicalImageName, ":") {
		canonicalImageName = canonicalImageName + ":latest"
	}

	return canonicalImageName
}

func GetCNIBinaryPath() string {
	var cniPath string
	var ok bool
	if cniPath, ok = os.LookupEnv("CNI_BIN"); !ok {
		cniPath = cniBin
	}
	return cniPath
}

// ContainerNSToPID resolves the name of a container via
// the "/run/netns/<CONTAINERNAME>" to its PID.
func ContainerNSToPID(cID string) (int, error) {
	pnns, err := filepath.EvalSymlinks("/run/netns/" + cID)
	if err != nil {
		return 0, err
	}
	pathElem := strings.Split(pnns, "/")
	if len(pathElem) != 4 {
		return 0, fmt.Errorf("unexpected result looking up container PID")
	}
	pid, err := strconv.Atoi(pathElem[1])
	if err != nil {
		return 0, fmt.Errorf("error converting the string part of the namespace link to int")
	}
	return pid, nil
}
