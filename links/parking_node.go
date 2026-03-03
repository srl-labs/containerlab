package links

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"

	"github.com/vishvananda/netns"
)

const (
	parkingNetnsPrefix = "clab-park-"
	// Be conservative and keep the name comfortably within Linux NAME_MAX (255 bytes).
	maxParkingNetnsNameLen = 200
)

func parkingNetnsName(containerName string) string {
	name := parkingNetnsPrefix + containerName
	if len(name) <= maxParkingNetnsNameLen {
		return name
	}

	sum := sha1.Sum([]byte(containerName))
	suffix := hex.EncodeToString(sum[:])[:10]

	// leave room for "-" + suffix
	maxBaseLen := maxParkingNetnsNameLen - 1 - len(suffix)
	return name[:maxBaseLen] + "-" + suffix
}

func getOrCreateNamedNetNS(name string) (nsPath string, err error) {
	nsPath = filepath.Join("/run/netns", name)

	if err := os.MkdirAll(filepath.Dir(nsPath), 0o755); err != nil {
		return "", err
	}

	if _, statErr := os.Stat(nsPath); statErr == nil {
		return nsPath, nil
	} else if !os.IsNotExist(statErr) {
		return "", statErr
	}

	goruntime.LockOSThread()
	defer goruntime.UnlockOSThread()

	currentNS, err := netns.Get()
	if err != nil {
		return "", err
	}
	defer currentNS.Close()

	defer func() {
		if restoreErr := netns.Set(currentNS); restoreErr != nil {
			if err == nil {
				err = restoreErr
				return
			}
			err = fmt.Errorf("%w (failed restoring netns: %v)", err, restoreErr)
		}
	}()

	newNS, err := netns.NewNamed(name)
	if err != nil {
		if os.IsExist(err) {
			if _, statErr := os.Stat(nsPath); statErr == nil {
				return nsPath, nil
			} else if !os.IsNotExist(statErr) {
				return "", statErr
			}
		}
		return "", err
	}
	newNS.Close()

	return nsPath, nil
}

type ParkingNode struct {
	*genericLinkNode
}

func NewParkingNode(containerName string) (*ParkingNode, error) {
	parkName := parkingNetnsName(containerName)

	parkPath, err := getOrCreateNamedNetNS(parkName)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create parking netns %q: %w", parkName, err)
	}

	return &ParkingNode{
		genericLinkNode: newGenericLinkNode(parkName, parkPath),
	}, nil
}

func GetParkingNode(containerName string) (*ParkingNode, error) {
	nsName := parkingNetnsName(containerName)
	nsPath := filepath.Join("/run/netns", nsName)

	if _, err := os.Stat(nsPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("parking netns %q does not exist", nsName)
		}
		return nil, err
	}

	return &ParkingNode{
		genericLinkNode: newGenericLinkNode(nsName, nsPath),
	}, nil
}

func (p *ParkingNode) NSPath() string {
	return p.nspath
}

// DeleteParkingNetns removes the parking netns created for a container.
func DeleteParkingNetns(containerName string) error {
	nsName := parkingNetnsName(containerName)
	if err := netns.DeleteNamed(nsName); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return nil
}
