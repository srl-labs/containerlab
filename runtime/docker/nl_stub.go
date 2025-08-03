//go:build !linux
// +build !linux

package docker

func getIPv4Family() int {
    return 2 // syscall.AF_INET; safe fallback if you just want to compile
}
