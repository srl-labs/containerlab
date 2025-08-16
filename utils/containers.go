package utils

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
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

		switch {
		case strings.Contains(nameSplit[0], "."):
			canonicalImageName = imageName
		case strings.Contains(nameSplit[0], "localhost"):
			// case of localhost/foo:bar - podman prefixes local images with "localhost"
			canonicalImageName = imageName
		default:
			canonicalImageName = "docker.io/" + imageName
		}
	}
	// append latest tag if no tag was provided
	if !strings.Contains(canonicalImageName, ":") {
		canonicalImageName += ":latest"
	}

	return canonicalImageName
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

// DestinationBindMountExists checks if a bind mount destination exists in a list of bind mounts.
// The bind options are not matched, only the destination is matched.
// The binds are expected to be in the format of "source:destination[:options]".
func DestinationBindMountExists(binds []string, dest string) bool {
	for _, b := range binds {
		parts := strings.Split(b, ":")
		if len(parts) >= 2 {
			// The destination is the second part
			if parts[1] == dest {
				return true
			}
		}
	}
	return false
}

// ContainerNameFromNetworkMode takes the NetworkMode config string and returns the container name
// from it.
func ContainerNameFromNetworkMode(s string) (string, error) {
	after, found := strings.CutPrefix(s, "container:")
	if !found {
		return "", fmt.Errorf("%s not a valid container reference", s)
	}
	return after, nil
}
