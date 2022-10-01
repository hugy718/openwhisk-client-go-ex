package main

import (
	"encoding/json"
	"fmt"
	"os"
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

func main() {
	body := UserRequest{
		ModelType:       "mobilenet_v1_1.0_224",
		UserId:          "admin",
		KeyServiceAddr:  "10.10.10.227",
		KeyServicePort:  13571,
		EncryptedSample: "ab6teZMD",
	}

	output, _ := json.Marshal(body)

	_ = os.WriteFile("test.json", output, 0644)
	unmarshaled := UserRequest{}
	input, _ := os.ReadFile("test.json")
	json.Unmarshal([]byte(input), &unmarshaled)
	fmt.Printf("%v", unmarshaled)
	es, _ := os.ReadFile("es224.txt")

	body2 := UserRequest{
		ModelType:       "mobilenet_v1_1.0_224",
		UserId:          "admin",
		KeyServiceAddr:  "10.10.10.227",
		KeyServicePort:  13571,
		EncryptedSample: string(es),
	}
	output, _ = json.Marshal(body2)
	_ = os.WriteFile("test.json", output, 0644)
}
