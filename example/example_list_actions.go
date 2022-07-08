/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/apache/openwhisk-client-go/whisk"
)

type UserRequest struct {
	model_type       string
	use_weight       bool
	key_service_port int
}

func main() {

	client, err := whisk.NewClient(http.DefaultClient, nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}

	body := map[string]interface{}{
		"name":             "world",
		"use_weight":       true,
		"key_service_port": 13571,
	}
	fmt.Printf("body: %v\n", body)

	// create action
	var action = new(whisk.Action)
	// action.Name = "test"
	action.Name = "testfm"
	action.Namespace = "_"
	concurrency := 1
	action.Limits = &whisk.Limits{Concurrency: &concurrency}
	action.Exec = &whisk.Exec{Kind: "blackbox", Image: "hugy718/wsk-blackbox-action:latest"}
	_, resp, err := client.Actions.InsertFm(action, false)
	// _, resp, err := client.Actions.Insert(action, false)

	// invoke action
	// res, resp, err := client.Actions.InvokeFm(action.Name, body, false, false)
	// res, resp, err := client.Actions.InvokeFm(action.Name, body, true, true)
	// res, resp, err := client.Actions.Invoke(action.Name, body, false, false)
	// res, resp, err := client.Actions.Invoke(action.Name, nil, true, true)

	// delete action
	// resp, err := client.Actions.DeleteFm(action.Name)
	// resp, err := client.Actions.Delete(action.Name)

	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}

	// fmt.Printf("returned result: %v\n", res)
	fmt.Printf("returned response: %v\n", resp)

}
