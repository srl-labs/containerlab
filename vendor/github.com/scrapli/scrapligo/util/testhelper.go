package util

import "strings"

// PlatformOK is only used for testing and checks if the cli flag platforms is either "all" or
// contains the requested platform name platformName.
func PlatformOK(platforms *string, platformName string) bool {
	if *platforms == All {
		return true
	}

	p := strings.Split(*platforms, ",")

	return StringSliceContains(p, platformName)
}

// TransportOK is only used for testing and checks if the cli flag transports is either "all" or
// contains the requested transport name transportName.
func TransportOK(transports *string, transportName string) bool {
	if *transports == All {
		return true
	}

	t := strings.Split(*transports, ",")

	return StringSliceContains(t, transportName)
}
