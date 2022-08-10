package types

import (
	"testing"
)

type mysocketioTestDataType = struct {
	entry                  *MySocketIoEntry
	getContainerNameOutput string
	getContainerNameErr    bool
	isClabEntryResult      bool
}

func getMySocketIoTestData() []mysocketioTestDataType {
	return []mysocketioTestDataType{
		{
			entry:                  &MySocketIoEntry{Name: strptr("clab-slr01-srlnode1-tcp-22")},
			getContainerNameErr:    false,
			getContainerNameOutput: "slr01-srlnode1",
			isClabEntryResult:      true,
		},
		{
			entry:                  &MySocketIoEntry{Name: strptr("clab-slr01-srlnode1-udp-67464563")},
			getContainerNameErr:    false,
			getContainerNameOutput: "slr01-srlnode1",
			isClabEntryResult:      true,
		},
		{
			entry:                  &MySocketIoEntry{Name: strptr("clab-srlnode1-udp-67464563")},
			getContainerNameErr:    false,
			getContainerNameOutput: "srlnode1",
			isClabEntryResult:      true,
		},
		{
			entry:                  &MySocketIoEntry{Name: strptr("clab-srlnode1")},
			getContainerNameErr:    true,
			getContainerNameOutput: "",
			isClabEntryResult:      false,
		},
	}
}

// helper to convert from string to stringpointer.
func strptr(s string) *string {
	return &s
}

func TestMySocketIoEntry_getContainerName(t *testing.T) {
	for _, x := range getMySocketIoTestData() {

		result, err := x.entry.getContainerName()
		if (err != nil) != x.getContainerNameErr {
			t.Errorf("Error expected [%v] on %s but error was %v",
				x.getContainerNameErr, *x.entry.Name, err)
			continue
		}
		if x.getContainerNameOutput != "" && result != x.getContainerNameOutput {
			t.Errorf("Expected %s but got %s from input %s", x.getContainerNameOutput, result, *x.entry.Name)
			continue
		}
	}
}

func TestMySocketIoEntry_isClabEntry(t *testing.T) {
	for _, x := range getMySocketIoTestData() {
		result := x.entry.isClabEntry()
		if result != x.isClabEntryResult {
			t.Errorf("isClabentry check for %s unexpectedly resulted in %v", *x.entry.Name, result)
			continue
		}
	}
}
