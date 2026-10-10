package labruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const (
	// kubernetesNameMaxLen is the maximum length of a DNS label, the form the object names a lab
	// runtime derives from containerlab names have to take.
	kubernetesNameMaxLen = 63
	// nameDigestLen is the number of digest characters appended to a name that had to be
	// truncated to fit kubernetesNameMaxLen.
	nameDigestLen = 7
	// leadingLetterPrefix prefixes a name that would otherwise start with a digit or a dash,
	// which a DNS-1035 label cannot do.
	leadingLetterPrefix = "clab-"
)

// nonKubernetesNameChars matches every run of characters a DNS label cannot carry, once the name
// has been lower-cased.
var nonKubernetesNameChars = regexp.MustCompile(`[^a-z0-9-]+`)

// SanitizeName maps a containerlab name onto the DNS-1035 label a lab runtime can name the objects
// it creates with: lower case, made up of a-z, 0-9 and '-', starting with a letter and at most 63
// characters long. A large share of public labs name their nodes R1/PE_1 or capitalize the lab
// name, and Kubernetes cannot carry those names as they are -- renaming them beats making the user
// edit every reference in the topology.
//
// The mapping is deterministic and idempotent: a name Kubernetes already accepts maps onto itself,
// so a sanitized name coming back from the runtime -- or typed by a user reading it off
// containerlab output -- resolves to the same object. The result is empty only for a name holding
// nothing a Kubernetes name can be built from.
func SanitizeName(name string) string {
	sanitized := strings.Trim(
		nonKubernetesNameChars.ReplaceAllString(strings.ToLower(name), "-"),
		"-",
	)
	if sanitized == "" {
		return ""
	}

	if sanitized[0] < 'a' || sanitized[0] > 'z' {
		sanitized = leadingLetterPrefix + sanitized
	}

	if len(sanitized) > kubernetesNameMaxLen {
		digest := sha256.Sum256([]byte(name))
		sanitized = strings.TrimRight(
			sanitized[:kubernetesNameMaxLen-nameDigestLen-1],
			"-",
		) + "-" + hex.EncodeToString(digest[:])[:nameDigestLen]
	}

	return sanitized
}

// SanitizeNodeNames returns the sanitized name of every node name that needs one, keyed by the name
// the topology uses. Node names Kubernetes can carry as they are are absent from the result.
//
// Two node names that differ only in something Kubernetes cannot carry (R1 and r1) would collapse
// onto one object, so that is reported as an error instead of silently merging the two nodes.
func SanitizeNodeNames(nodeNames []string) (map[string]string, error) {
	renames := map[string]string{}
	origins := make(map[string]string, len(nodeNames))

	sorted := slices.Clone(nodeNames)
	slices.Sort(sorted)

	for _, nodeName := range sorted {
		sanitized := SanitizeName(nodeName)
		if sanitized == "" {
			return nil, fmt.Errorf(
				"node name %q holds no character a Kubernetes object name can be built from",
				nodeName,
			)
		}

		if origin, taken := origins[sanitized]; taken {
			return nil, fmt.Errorf(
				"node names %q and %q both map onto the Kubernetes name %q; rename one of them",
				origin,
				nodeName,
				sanitized,
			)
		}
		origins[sanitized] = nodeName

		if sanitized != nodeName {
			renames[nodeName] = sanitized
		}
	}

	return renames, nil
}
