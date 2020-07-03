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
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
)

type RpcResult struct {
	Id      json.Number
	Result  json.Number
	Jsonrpc string
}

type Params struct {
	Id    string `json:"id"`
	Stack string `json:"stack"`
}

type ConnectFrames struct {
	Jsonrpc string  `json:"jsonrpc"`
	Method  string  `json:"method"`
	Params  *Params `json:"params"`
}

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
			url := "ws://127.0.0.1:4444/connect"

			c, _, err := websocket.DefaultDialer.Dial(url, nil)
			if err != nil {
				return errors.New("Eclipse Che machine exec is not running:" + err.Error())
			}
			defer c.Close()

			identifierContent := map[string]interface{}{
				"machineName": "tools",
				"workspaceId": workspaceId,
			}

			var command = []string{"sh", "-c", "go --version"}

			paramsContent := map[string]interface{}{
				"identifier": identifierContent,
				"cmd":        command,
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
			c.WriteMessage(websocket.TextMessage, connectJson)

			_, message, err := c.ReadMessage()
			fmt.Println("reply is=" + string(message))

			_, messageBytes, err := c.ReadMessage()
			fmt.Println("reply2 is=" + string(message))

			var rpcResult RpcResult
			err = json.Unmarshal(messageBytes, &rpcResult)
			if err != nil {
				return errors.New("Unable to unmarshal JSON:" + err.Error())
			}

			channelNumber := rpcResult.Result

			fmt.Println("result is " + string(channelNumber))

			connectDone := make(chan struct{})
			go func() {
				defer close(connectDone)
				for {
					_, message, err := c.ReadMessage()
					if err != nil {
						fmt.Println("read:", err)
						return
					}

					// parse JSON
					var connectFrames ConnectFrames
					err = json.Unmarshal(messageBytes, &connectFrames)
					if err != nil {
						fmt.Println("unmarshal error:", err)
						return
					}

					if connectFrames.Method == "onExecError" {
						fmt.Println("Received onExecError code, closing the socket...")
						fmt.Println(connectFrames.Params.Stack)
						// need to close
						c.Close()
						return
					}

					if connectFrames.Method == "onExecExit" {
						fmt.Println("Received onExecExit code, closing the socket...")
						// need to close
						c.Close()
						return
					}

					// onExecError
					// {"jsonrpc":"2.0","method":"onExecError","params":{"id":12,"stack":"command terminated with exit code 1"}}

					// {"jsonrpc":"2.0","method":"onExecExit","params":{"id":13}}

					fmt.Printf("recv: %s", message)
				}
			}()

			attachUrl := "ws://127.0.0.1:4444/attach/" + string(channelNumber)
			attachConnection, _, err := websocket.DefaultDialer.Dial(attachUrl, nil)
			if err != nil {
				return errors.New("Eclipse Che machine exec is not running:" + err.Error())
			}
			defer attachConnection.Close()

			interrupt := make(chan os.Signal, 1)
			signal.Notify(interrupt, os.Interrupt)
			done := make(chan struct{})
			go func() {
				defer close(done)
				for {
					_, message, err := attachConnection.ReadMessage()
					if err != nil {
						fmt.Println("read:", err)
						return
					}

					fmt.Printf("recv: %s", message)
				}
			}()

			//request := []byte("while true; do date; sleep 1; done\n")
			//request := []byte("sh -c 'go --version'")
			//attachConnection.WriteMessage(websocket.TextMessage, request)

			/*			_, messageReadBytes, err := attachConnection.ReadMessage()
						fmt.Println("reply3 is=" + string(messageReadBytes))
						_, messageReadBytes, err = attachConnection.ReadMessage()
						fmt.Println("reply4 is=" + string(messageReadBytes))
			*/
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-done:
					return nil
				/*case t := <-ticker.C:
				 fmt.Println("writing...." + t.String())
				 err := attachConnection.WriteMessage(websocket.TextMessage, []byte(t.String()))
				 if err != nil {
					 fmt.Println("write:", err)
					 return nil
				 }*/
				case <-interrupt:
					fmt.Println("interrupt the process, close socket")

					// Cleanly close the connection by sending a close message and then
					// waiting (with timeout) for the server to close the connection.
					err := attachConnection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
					if err != nil {
						fmt.Println("write close:", err)
						return nil
					}
					select {
					case <-done:
					case <-time.After(time.Second):
					}
					return nil
				}
			}

		},
	}
}

func init() {
	openCmd := NewCallCmd()
	rootCmd.AddCommand(openCmd)
}
