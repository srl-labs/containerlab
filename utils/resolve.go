package utils

import (
	"bufio"
	"io/fs"
	"net"
	"strings"

	"github.com/charmbracelet/log"
	"golang.org/x/exp/slices"
)

// ExtractDNSServersFromResolvConf extracts IP addresses
// of the DNS servers from the resolv.conf-formatted files passed in filenames list.
// Returns a list of IP addresses of the DNS servers.
func ExtractDNSServersFromResolvConf(filesys fs.FS, filenames []string) ([]string, error) {
	// list of DNS servers up to 10 elements
	// since realistically there should be no more than 3
	// DNS servers, we leave some room for duplicates
	DNSServers := make([]string, 0, 10)

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

				DNSServers = append(DNSServers, ip.String())
			}
		}

		readFile.Close()
	}

	if len(DNSServers) == 0 {
		return nil, nil
	}

	// remove duplicates
	slices.Sort(DNSServers)
	DNSServers = slices.Compact(DNSServers)

	if len(DNSServers) > 3 {
		DNSServers = DNSServers[:3]
	}

	return DNSServers, nil
}
