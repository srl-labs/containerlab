package tc

import (
	"net"
)

// ipToUint32 converts a legacy ip object to its uint32 representative.
// For IPv6 addresses it returns ErrInvalidArg.
func ipToUint32(ip net.IP) (uint32, error) {
	tmp := ip.To4()
	if tmp == nil {
		return 0, ErrInvalidArg
	}
	return nativeEndian.Uint32(tmp), nil
}

// uint32ToIP converts a legacy ip to a net.IP object.
func uint32ToIP(ip uint32) net.IP {
	netIP := make(net.IP, 4)
	nativeEndian.PutUint32(netIP, ip)
	return netIP
}

// bytesToIP converts a slice of bytes into a net.IP object.
func bytesToIP(ip []byte) (net.IP, error) {
	if len(ip) != net.IPv4len && len(ip) != net.IPv6len {
		return nil, ErrInvalidArg
	}
	return net.IP(ip), nil
}

// ipToBytes casts a ip object into its byte slice representative.
func ipToBytes(ip net.IP) []byte {
	return []byte(ip)
}

// bytesToHardwareAddr converts a slice of bytes into a net.HardwareAddr object.
func bytesToHardwareAddr(mac []byte) net.HardwareAddr {
	return net.HardwareAddr(mac[:])
}

// hardwareAddrToBytes casts a net.HardwareAddr object into its byte slice representative.
func hardwareAddrToBytes(mac net.HardwareAddr) []byte {
	return []byte(mac)
}

func endianSwapUint16(in uint16) uint16 {
	return (in << 8) | (in >> 8)
}

func endianSwapUint32(in uint32) uint32 {
	return ((in & 0x000000FF) << 24) |
		((in & 0x0000FF00) << 8) |
		((in & 0x00FF0000) >> 8) |
		((in & 0xFF000000) >> 24)
}

// bytesToInt32 converts an array for four bytes in big endian format to an integer.
func bytesToInt32(b []byte) int32 {
	if len(b) != 4 {
		return 0
	}
	return int32(b[0])<<24 | int32(b[1])<<16 | int32(b[2])<<8 | int32(b[3])
}
