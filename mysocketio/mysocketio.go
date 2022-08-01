package mysocketio

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	h "net/http"
	"os"
	"strings"
)

type Client struct {
	token string
}

func GetApiUrl() string {
	if os.Getenv("MYSOCKET_API") != "" {
		return os.Getenv("MYSOCKET_API")
	} else {
		return "https://api.mysocket.io"
	}
}

func NewClient(tokenfile string) (*Client, error) {
	token, err := GetToken(tokenfile)
	if err != nil {
		return nil, err
	}

	c := &Client{token: token}

	return c, nil
}

func (c *Client) Request(method, url string, target, data interface{}) error {
	jv, _ := json.Marshal(data)
	body := bytes.NewBuffer(jv)

	req, _ := h.NewRequest(method, fmt.Sprintf("%s/%s", GetApiUrl(), url), body)
	req.Header.Add("x-access-token", c.token)
	req.Header.Set("Content-Type", "application/json")
	client := &h.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return errors.New("no valid token, Please login")
	}

	if resp.StatusCode < 200 || resp.StatusCode > 204 {
		responseData, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("failed to create object (%d) %v", resp.StatusCode, string(responseData))
	}

	if resp.StatusCode == 204 {
		return nil
	}

	err = json.NewDecoder(resp.Body).Decode(target)
	if err != nil {
		return errors.New("failed to decode data")
	}

	return nil
}

func GetToken(tokenfile string) (string, error) {
	if _, err := os.Stat(tokenfile); os.IsNotExist(err) {
		return "", errors.New("please login first (no token found)")
	}
	content, err := ioutil.ReadFile(tokenfile)
	if err != nil {
		return "", err
	}

	tokenString := strings.TrimRight(string(content), "\n")
	return tokenString, nil
}

type Socket struct {
	Tunnels               []Tunnel `json:"tunnels,omitempty"`
	Username              string   `json:"user_name,omitempty"`
	SocketID              string   `json:"socket_id,omitempty"`
	SocketTcpPorts        []int    `json:"socket_tcp_ports,omitempty"`
	Dnsname               string   `json:"dnsname,omitempty"`
	Name                  string   `json:"name,omitempty"`
	SocketType            string   `json:"socket_type,omitempty"`
	ProtectedSocket       bool     `json:"protected_socket"`
	ProtectedUsername     string   `json:"protected_username"`
	ProtectedPassword     string   `json:"protected_password"`
	CloudAuthEnabled      bool     `json:"cloud_authentication_enabled,omitempty"`
	AllowedEmailAddresses []string `json:"cloud_authentication_email_allowed_addressses,omitempty"`
	AllowedEmailDomains   []string `json:"cloud_authentication_email_allowed_domains,omitempty"`
	SSHCa                 string   `json:"ssh_ca,omitempty"`
	UpstreamUsername      string   `json:"upstream_username,omitempty"`
	UpstreamPassword      string   `json:"upstream_password,omitempty"`
	UpstreamHttpHostname  string   `json:"upstream_http_hostname,omitempty"`
	UpstreamType          string   `json:"upstream_type,omitempty"`
}

type Tunnel struct {
	TunnelID     string `json:"tunnel_id,omitempty"`
	LocalPort    int    `json:"local_port,omitempty"`
	TunnelServer string `json:"tunnel_server,omitempty"`
}
