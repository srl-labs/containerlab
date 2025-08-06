package hostnet
import (
	"fmt"
	osexec "os/exec"
	"strings"
	"net"
	"regexp"

	"github.com/charmbracelet/log"
	"github.com/vishvananda/netlink"
)

// MacvlanConfig contains all the configuration needed for macvlan operations
type MacvlanConfig struct {
	NetworkName    string
	ParentIface    string
	MacvlanMode    string
	AuxAddress     string  // Can be IP or IP/CIDR
	IPv4Subnet     string  // The main macvlan network subnet
}

// PostCreateMacvlanActions performs macvlan-specific post-creation actions
func PostCreateMacvlanActions(cfg *MacvlanConfig) error {
	log.Info("Starting macvlan post-creation actions")
	log.Debugf("AuxAddress: %s, IPv4Subnet: %s", cfg.AuxAddress, cfg.IPv4Subnet)
	
	// 1. Verify parent interface exists and is UP
	parentLink, err := netlink.LinkByName(cfg.ParentIface)
	if err != nil {
		return fmt.Errorf("failed to get parent interface %s: %w", cfg.ParentIface, err)
	}
	
	// Check if interface is UP
	if parentLink.Attrs().OperState != netlink.OperUp {
		log.Warnf("Parent interface %s is not UP (state: %s), containers may not have connectivity", 
			cfg.ParentIface, parentLink.Attrs().OperState)
	}
	
	// 2. Check promiscuous mode
	if parentLink.Attrs().Promisc == 0 {
		log.Debugf("Parent interface %s is not in promiscuous mode, enabling it for better macvlan compatibility", 
			cfg.ParentIface)
		if err := EnablePromiscuousMode(cfg.ParentIface); err != nil {
			log.Warnf("failed to enable promiscuous mode on %s: %v", cfg.ParentIface, err)
		}
	}
	
	// 3. Log MTU information
	parentMTU := parentLink.Attrs().MTU
	log.Debugf("Parent interface %s has MTU %d, macvlan interfaces will inherit this", 
		cfg.ParentIface, parentMTU)
	
	// 4. Create host macvlan interface if aux address is specified
    if cfg.AuxAddress != "" {
        // Check for potential subnet conflicts
        if err := checkSubnetConflicts(cfg); err != nil {
            log.Warnf("Subnet configuration warning: %v", err)
            log.Info("")
            log.Info("=== MACVLAN SUBNET CONFIGURATION GUIDANCE ===")
            log.Info("When the macvlan subnet matches your host's subnet, you have three options:")
            log.Info("")
            log.Info("Option 1: Use a smaller subnet for container routes")
            log.Info("  - If host is on 192.168.1.0/24, use a /26 or /27 for containers")
            log.Info("  - Example: ipv4-subnet: 192.168.1.128/26")
            log.Info("  - This allows 62 container IPs while avoiding route conflicts")
            log.Info("")
            log.Info("Option 2: Use a different subnet with proper routing")
            log.Info("  - Use a completely different subnet (e.g., 10.100.0.0/24)")
            log.Info("  - Configure routing on your network to reach this subnet")
            log.Info("  - Containers won't be on the same L2 segment as other devices")
            log.Info("")
            log.Info("Option 3: Accept no host-to-container connectivity")
            log.Info("  - Don't set macvlan-aux (no host interface)")
            log.Info("  - Containers can reach external networks")
            log.Info("  - Host cannot directly communicate with containers")
            log.Info("=============================================")
            log.Info("")
        }
        
        if err := CreateHostMacvlanInterface(cfg); err != nil {
            // Don't fail the entire operation, just warn
            log.Warnf("Failed to create host macvlan interface: %v", err)
            // ... rest of manual instructions ...
        } else {
            log.Infof("Created host macvlan interface %shost with IP %s", 
                cfg.NetworkName, cfg.AuxAddress)
        }
    } else {
        // Still warn about the limitation
        log.Info("Note: Host cannot directly communicate with macvlan containers due to kernel limitations. " +
            "Consider setting 'macvlan-aux' to create a host interface.")
    }
	
	return nil
}

// parseAuxAddress extracts IP and subnet from aux address
// Returns: IP address, subnet CIDR, error
func parseAuxAddress(auxAddr string, defaultSubnet string) (string, string, error) {
	// Check if it contains CIDR notation
	if strings.Contains(auxAddr, "/") {
		// Parse as CIDR
		ip, ipnet, err := net.ParseCIDR(auxAddr)
		if err != nil {
			return "", "", fmt.Errorf("invalid CIDR notation in aux address: %w", err)
		}
		return ip.String(), ipnet.String(), nil
	}
	
	// Just an IP address - use the default subnet
	ip := net.ParseIP(auxAddr)
	if ip == nil {
		return "", "", fmt.Errorf("invalid IP address: %s", auxAddr)
	}
	return ip.String(), defaultSubnet, nil
}

// checkSubnetConflicts checks if the macvlan subnet conflicts with existing routes
func checkSubnetConflicts(cfg *MacvlanConfig) error {
    _, macvlanNet, err := net.ParseCIDR(cfg.IPv4Subnet)
    if err != nil {
        return fmt.Errorf("invalid subnet: %w", err)
    }
    
    // Get existing routes
    routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
    if err != nil {
        return fmt.Errorf("failed to list routes: %w", err)
    }
    
    // Check for conflicts
    for _, route := range routes {
        if route.Dst != nil {
            // Skip default route
            if route.Dst.String() == "0.0.0.0/0" {
                continue
            }
            
            // Check if macvlan subnet overlaps with existing route
            if netsOverlap(macvlanNet, route.Dst) {
                // Get the interface name for the route
                var ifaceName string
                if route.LinkIndex > 0 {
                    link, err := netlink.LinkByIndex(route.LinkIndex)
                    if err == nil {
                        ifaceName = link.Attrs().Name
                    }
                }
                
                return fmt.Errorf("macvlan subnet %s conflicts with existing route %s on interface %s", 
                    cfg.IPv4Subnet, route.Dst.String(), ifaceName)
            }
        }
    }
    
    return nil
}

// netsOverlap checks if two networks overlap
func netsOverlap(n1, n2 *net.IPNet) bool {
    return n1.Contains(n2.IP) || n2.Contains(n1.IP)
}

// CreateHostMacvlanInterface creates a macvlan interface on the host for container communication
func CreateHostMacvlanInterface(cfg *MacvlanConfig) error {
	hostIfNameNonAlpha := cfg.NetworkName + "host"
	hostIfName := SanitizeInterfaceName(hostIfNameNonAlpha)
	
	// Parse aux address to get IP and route subnet
	auxIP, routeSubnet, err := parseAuxAddress(cfg.AuxAddress, cfg.IPv4Subnet)
	if err != nil {
		return fmt.Errorf("failed to parse aux address: %w", err)
	}
	
	log.Debugf("Creating host macvlan interface: name=%s, parent=%s, mode=%s, IP=%s, route=%s", 
		hostIfName, cfg.ParentIface, cfg.MacvlanMode, auxIP, routeSubnet)
	
	// Check if interface already exists
	if existingLink, err := netlink.LinkByName(hostIfName); err == nil {
		log.Debugf("Host macvlan interface %s already exists", hostIfName)
		// Check if it has the correct IP
		addrs, err := netlink.AddrList(existingLink, netlink.FAMILY_V4)
		if err == nil {
			for _, addr := range addrs {
				if addr.IP.String() == auxIP {
					log.Debugf("Interface %s already has IP %s", hostIfName, auxIP)
					return nil
				}
			}
		}
		// Interface exists but might not have the right IP, delete and recreate
		log.Debugf("Removing existing interface %s to recreate with correct settings", hostIfName)
		if err := netlink.LinkDel(existingLink); err != nil {
			log.Warnf("Failed to delete existing interface: %v", err)
		}
	}
	
	// Get parent link
	parentLink, err := netlink.LinkByName(cfg.ParentIface)
	if err != nil {
		return fmt.Errorf("parent interface %s not found: %w", cfg.ParentIface, err)
	}
	
	// Determine macvlan mode
	mode := parseMacvlanMode(cfg.MacvlanMode)
	
	// Create macvlan link
	macvlan := &netlink.Macvlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:        hostIfName,
			ParentIndex: parentLink.Attrs().Index,
		},
		Mode: mode,
	}
	
	// Create the interface via netlink
	if err := netlink.LinkAdd(macvlan); err != nil {
		if strings.Contains(err.Error(), "numerical result") {
			log.Errorf("Netlink error details - this often indicates an issue with the parent interface index or mode value")
			log.Errorf("Parent index: %d, Mode: %d", parentLink.Attrs().Index, mode)
		}
		return fmt.Errorf("failed to create macvlan interface: %w", err)
	}
	
	// Get the created interface
	link, err := netlink.LinkByName(hostIfName)
	if err != nil {
		netlink.LinkDel(macvlan)
		return fmt.Errorf("failed to get created interface: %w", err)
	}
	
	// Parse and add IP address (always use /32 for the interface itself)
	addrStr := auxIP + "/32"
	addr, err := netlink.ParseAddr(addrStr)
	if err != nil {
		netlink.LinkDel(link)
		return fmt.Errorf("failed to parse IP address %s: %w", addrStr, err)
	}
	
	if err := netlink.AddrAdd(link, addr); err != nil {
		netlink.LinkDel(link)
		return fmt.Errorf("failed to add IP address: %w", err)
	}
	
	// Bring the interface up
	if err := netlink.LinkSetUp(link); err != nil {
		netlink.LinkDel(link)
		return fmt.Errorf("failed to bring interface up: %w", err)
	}
	
	log.Infof("Created host macvlan interface %s with IP %s", hostIfName, auxIP)
	
	// Add route using the specified or default subnet
	_, ipnet, err := net.ParseCIDR(routeSubnet)
	if err != nil {
		log.Warnf("Failed to parse subnet for route: %v", err)
		return nil
	}
	
	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       ipnet,
		Scope:     netlink.SCOPE_LINK,
	}
	
	if err := netlink.RouteAdd(route); err != nil {
		if strings.Contains(err.Error(), "file exists") {
			log.Warnf("Route %s already exists - this usually means the subnet overlaps with your host network", routeSubnet)
			if routeSubnet == cfg.IPv4Subnet {
				log.Info("Consider using CIDR notation in macvlan-aux (e.g., 192.168.1.129/26) to specify a smaller route subnet")
			}
		} else {
			log.Warnf("Failed to add route %s dev %s: %v", routeSubnet, hostIfName, err)
		}
	} else {
		log.Infof("Added route %s dev %s", routeSubnet, hostIfName)
		if routeSubnet != cfg.IPv4Subnet {
			log.Infof("Note: Using route subnet %s (from aux CIDR) instead of full network %s", routeSubnet, cfg.IPv4Subnet)
		}
	}

    return nil
}

// CleanupMacvlanPostActions reverses the changes made in PostCreateMacvlanActions
func CleanupMacvlanPostActions(cfg *MacvlanConfig) error {
	// First, remove the static route if it exists
	if cfg.AuxAddress != "" && cfg.IPv4Subnet != "" {
		hostIfNameNonAlpha := cfg.NetworkName + "host"
		hostIfName := SanitizeInterfaceName(hostIfNameNonAlpha)
		
		// Parse aux address to get the route subnet
		_, routeSubnet, err := parseAuxAddress(cfg.AuxAddress, cfg.IPv4Subnet)
		if err == nil {
			_, ipnet, err := net.ParseCIDR(routeSubnet)
			if err == nil {
				routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
				if err == nil {
					for _, route := range routes {
						if route.Dst != nil && route.Dst.String() == ipnet.String() {
							if route.LinkIndex > 0 {
								link, err := netlink.LinkByIndex(route.LinkIndex)
								if err == nil && link.Attrs().Name == hostIfName {
									if err := netlink.RouteDel(&route); err != nil {
										log.Debugf("Failed to delete route %s dev %s: %v", 
											route.Dst.String(), hostIfName, err)
									} else {
										log.Infof("Removed route %s dev %s", 
											route.Dst.String(), hostIfName)
									}
								}
							}
						}
					}
				}
			}
		}
	}
	
	// Then cleanup the host interface
	if err := CleanupHostMacvlanInterface(cfg); err != nil {
		log.Warnf("Failed to cleanup host macvlan interface: %v", err)
	}
	
	// Disable promiscuous mode on parent interface
	parentLink, err := netlink.LinkByName(cfg.ParentIface)
	if err != nil {
		log.Debugf("Parent interface %s not found during cleanup: %v", cfg.ParentIface, err)
		return nil
	}
	
	// Check if there are other macvlan interfaces using this parent
	links, err := netlink.LinkList()
	if err == nil {
		otherMacvlans := false
		hostIfName := SanitizeInterfaceName(cfg.NetworkName + "host")
		for _, link := range links {
			if macvlan, ok := link.(*netlink.Macvlan); ok {
				if macvlan.ParentIndex == parentLink.Attrs().Index && 
				   macvlan.Name != hostIfName {
					otherMacvlans = true
					break
				}
			}
		}
		
		// Only disable promiscuous mode if no other macvlans are using this parent
		if !otherMacvlans {
			if err := DisablePromiscuousMode(cfg.ParentIface); err != nil {
				log.Warnf("Failed to disable promiscuous mode on %s: %v", cfg.ParentIface, err)
			} else {
				log.Debugf("Disabled promiscuous mode on %s", cfg.ParentIface)
			}
		} else {
			log.Debugf("Other macvlan interfaces exist on %s, keeping promiscuous mode enabled", cfg.ParentIface)
		}
	}
	
	return nil
}

// CleanupHostMacvlanInterface removes the host macvlan interface if it exists
func CleanupHostMacvlanInterface(cfg *MacvlanConfig) error {
	if cfg.AuxAddress == "" {
		return nil
	}
	
	hostIfNameNonAlpha := cfg.NetworkName + "host"
	hostIfName := SanitizeInterfaceName(hostIfNameNonAlpha)

	link, err := netlink.LinkByName(hostIfName)
	if err != nil {
		// Interface doesn't exist, nothing to clean up
		return nil
	}
	
	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("failed to delete host macvlan interface: %w", err)
	}
	
	log.Infof("Removed host macvlan interface %s", hostIfName)
	return nil
}

// EnablePromiscuousMode enables promiscuous mode on an interface
func EnablePromiscuousMode(ifName string) error {
	cmd := osexec.Command("ip", "link", "set", ifName, "promisc", "on")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable promiscuous mode: %w", err)
	}
	return nil
}

// DisablePromiscuousMode disables promiscuous mode on an interface
func DisablePromiscuousMode(ifName string) error {
	cmd := osexec.Command("ip", "link", "set", ifName, "promisc", "off")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to disable promiscuous mode: %w", err)
	}
	return nil
}

// SanitizeInterfaceName removes non-alphanumeric characters from interface names
func SanitizeInterfaceName(input string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	return re.ReplaceAllString(input, "")
}

// Helper functions

func parseMacvlanMode(mode string) netlink.MacvlanMode {
	switch mode {
	case "", "bridge":
		return netlink.MACVLAN_MODE_BRIDGE
	case "vepa":
		return netlink.MACVLAN_MODE_VEPA
	case "private":
		return netlink.MACVLAN_MODE_PRIVATE
	case "passthru":
		return netlink.MACVLAN_MODE_PASSTHRU
	default:
		log.Warnf("Unknown macvlan mode %s, defaulting to bridge", mode)
		return netlink.MACVLAN_MODE_BRIDGE
	}
}