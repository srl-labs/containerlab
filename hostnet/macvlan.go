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
	AuxAddress     string
	IPv4Subnet     string
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
		if err := CreateHostMacvlanInterface(cfg); err != nil {
			// Don't fail the entire operation, just warn
			log.Warnf("Failed to create host macvlan interface: %v", err)
			log.Info("You can manually create it with:")
			log.Infof("  sudo ip link add %s-host link %s type macvlan mode bridge", 
				cfg.NetworkName, cfg.ParentIface)
			log.Infof("  sudo ip addr add %s/%s dev %s-host", 
				cfg.AuxAddress, getSubnetPrefix(cfg.IPv4Subnet), cfg.NetworkName)
			log.Infof("  sudo ip link set %s-host up", cfg.NetworkName)
		} else {
			log.Infof("Created host macvlan interface %s-host with IP %s", 
				cfg.NetworkName, cfg.AuxAddress)
		}
	} else {
		// Still warn about the limitation
		log.Info("Note: Host cannot directly communicate with macvlan containers due to kernel limitations. " +
			"Consider setting 'macvlan-aux' to create a host interface.")
	}
	
	return nil
}

// CreateHostMacvlanInterface creates a macvlan interface on the host for container communication
func CreateHostMacvlanInterface(cfg *MacvlanConfig) error {
	hostIfNameNonAlpha := cfg.NetworkName + "-host"
	hostIfName := SanitizeInterfaceName(hostIfNameNonAlpha)
	
	log.Debugf("Creating host macvlan interface: name=%s, parent=%s, mode=%s", 
		hostIfName, cfg.ParentIface, cfg.MacvlanMode)
	
	// Check if interface already exists
	if existingLink, err := netlink.LinkByName(hostIfName); err == nil {
		log.Debugf("Host macvlan interface %s already exists", hostIfName)
		// Check if it has the correct IP
		addrs, err := netlink.AddrList(existingLink, netlink.FAMILY_V4)
		if err == nil {
			for _, addr := range addrs {
				if addr.IP.String() == cfg.AuxAddress {
					log.Debugf("Interface %s already has IP %s", hostIfName, cfg.AuxAddress)
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
	
	// Parse and add IP address
	addrStr := cfg.AuxAddress + "/26"
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
	
	// Add route to the subnet
	_, ipnet, err := net.ParseCIDR(cfg.IPv4Subnet)
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
		if !strings.Contains(err.Error(), "file exists") {
			log.Warnf("Failed to add route %s dev %s: %v", cfg.IPv4Subnet, hostIfName, err)
		}
	} else {
		log.Infof("Added route %s dev %s", cfg.IPv4Subnet, hostIfName)
	}
	
	return nil
}

// CleanupMacvlanPostActions reverses the changes made in PostCreateMacvlanActions
func CleanupMacvlanPostActions(cfg *MacvlanConfig) error {
	// First, remove the static route if it exists
	if cfg.AuxAddress != "" && cfg.IPv4Subnet != "" {
		hostIfNameNonAlpha := cfg.NetworkName + "-host"
		hostIfName := SanitizeInterfaceName(hostIfNameNonAlpha)
		
		_, ipnet, err := net.ParseCIDR(cfg.IPv4Subnet)
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
		hostIfName := SanitizeInterfaceName(cfg.NetworkName + "-host")
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
	
	hostIfNameNonAlpha := cfg.NetworkName + "-host"
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

func getSubnetPrefix(subnet string) string {
	parts := strings.Split(subnet, "/")
	if len(parts) == 2 {
		return parts[1]
	}
	return "24" // default
}

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