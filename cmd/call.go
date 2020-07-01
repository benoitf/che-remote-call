/*********************************************************************
 * Copyright (c) 2020 Red Hat, Inc.
 *
 * This program and the accompanying materials are made
 * available under the terms of the Eclipse Public License 2.0
 * which is available at https://www.eclipse.org/legal/epl-2.0/
 *
 * SPDX-License-Identifier: EPL-2.0
 **********************************************************************/

package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/net/websocket"
)

func NewCallCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "call",
		Short:        "Call a remote command in a separate container",
		Long:         "Call a remote command in a separate container",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("The container argument is required")
			}

			// grab URL for theia endpoint
			workspaceId, defined := os.LookupEnv("CHE_WORKSPACE_ID")
			if !defined {
				return errors.New("CHE_WORKSPACE_ID is not defined as environment variable")
			}

			/*cheApiToken, defined := os.LookupEnv("CHE_MACHINE_TOKEN")
			if !defined {
				return errors.New("CHE_MACHINE_TOKEN is not defined as environment variable")
			}*/

			// connect URL to exec
			origin := "http://localhost"
			url := "ws://127.0.0.1:4444/connect"
			ws, err := websocket.Dial(url, "", origin)
			if err != nil {
				return errors.New("Eclipse Che machine exec is not running:" + err.Error())
			}

			defer ws.Close()

			identifierContent := map[string]interface{}{
				"machineName": "go-cli",
				"workspaceId": workspaceId,
			}

			var empty []string = nil

			paramsContent := map[string]interface{}{
				"identifier": identifierContent,
				"cmd":        empty,
				"cols":       80,
				"rows":       24,
				"tty":        false,
			}

			connectMessage := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      0,
				"method":  "create",
				"params":  paramsContent,
			}
			connectJson, err := json.Marshal(connectMessage)
			if err != nil {
				return errors.New("Unable to marshal JSON:" + err.Error())
			}
			fmt.Println("json to send=", string(connectJson))
			ws.Write(connectJson)

			var reply []byte
			ws.Read(reply)

			fmt.Println("reply is=", string(reply))
			return nil

		},
	}
}

func init() {
	openCmd := NewCallCmd()
	rootCmd.AddCommand(openCmd)
}
