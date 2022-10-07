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
	InvokeAction(name string, req *UserRequest, rid int) (string, string, error)
	GetResult(rid int, resp_id string) (string, error)
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
func (oc *OwClient) InvokeAction(n string, r *UserRequest, rid int) (string, string, error) {
	log.Printf("invoke (%v at %v)", rid, time.Now().UnixMicro())
	wskresp, resp, err := oc.cli.Actions.Invoke(n, r.OwBodySerialize(), false, false)
	log.Printf("invoke response (%v at %v): %v", rid, time.Now().UnixMicro(), resp)
	resp_id := fmt.Sprintf("%v", wskresp["activationId"])
	return resp_id, fmt.Sprintf("invoke response: %v", resp), err
}

func (oc *OwClient) GetResult(rid int, resp_id string) (string, error) {
	// log.Printf("get response : %v", resp_id)
	_, resp, err := oc.cli.Activations.Get(resp_id)
	retry_count := 18
	for err != nil {
		retry_count--
		if resp.Status != "404 Not Found" {
			break
		}
		_, resp, err = oc.cli.Activations.Get(resp_id)
		// loop to fetch. The activation will return not found before it is finished.
		if retry_count <= 0 {
			log.Printf("invoke response (%v at %v): exhausted retries", rid, time.Now().UnixMicro())
			break
		}
		time.Sleep(time.Duration(2) * time.Second)
	}
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

func send_con_req(num int, ctx *BenchmarkCtx) {
	var wg sync.WaitGroup
	for count := 0; count < num; count += 1 {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()
			resp_id, _, err := ctx.cli.InvokeAction(ctx.action_name, ctx.req, id)
			if err == nil {
				_, err = ctx.cli.GetResult(id, resp_id)
			}
			handle_error(err)
		}(count)
	}
	wg.Wait()

}

// warm_up gradually increase invoke rate to the target
// 	such that container are not scaled up all at once.
// step should be set as the concurrency value
// target is the action per seconds
func warm_up(start, step, target int, ctx *BenchmarkCtx) {
	for rate := start; rate < target+2*step; rate += step {
		log.Printf("Warming up(rate: %v)......", rate)
		send_con_req(rate, ctx)
	}
	log.Println("Warm up done")
}

type BenchmarkCtx struct {
	cli         ServerlessClient
	action_name string
	req         *UserRequest
}

// benchmark system warm latency
// 	start_rate start latency
// 	end_rate
// 	step: step to take measurement
// 	run_duration: at each measurement how many seconds it is run.
// 	warm_up_step: step to warm up instances
// Usage: benchmark_warm_lat(20, 200,)
func benchmark_step_warm_lat(start_rate, end_rate, step int, run_duration float64,
	warm_up_step int, ctx *BenchmarkCtx) {
	// print the setting
	log.Printf("Benchmarking warm latency:\n start rate: %v req/s\n end rate: %v req/s\n step: %v\n run duration: %vmins\n warmup step: %v\n action name %v\n", start_rate, end_rate, step, run_duration, warm_up_step, ctx.action_name)

	has_error := int32(0)
	cur_rate := 0
	for target_rate := start_rate; target_rate <= end_rate; target_rate += step {
		warm_up(cur_rate, warm_up_step, target_rate, ctx)
		cur_rate = target_rate
		log.Printf("testing at rate: %d\n", target_rate)
		id := 0
		var wg sync.WaitGroup
		inter_arrival := 1.0 / float64(target_rate)
		start_time := time.Now().UnixMicro()
		for t := 0.0; t < run_duration*60.0; t += inter_arrival {
			wg.Add(1)
			now := time.Now().UnixMicro()
			if start_time+int64(t*1e6) > now {
				sleep_duration := start_time + int64(t*1e6) - now
				time.Sleep(time.Duration(sleep_duration) * time.Microsecond)
			}
			go func(id int) {
				defer wg.Done()
				resp_id, _, err := ctx.cli.InvokeAction(ctx.action_name, ctx.req, id)
				if err == nil {
					_, err = ctx.cli.GetResult(id, resp_id)
				}
				handle_error(err)
				if err != nil {
					atomic.StoreInt32(&has_error, 1)
				}
			}(id)
			id++
			// skip subsequent requests
			if has_error > 0 {
				break
			}
		}
		wg.Wait()
		// skip later rounds
		if has_error > 0 {
			break
		}
	}
}

func benchmark_warm_at_fixed_provision(pc_right, keep_warm_time int,
	ctx *BenchmarkCtx) {
	for pc := pc_right; pc > 0; pc-- {
		for tc := pc; tc > 0; tc-- {
			log.Printf("Provisioned concurrency: %d", pc)
			send_con_req(pc, ctx)
			log.Printf("Test concurrency: %d", tc)
			for rep_time := 0; rep_time < 3; rep_time++ {
				log.Printf("Repetition: %d", rep_time)
				send_con_req(tc, ctx)
			}
		}
		time.Sleep(time.Duration(keep_warm_time) * time.Minute)
	}
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

	// tvm-mb
	// action_name := "crtw-tvm-mb-1"
	// action_name := "crtw-tvm-mb-4"
	// action_name := "citw-tvm-mb-4"
	// action_name := "natw-tvm-mb-1"
	// action_name := "nutw-tvm-mb-1"
	// tflm-mb
	// action_name := "crtw-tflm-mb-1"
	action_name := "crtw-tflm-mb-4"
	// action_name := "citw-tflm-mb-4"
	// action_name := "natw-tflm-mb-1"
	// action_name := "nutw-tflm-mb-1"

	// create action
	// client.CreateAction(action_name, "blackbox", "hugy718/wsk-blackbox-action:latest", 4)

	req := UserRequest{}

	// req.Load("tvm_rs_req.json")

	// tvm mb
	// req.Load("tvm_mb_req.json")
	// req.Load("tvm_mb_req_nenc.json")

	// tflm mb
	req.Load("tflm_mb_req.json")
	// req.Load("tflm_mb_req_nenc.json")

	req.KeyServiceAddr = "10.10.10.227"

	// var has_error int32
	// for has_error = 1; has_error != 0; {
	// 	has_error = 0
	// 	fmt.Println("last round failed retrying...")
	// 	// invoke action
	// 	var wg sync.WaitGroup
	// 	for count := 0; count < 7; count += 1 {
	// 		wg.Add(1)

	// 		go func(id int) {
	// 			defer wg.Done()
	// 			resp_id, _, err := client.InvokeAction(action_name, &req, id)
	// 			if err == nil {
	// 				log.Printf("activation id: %v", resp_id)
	// 				_, err = client.GetResult(id, resp_id)
	// 			}
	// 			handle_error(err)
	// 			if err != nil {
	// 				atomic.StoreInt32(&has_error, 1)
	// 			}
	// 		}(count)
	// 	}
	// 	wg.Wait()
	// }

	ctx := BenchmarkCtx{&client, action_name, &req}
	benchmark_warm_at_fixed_provision(20, 2, &ctx)

	log.Print("switch testing to crtw-tflm--mb-1")
	ctx.action_name = "crtw-tflm-mb-1"
	benchmark_warm_at_fixed_provision(20, 2, &ctx)

	// benchmark_step_warm_lat(2, 20, 2, 1, 4, &ctx)
}
