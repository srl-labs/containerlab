// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package border0_api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/utils"
	"gopkg.in/yaml.v2"
)

const apiUrl = "https://api.border0.com/api/v1"

var supportedSockTypes = []string{"ssh", "tls", "http", "https"}

func Login(email, password string) error {
	// if password not set read from terminal
	if password == "" {
		var err error
		password, err = utils.ReadPasswordFromTerminal()
		if err != nil {
			return err
		}
	}

	loginReq := &LoginRequest{
		Email:    email,
		Password: password,
	}
	loginResp := &LoginResponse{}

	err := Request("POST", "login", loginResp, loginReq, false)
	if err != nil {
		return err
	}

	err = writeToken(loginResp.Token)
	if err != nil {
		return err
	}
	return nil
}

func getApiUrl() string {
	if os.Getenv("BORDER0_API") != "" {
		return os.Getenv("BORDER0_API")
	} else {
		return apiUrl
	}
}

func getToken() (string, error) {
	if os.Getenv("BORDER0_ADMIN_TOKEN") != "" {
		return os.Getenv("BORDER0_ADMIN_TOKEN"), nil
	}

	if _, err := os.Stat(tokenfile()); os.IsNotExist(err) {
		return "", errors.New("API: please login first (no token found)")
	}
	content, err := os.ReadFile(tokenfile())
	if err != nil {
		return "", err
	}

	tokenString := strings.TrimSpace(string(content))
	return tokenString, nil
}

func checkPoliciesExist(policyNames map[string]struct{}) error {
	policies, err := GetExistingPolicies()
	if err != nil {
		return err
	}
	notFound := []string{}
OUTER:
	for name := range policyNames {
		for _, policy := range policies {
			if name == policy.Name {
				continue OUTER
			}
		}
		notFound = append(notFound, name)
	}
	if len(notFound) > 0 {
		return fmt.Errorf("border0.com policies %q referenced but they don't exist", strings.Join(notFound, ", "))
	}
	return nil
}

func tokenfile() string {
	cwd, _ := os.Getwd()
	tokenfile := fmt.Sprintf("%s/.border0_token", cwd)
	if utils.FileExists(tokenfile) {
		return tokenfile
	}
	tokenfile = fmt.Sprintf("%s/.border0/token", os.Getenv("HOME"))
	return tokenfile
}

// cwdTokenFilePath get the abspath of the token file in the current working directory
func cwdTokenFilePath() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return filepath.Join(cwd, ".border0_token")
}

func GetExistingPolicies() ([]Policy, error) {
	var policies []Policy
	err := Request("GET", "policies", &policies, nil, true)
	if err != nil {
		return nil, err
	}
	return policies, nil
}

func Request(method string, url string, targetStruct interface{}, data interface{}, requireAccessToken bool) error {
	jv, _ := json.Marshal(data)
	body := bytes.NewBuffer(jv)

	req, _ := http.NewRequest(method, fmt.Sprintf("%s/%s", getApiUrl(), url), body)

	var token = ""
	//try to find the token in the environment
	if requireAccessToken {
		token, _ = getToken()
	}
	if token != "" {
		token = strings.TrimSpace(token)
		req.Header.Add("x-access-token", token)
	}
	req.Header.Add("x-client-requested-with", "containerlab")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("401 Unauthorized, maybe the token expired")
	}

	if resp.StatusCode == 429 {
		responseData, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("rate limit error: %v", string(responseData))
	}

	if resp.StatusCode == 404 {
		return fmt.Errorf("404 NotFound")
	}

	if resp.StatusCode < 200 || resp.StatusCode > 204 {
		var errorMessage ErrorMessage
		json.NewDecoder(resp.Body).Decode(&errorMessage)

		return fmt.Errorf("failed to create object (%d) %v", resp.StatusCode, errorMessage.ErrorMessage)
	}

	if resp.StatusCode == 204 {
		return nil
	}

	err = json.NewDecoder(resp.Body).Decode(targetStruct)
	if err != nil {
		return fmt.Errorf("error decoding body: %w", err)
	}

	return nil
}

// RefreshLogin checking the validity of the login token as well as it
func RefreshLogin(ctx context.Context) error {
	t := &LoginRefreshResponse{}
	log.Debug("Validating and refreshing border0.com token")
	err := Request("POST", "login/refresh", t, nil, true)
	if err != nil {
		return err
	}
	err = writeToken(t.Token)
	if err != nil {
		return err
	}
	return nil
}

// writeToken writes the given token to a file
func writeToken(token string) error {
	absPathToken := cwdTokenFilePath()

	err := ioutil.WriteFile(absPathToken, []byte(token), 0600)
	if err != nil {
		return fmt.Errorf("failed to write border0.com token file as %s: %v",
			absPathToken, err)
	}
	log.Infof("Saved border0.com token to %s", absPathToken)
	return nil
}

func CreateBorder0Config(ctx context.Context, nodesMap map[string]nodes.Node, labname string) (string, error) {

	log.Debug("Creating the border0.com configuration")
	// acquire token
	border0Token, err := getToken()
	if err != nil {
		return "", err
	}
	// init config struct
	yamlConfig := &StaticSocketsConfig{
		Connector: &configConnector{
			Name: labname,
		},
		Credentials: &configCredentials{
			Token: border0Token,
		},
		Sockets: []map[string]*configSocket{},
	}

	// helper map to collect all the referenced policies for later existence check
	policyNames := map[string]struct{}{}

	// iterate over structs to generate socket configs
	for _, n := range nodesMap {
		for _, socket := range n.Config().Publish {
			// Parse the socket config
			sockConfig, err := ParseSocketCfg(socket, n.Config().LongName)
			if err != nil {
				return "", err
			}

			// add the referenced policies to the map for later checking
			for _, policy := range sockConfig.Policies {
				policyNames[policy] = struct{}{}
			}

			// determine the socketname
			socketName := fmt.Sprintf("%s-%s-%d", n.Config().LongName, sockConfig.Type, sockConfig.Port)
			yamlConfig.Sockets = append(yamlConfig.Sockets, map[string]*configSocket{
				socketName: sockConfig,
			})
		}
	}

	// check for the existence of referenced policies
	err = checkPoliciesExist(policyNames)
	if err != nil {
		return "", err
	}

	// marshall the config into a []byte
	bconfig, err := yaml.Marshal(yamlConfig)
	if err != nil {
		return "", err
	}

	// return yaml based string configuration
	return string(bconfig), nil
}

func ParseSocketCfg(s, host string) (*configSocket, error) {

	result := &configSocket{}

	split := strings.Split(s, "/")
	if len(split) > 3 {
		return result, fmt.Errorf("wrong mysocketio publish section %s. should be <type>/<port-number>[/<policyname>,] i.e. tcp/22, tls/22/mypolicy1 or tls/22/mypolicy1,myotherpolicy", s)
	}

	// process SocketType
	if err := checkSockType(split[0]); err != nil {
		return result, err
	}
	result.Type = split[0]
	// process port
	p, err := strconv.Atoi(split[1]) // port
	if err != nil {
		return result, err
	}
	if err := checkSockPort(p); err != nil {
		return result, err
	}
	result.Port = p

	// process policy
	result.Policies = []string{}
	if len(split) == 3 {
		// split the possible multiple policies
		splitPolicy := strings.Split(split[2], ",")
		// add all the mentioned policies to the result
		for _, x := range splitPolicy {
			// trim whitespaces and add the result
			result.Policies = append(result.Policies, strings.TrimSpace(x))
		}
	}
	// process host
	result.Host = host
	return result, nil
}

func checkSockType(t string) error {
	if _, ok := utils.StringInSlice(supportedSockTypes, t); !ok {
		return fmt.Errorf("border0.com does not support socket type %q. Supported types are %q", t, strings.Join(supportedSockTypes, "|"))
	}
	return nil
}

func checkSockPort(p int) error {
	if p < 1 || p > 65535 {
		return fmt.Errorf("incorrect port number %v", p)
	}
	return nil
}
