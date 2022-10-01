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
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apache/openwhisk-client-go/whisk"
)

type UserRequest struct {
	ModelType       string `json:"model_type"`
	UserId          string `json:"user_id"`
	KeyServiceAddr  string `json:"key_service_address"`
	KeyServicePort  int    `json:"key_service_port"`
	EncryptedSample string `json:"encrypted_sample"`
}

func (r *UserRequest) OwBodySerialize() (body map[string]interface{}) {
	return map[string]interface{}{
		"model_type":          r.ModelType,
		"user_id":             r.UserId,
		"key_service_address": r.KeyServiceAddr,
		"key_service_port":    r.KeyServicePort,
		"encrypted_sample":    r.EncryptedSample,
	}
}

func (r *UserRequest) Load(file string) (err error) {
	input, _ := os.ReadFile(file)
	return json.Unmarshal([]byte(input), r)
}

// a serverless platform client should implement APIs to be thread safe
type ServerlessClient interface {
	Init() error
	CreateAction(name, kind, image string, concurrency int) error
	InvokeAction(name string, req *UserRequest, rid int) (string, error)
	DeleteAction(name string) error
}

type OwClient struct {
	cli *whisk.Client
}

// thread safe
func (oc *OwClient) Init() error {
	var err error
	oc.cli, err = whisk.NewClient(http.DefaultClient, nil)
	return err
}

// thread safe
func (oc *OwClient) CreateAction(n, k, im string, c int) error {
	timeout := 300000
	action := whisk.Action{
		Name:      n,
		Namespace: "_",
		Limits:    &whisk.Limits{Concurrency: &c, Timeout: &timeout},
		Exec:      &whisk.Exec{Kind: k, Image: im},
	}
	_, resp, err := oc.cli.Actions.Insert(&action, false)
	log.Printf("create response (at %v): %v", time.Now().UnixMicro(), resp)
	return err
}

// thread safe
func (oc *OwClient) InvokeAction(n string, r *UserRequest, rid int) (string, error) {
	log.Printf("invoke (%v at %v)", rid, time.Now().UnixMicro())
	_, resp, err := oc.cli.Actions.Invoke(n, r.OwBodySerialize(), true, false)
	log.Printf("invoke response (%v at %v): %v", rid, time.Now().UnixMicro(), resp)
	return fmt.Sprintf("invoke response: %v", resp), err
}

// thread safe
func (oc *OwClient) DeleteAction(n string) error {
	resp, err := oc.cli.Actions.Delete(n)
	log.Printf("delete response (at %v): %v", time.Now().UnixMicro(), resp)
	return err
}

func handle_error(err error) bool {
	if err == nil {
		return false
	}
	fmt.Print(err)
	return true
}

// warm_up gradually increase invoke rate to the target
// 	such that container are not scaled up all at once.
// step should be set as the concurrency value
// target is the action per seconds
func warm_up(start, step, target int, cli ServerlessClient, action_name string,
	req *UserRequest) {
	for rate := start; rate < target; rate += step {
		fmt.Printf("Warming up(rate: %v)......", rate)
		var wg sync.WaitGroup
		for count := 0; count < rate; count += 1 {
			wg.Add(1)

			go func(id int) {
				defer wg.Done()
				_, err := cli.InvokeAction(action_name, req, id)
				handle_error(err)
			}(count)
		}
		wg.Wait()
	}
	fmt.Println("Warm up done")
}

// benchmark system warm latency
// 	start_rate start latency
// 	end_rate
// 	step: step to take measurement
//  run_duration: at each measurement how many seconds it is run.
//  warm_up_step: step to warm up instances
// Usage: benchmark_warm_lat(20, 200,)
func benchmark_warm_lat(start_rate, end_rate, step, run_duration,
	warm_up_step int) {

}

func main() {

	// setup client
	client := OwClient{}
	err := client.Init()
	if handle_error(err) {
		os.Exit(-1)
	}

	// select action
	// action_name := "crtw-tvm-resnet-4"
	action_name := "crtw-tvm-mb-4"

	// create action
	// client.CreateAction(action_name, "blackbox", "hugy718/wsk-blackbox-action:latest", 4)

	req := UserRequest{}
	// req.Load("tvm_rs_req.json")
	req.Load("tvm_mb_req.json")
	req.KeyServiceAddr = "10.10.10.227"

	// warm_up(4, 16, &client, action_name, &req)
	var has_error int32
	for has_error = 1; has_error != 0; {
		has_error = 0
		fmt.Println("last round failed retrying...")
		// invoke action
		var wg sync.WaitGroup
		for count := 0; count < 128; count += 1 {
			wg.Add(1)

			go func(id int) {
				defer wg.Done()
				_, err := client.InvokeAction(action_name, &req, id)
				handle_error(err)
				if err != nil {
					atomic.StoreInt32(&has_error, 1)
				}
			}(count)
		}
		wg.Wait()
	}
}
