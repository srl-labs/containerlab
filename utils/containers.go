package utils

import "strings"

// produces a canonical image name.
// returns the canonical image name including the tag
// if the input name did not specify a tag, the implicit "latest" tag is returned.
func GetCanonicalImageName(imageName string) string {
	// might need canonical name e.g.
	//    -> alpine == docker.io/library/alpine
	//    -> foo/bar == docker.io/foo/bar
	//    -> foo.bar/baz == foo.bar/bar
	//    -> docker.elastic.co/elasticsearch/elasticsearch == docker.elastic.co/elasticsearch/elasticsearch
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
		} else {
			canonicalImageName = "docker.io/" + imageName
		}
	}
	tag := "latest"
	colonSplit := strings.Split(imageName, ":")
	if len(colonSplit) == 2 {
		canonicalImageName = colonSplit[0]
		tag = colonSplit[1]
	}
	return canonicalImageName + ":" + tag
}
