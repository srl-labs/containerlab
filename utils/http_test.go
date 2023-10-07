package utils

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestIsGithubURL(t *testing.T) {
	// tests that github urls are detected
	var tests = []struct {
		input    string
		expected bool
	}{
		{"github.com", true},
		{"github.com/containers/containerlab/blob/master/README.md", true},
		{"google.com/containers", false},
		{"google.com/containers/containerlab/blob/master/README.md", false},
		{"gitlab.com/containers", false},
		{"raw.githubusercontent.com/containers", true},
	}
	for _, test := range tests {
		if output := IsGitHubURL(test.input); output != test.expected {
			t.Error("Test Failed: {} inputted, {} expected, recieved: {}", test.input, test.expected, output)
		}
	}
}

func TestGetRawURL(t *testing.T) {
	// tests that github urls are converted to raw urls evething else is left as is
	var tests = []struct {
		input    string
		expected string
	}{
		{"github.com", "raw.githubusercontent.com"},
		{"github.com/containers/containerlab/blob/master/README.md", "raw.githubusercontent.com/containers/containerlab/master/README.md"},
		{"google.com/containers", "google.com/containers"},
		{"google.com/containers/containerlab/blob/master/README.md", "google.com/containers/containerlab/blob/master/README.md"},
		{"gitlab.com/containers", "gitlab.com/containers"},
		{"raw.githubusercontent.com/containers", "raw.githubusercontent.com/containers"},
	}
	for _, test := range tests {
		if output := GetRawURL(test.input); output != test.expected {
			t.Error("Test Failed: {} inputted, {} expected, recieved: {}", test.input, test.expected, output)
		}
	}
}

func TestGetFileName(t *testing.T) {
	// tests for valid file name
	var tests = []struct {
		input    string
		expected string
	}{
		{"github.com", "github.com"},
		{"github.com/containers/containerlab/blob/master/README.md", "README.md"},
		{"google.com/containers", "containers"},
		{"google.com/containers/containerlab/blob/master/README.md", "README.md"},
		{"gitlab.com/containers", "containers"},
		{"raw.githubusercontent.com/containers", "containers"},
	}
	for _, test := range tests {
		if output := GetFileName(test.input); output != test.expected {
			t.Error("Test Failed: {} inputted, {} expected, recieved: {}", test.input, test.expected, output)
		}
	}
}

func TestCheckSuffix(t *testing.T) {
	// tests for valid suffix
    var tests = []struct {
        input    string
        expected error
    }{
        {"github.com", ErrInvalidSuffix},
        {"github.com/containers/containerlab/blob/master/README.md", ErrInvalidSuffix},
        {"google.com/containers", ErrInvalidSuffix},
        {"google.com/containers/containerlab/blob/master/README.md", ErrInvalidSuffix},
        {"gitlab.com/containers", ErrInvalidSuffix},
        {"raw.githubusercontent.com/containers", ErrInvalidSuffix},
        {"github.com/containers/containerlab/blob/master/README.yml", nil},
        {"github.com/containers/containerlab/blob/master/README.yaml", nil},
    }
    for _, test := range tests {
        if output := CheckSuffix(test.input); output != test.expected {
            t.Error("Test Failed: {} inputted, {} expected, recieved: {}", test.input, test.expected, output)
        }
    }
}

func TestDownloadFile(t *testing.T) {
    tempDir := os.TempDir() 
    // Create a mock HTTP server for testing
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Respond with a sample file content for testing
        if r.URL.Path == "/valid" {
            w.WriteHeader(http.StatusOK)
            w.Write([]byte("This is a sample file content"))
        } else {
            w.WriteHeader(http.StatusNotFound)
        }
    }))
    defer ts.Close()

    // Test case 1: Download a file that exists
    outputFileName := tempDir + "/downloaded_file.txt"
    err := DownloadFile(ts.URL+"/valid", outputFileName)
    if err != nil {
        t.Fatalf("Expected no error, but got: %v", err)
    }

    // Check the content of the downloaded file
    content, err := os.ReadFile(outputFileName)
    if err != nil {
        t.Fatalf("Failed to read the downloaded file: %v", err)
    }
    expectedContent := "This is a sample file content"
    if string(content) != expectedContent {
        t.Errorf("Expected content: %s, but got: %s", expectedContent, string(content))
    }
	os.Remove(outputFileName)
    // Test case 2: Download a file that does not exist (simulate a non-200 response)
    outputFileName = tempDir + "/nonexistent_file.txt"
    err = DownloadFile(ts.URL+"/nonexistent", outputFileName)
    expectedErrorMsg := "URL does not exist"
    if err == nil || err.Error() != expectedErrorMsg {
        t.Errorf("Expected error message '%s', but got: %v", expectedErrorMsg, err)
    }
}