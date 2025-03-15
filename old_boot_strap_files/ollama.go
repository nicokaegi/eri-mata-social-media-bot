package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ollama/ollama/api"
)

var messages []api.Message = []api.Message{
	api.Message{
		Role:    "system",
		Content: "Provide very brief, concise responses",
	},
	api.Message{
		Role:    "user",
		Content: "Name some unusual animals",
	},
	api.Message{
		Role:    "assistant",
		Content: "Monotreme, platypus, echidna",
	},
	api.Message{
		Role:    "user",
		Content: " now give me 200 words",
	},
}

var ctx context.Context = context.Background()

func test() {

	client, err := api.ClientFromEnvironment()
	if err != nil {
		log.Fatal(err)
	}

	respFunc := func(resp api.ChatResponse) error {
		fmt.Print(resp.Message.Content)
		return nil
	}

	req := &api.ChatRequest{
		Model:    "llama3.2:1b-instruct-q2_K",
		Messages: messages,
		Stream:   new(bool),
	}

	err = client.Chat(ctx, req, respFunc)
	if err != nil {
		log.Fatal(err)
	}

}

func main() {

	test()

}
