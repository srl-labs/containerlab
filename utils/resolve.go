package utils

import (
	"bufio"
	"io/fs"
	"net"
	"strings"

	log "github.com/sirupsen/logrus"
)

// ExtractDNSServersFromResolvConf extracts IP addresses
// of the DNS servers from the resolv.conf-formatted files passed in filenames list.
// Returns a list of IP addresses of the DNS servers.
func ExtractDNSServersFromResolvConf(filesys fs.FS, filenames []string) ([]string, error) {
	DNSServersMap := map[string]struct{}{}

	for _, filename := range filenames {
		readFile, err := filesys.Open(filename)
		if err != nil {
			log.Debugf("Error opening host DNS config %s: %v", filename, err)
			continue
		}

		fileScanner := bufio.NewScanner(readFile)
		fileScanner.Split(bufio.ScanLines)

		// check line by line for a match
		for fileScanner.Scan() {
			line := strings.TrimSpace(fileScanner.Text())
			if strings.HasPrefix(line, "nameserver") {
				fields := strings.Fields(line)
				if len(fields) != 2 {
					continue
				}

				ip := net.ParseIP(fields[1])
				if ip == nil || ip.IsLoopback() {
					continue
				}

				DNSServersMap[ip.String()] = struct{}{}
			}
		}

		readFile.Close()
	}

	if len(DNSServersMap) == 0 {
		return nil, nil
	}

	DNSServers := make([]string, 0, len(DNSServersMap))
	var count int
	for k := range DNSServersMap {
		if count == 3 {
			// keep only the first three DNS servers
			// since C DNS resolver can't handle more
			break
		}
		DNSServers = append(DNSServers, k)
		count++
	}

	return DNSServers, nil
}
