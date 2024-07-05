package virt

import (
	"bufio"
	"os"
	"strings"

	"github.com/klauspost/cpuid/v2"
	log "github.com/sirupsen/logrus"
)

// VerifySSSE3Support check if SSSE3 is supported on the host.
func VerifySSSE3Support() bool {
	return cpuid.CPU.Has(cpuid.SSSE3)
}

// VerifyVirtSupport checks if virtualization is supported by a cpu in case topology contains VM-based nodes
// when clab itself is being invoked as a container, this check is bypassed.
func VerifyVirtSupport() bool {
	// check if we are being executed in a container environment
	// in that case we skip this check as there are no convenient ways to interrogate hosts capabilities
	// check if /proc/2 exists, and if it does, check if the name of the proc is kthreadd
	// otherwise it is a container env

	f, err := os.Open("/proc/2/status")
	if err != nil {
		log.Debug("/proc/2/status file was not found. This means we run in a container and no virt checks are possible")
		return true
	}
	defer f.Close() // skipcq: GO-S2307

	// read first line of a /proc/2/status file to check if it contains kthreadd
	// if it doesn't, we are in a container
	scanner := bufio.NewScanner(f)

	scanner.Scan()
	if !strings.Contains(scanner.Text(), "kthreadd") {
		log.Debug("/proc/2/status first line doesn't contain kthreadd. This means we run in a container and no virt checks are possible")
		return true
	}

	f, err = os.Open("/proc/cpuinfo")
	if err != nil {
		log.Debugf("Error checking VirtSupport: %v", err)
		return false
	}
	defer f.Close() // skipcq: GO-S2307

	scanner = bufio.NewScanner(f)

	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "vmx") ||
			strings.Contains(scanner.Text(), "svm") {

			log.Debug("virtualization support found")

			return true
		}
	}

	if err := scanner.Err(); err != nil {
		return false
	}

	err = f.Sync()
	if err != nil {
		return false
	}

	return false
}
