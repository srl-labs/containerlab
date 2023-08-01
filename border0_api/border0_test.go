// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package border0_api

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/h2non/gock"
	"github.com/srl-labs/containerlab/mocks/mocknodes"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
)

func TestLogin(t *testing.T) {
	type args struct {
		email    string
		password string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test login fail",
			args: args{
				email:    "fgsdfg@dfgsdfg.fr",
				password: "1defsdffgd",
			},
			wantErr: true,
		},
		// // re-add this line and create a seperate file in this folder
		// // defining the two variables BORDER0USER and BORDER0PASSWORD with
		// // valid border0 credentials to test again live api
		// // e.g.:
		// // package border0_api
		// // const BORDER0USER = "e@mail.address"
		// // const BORDER0PASSWORD = "PASSWORD"
		// //
		// {
		// 	name: "test login fail",
		// 	args: args{
		// 		email:    BORDER0USER,
		// 		password: BORDER0PASSWORD,
		// 	},
		// 	wantErr: false,
		// },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			if err := Login(ctx, tt.args.email, tt.args.password); (err != nil) != tt.wantErr {
				t.Errorf("Login() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_createBorder0Config(t *testing.T) {
	type args struct {
		ctx      context.Context
		nodesMap func(mockCtrl *gomock.Controller) map[string]nodes.Node
		labname  string
	}
	tests := []struct {
		name string
		args args
		// want    func() string
		wantErr bool
	}{
		{
			name: "test ",
			args: args{
				ctx:      context.TODO(),
				nodesMap: getNodeMap,
				labname:  "MyTinyTestLab",
			},
			// want: func() string {
			// 	ssc := StaticSocketsConfig{
			// 		Connector: &configConnector{
			// 			Name: "MyTinyTestLab",
			// 		},
			// 		Credentials: &configCredentials{
			// 			Token: "SomeValueOtherThenNil",
			// 		},
			// 		Sockets: []map[string]*configSocket{
			// 			{
			// 				"clab-TestTopo-node2-tls-22": {
			// 					Port: 22,
			// 					Type: "tls",
			// 					Host: "clab-TestTopo-node2",
			// 				},
			// 			},
			// 			{
			// 				"clab-TestTopo-node2-tls-23": {
			// 					Port:     23,
			// 					Type:     "tls",
			// 					Host:     "clab-TestTopo-node2",
			// 					Policies: []string{"myfunnypolicy"},
			// 				},
			// 			},
			// 			{
			// 				"clab-TestTopo-node5-tls-22": {
			// 					Port: 22,
			// 					Type: "tls",
			// 					Host: "clab-TestTopo-node5",
			// 				},
			// 			},
			// 			{
			// 				"clab-TestTopo-node5-tls-25": {
			// 					Port: 25,
			// 					Type: "tls",
			// 					Host: "clab-TestTopo-node5",
			// 					Policies: []string{
			// 						"test",
			// 						"additionalpolicy",
			// 					},
			// 				},
			// 			},
			// 		},
			//	}
			// 	bytesData, err := yaml.Marshal(ssc)
			// 	if err != nil {
			// 		t.Error(err)
			// 	}
			// 	return string(bytesData)
			// },
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// set the token variable first
			err := os.Setenv(ENV_NAME_BORDER0_ADMIN_TOKEN, "SomeValueOtherThenNil")
			if err != nil {
				t.Error(err)
			}

			// mock the http client, to be able to inject responses
			defer gock.Off()
			gock.New(getApiUrl()).
				Get("/policies").
				Reply(200).
				JSON([]Policy{
					{
						ID:          "1",
						Name:        "Foo",
						Description: "",
						PolicyData:  PolicyData{},
						SocketIDs:   []string{},
						OrgID:       "",
						OrgWide:     false,
						CreatedAt:   time.Time{},
					},
					{
						ID:          "2",
						Name:        "test",
						Description: "",
						PolicyData:  PolicyData{},
						SocketIDs:   []string{},
						OrgID:       "",
						OrgWide:     false,
						CreatedAt:   time.Time{},
					},
					{
						ID:          "3",
						Name:        "additionalpolicy",
						Description: "",
						PolicyData:  PolicyData{},
						SocketIDs:   []string{},
						OrgID:       "",
						OrgWide:     false,
						CreatedAt:   time.Time{},
					},
					{
						ID:          "4",
						Name:        "myfunnypolicy",
						Description: "",
						PolicyData:  PolicyData{},
						SocketIDs:   []string{},
						OrgID:       "",
						OrgWide:     false,
						CreatedAt:   time.Time{},
					},
				})

			// Init Nodes mock
			mockCtrl := gomock.NewController(t)
			mockerNodes := tt.args.nodesMap(mockCtrl)

			// call function under test
			_, err = CreateBorder0Config(tt.args.ctx, mockerNodes, tt.args.labname)

			// signal finish to mock
			mockCtrl.Finish()

			if (err != nil) != tt.wantErr {
				t.Errorf("createBorder0Config() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

// getNodeMap return a map of nodes for testing purpose.
func getNodeMap(mockCtrl *gomock.Controller) map[string]nodes.Node {
	// instantiate Mock Node 1
	mockNode1 := mocknodes.NewMockNode(mockCtrl)
	mockNode1.EXPECT().Config().Return(
		&types.NodeConfig{
			Image:     "alpine:3",
			ShortName: "node1",
			LongName:  "clab-TestTopo-node1",
		},
	).AnyTimes()

	// instantiate Mock Node 2
	mockNode2 := mocknodes.NewMockNode(mockCtrl)
	mockNode2.EXPECT().Config().Return(
		&types.NodeConfig{
			Image:     "alpine:3",
			ShortName: "node2",
			WaitFor:   []string{"node1"},
			Publish: []string{
				"tls/22",
				"tls/23/myfunnypolicy",
			},
			LongName: "clab-TestTopo-node2",
		},
	).AnyTimes()

	// instantiate Mock Node 3
	mockNode3 := mocknodes.NewMockNode(mockCtrl)
	mockNode3.EXPECT().Config().Return(
		&types.NodeConfig{
			Image:       "alpine:3",
			NetworkMode: "container:node2",
			ShortName:   "node3",
			WaitFor:     []string{"node1", "node2"},
			LongName:    "clab-TestTopo-node3",
		},
	).AnyTimes()

	// instantiate Mock Node 4
	mockNode4 := mocknodes.NewMockNode(mockCtrl)
	mockNode4.EXPECT().Config().Return(
		&types.NodeConfig{
			Image:           "alpine:3",
			MgmtIPv4Address: "172.10.10.1",
			ShortName:       "node4",
			NetworkMode:     "container:foobar",
		},
	).AnyTimes()

	// instantiate Mock Node 5
	mockNode5 := mocknodes.NewMockNode(mockCtrl)
	mockNode5.EXPECT().Config().Return(
		&types.NodeConfig{
			Image:           "alpine:3",
			MgmtIPv4Address: "172.10.10.2",
			ShortName:       "node5",
			WaitFor:         []string{"node3", "node4"},
			Publish: []string{
				"tls/22",
				"tls/25/test,additionalpolicy",
			},
			LongName: "clab-TestTopo-node5",
		},
	).AnyTimes()

	// nodeMap used for testing
	nodeMap := map[string]nodes.Node{}

	// nodemap is created with the node definition
	for _, x := range []nodes.Node{mockNode1, mockNode2, mockNode3, mockNode4, mockNode5} {
		// add node to nodemap
		nodeMap[x.Config().ShortName] = x
		// add node to dependencyManager
	}

	return nodeMap
}
