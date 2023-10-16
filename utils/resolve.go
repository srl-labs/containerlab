package utils

import (
	"bufio"
	"os"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

func ExtractDNSServerFromResolvConf(filenames []string) ([]string, error) {
	DNSServersMap := map[string]struct{}{}

	for _, filename := range filenames {
		readFile, err := os.Open(filename)
		defer readFile.Close()
		if err != nil {
			log.Debugf("Error opening host DNS config %s: %v", filename, err)
			continue
		}
		fileScanner := bufio.NewScanner(readFile)
		fileScanner.Split(bufio.ScanLines)
		ipPattern := `\s*nameserver\s+((\d{1,3}\.){3}\d{1,3})`
		// Compile the regular expression.
		re := regexp.MustCompile(ipPattern)

		// check line by line for a match
		for fileScanner.Scan() {
			if match := re.FindStringSubmatch(fileScanner.Text()); match != nil {
				// skip 127.x.y.z addresses
				if strings.HasPrefix(match[1], "127") {
					continue
				}
				DNSServersMap[match[1]] = struct{}{}
			}
		}
	}

	// if we've not found any DNS Servers we return
	if len(DNSServersMap) == 0 {
		return nil, nil
	}

	// convert the map into a slice
	DNSServers := make([]string, 0, len(DNSServersMap))
	for k, _ := range DNSServersMap {
		DNSServers = append(DNSServers, k)
	}
	return DNSServers, nil
}
