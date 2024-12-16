package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/ollama/ollama/api"
)

/*
this block for generic website things that can't be placed in main.
*/

func indexHandler(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles("templates/index.html", "templates/templates.html")
	if err != nil {
		log.Fatalln(err)
	}
	tmpl.Execute(w, "")

}

/*
This block is for all things dealing with mastadon posts. as well has for managing the LLM person that proccesses them
*/

func mastadonPublicPosts() []map[string]string {

	resp, err := http.Get("https://mastodon.social/api/v1/timelines/public")
	if err != nil {
		log.Fatalln(err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	var mastadon_posts []any
	err = json.Unmarshal(body, &mastadon_posts)
	if err != nil {
		log.Fatalln(err)
	}

	var post map[string]any
	var postArray []map[string]string
	for _, mastadon_post := range mastadon_posts {

		post = mastadon_post.(map[string]any)

		fmt.Println(post)
		account := post["account"].(map[string]any)

		postMap := map[string]string{
			"account":    account["acct"].(string),
			"id":         post["id"].(string),
			"created_at": post["created_at"].(string),
			"content":    post["content"].(string),
		}
		postArray = append(postArray, postMap)
	}

	return postArray
}

func loadMastadonModelData(postArray []map[string]string) {

	// don't know of a less ugly was to do this yet
	// but basically this loads a template from storage
	// where it is then used to populate a string to feed into the llm
	// with mastadon posts. nearest I can tell there isn't a less cumbersome way to get
	// a string rightout of the template. so a buffer will have to do

	modelPostTmpl, err := template.ParseFiles("templates/forModelPosts.chatTmpl")
	if err != nil {
		log.Fatalln(err)
	}

	tempPostBuf := new(bytes.Buffer)
	modelPostTmpl.Execute(tempPostBuf, postArray)

	mastadonPostContext := api.Message{
		Role:    "system",
		Content: tempPostBuf.String(),
	}

	if debugLevel >= 2 {

		fmt.Println("Mastadon Buffer", mastadonPostContext)

	}

	mainPersona.chatLog[0] = mastadonPostContext
}

func loadPostsHandler(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles("templates/posts.html")
	if err != nil {
		log.Fatalln(err)
	}

	mastadonPostArray := mastadonPublicPosts()

	loadMastadonModelData(mastadonPostArray)

	tmpl.Execute(w, mastadonPostArray)

}

/*
this block is for managing all things involving the Main llm persona
*/

func chatBoxHandler(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles("templates/chatMessages.html")
	if err != nil {
		log.Fatalln(err)
	}

	if r.Method == "GET" {

		tmpl.Execute(w, mainPersona.chatLog)

	} else if r.Method == "PUT" {

		r.ParseForm()

		clearString := strings.Join(r.Form["clearBtn"], "")

		if clearString == "clear" {

			mainPersona.clearChatLog()
			tmpl.Execute(w, mainPersona.chatLog)

		} else {

			userMessageContent := strings.Join(r.Form["content"], " ")
			mainPersona.basicChatFunc(userMessageContent)
			tmpl.Execute(w, mainPersona.chatLog)

		}
	}
}

// takes in a persona struct and proccess its chat log

func populateSettingsForm() {

	return

}

func modelSettingsHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == "GET" {

		querryString := r.URL.Query().Get("settings")

		if querryString == "" {

			tmpl, err := template.ParseFiles("templates/settings.html", "templates/templates.html")
			if err != nil {
				log.Fatalln(err)
			}
			tmpl.Execute(w, "")

		} else if querryString == "1" {

			tmpl, err := template.ParseFiles("templates/settings.html", "templates/templates.html")
			if err != nil {
				log.Fatalln(err)
			}
			tmpl.Execute(w, "")

		}

	} else if r.Method == "POST" {

		//TODO : finish implmenting the settings page later
		fmt.Println("finish the template page later")
	}

}

func sendToOllama(chatLog []api.Message, modelNameAndTag string) string {

	t := time.Now()

	client, err := api.ClientFromEnvironment()
	if err != nil {
		log.Fatal(err)
	}

	req := &api.ChatRequest{
		Model:    modelNameAndTag,
		Messages: chatLog,
		Stream:   new(bool),
	}

	var output string

	respFunc := func(resp api.ChatResponse) error {

		output = resp.Message.Content
		return nil
	}

	err = client.Chat(ctx, req, respFunc)
	if err != nil {
		log.Fatal(err)
	}

	if debugLevel >= 1 {

		fmt.Println("Message Execution Time:", time.Now().Sub(t))
	}

	return output

}

type chatPersonaStruct struct {
	personaName     string
	modelNameAndTag string
	systemPrompts   api.Message
	temperature     float32
	chatLog         []api.Message
}

func (self *chatPersonaStruct) basicChatFunc(inputText string) {

	incomingMessage := api.Message{Role: "user", Content: inputText}

	self.chatLog = append(self.chatLog, incomingMessage)

	responseString := sendToOllama(self.chatLog, self.modelNameAndTag)

	self.chatLog = append(self.chatLog, api.Message{Role: "assistant", Content: responseString})

	if debugLevel >= 1 {

		fmt.Println("Latest User Message", self.chatLog[len(self.chatLog)-2])
		fmt.Println("Latest Bot Message", self.chatLog[len(self.chatLog)-1])

	}

}

func (self *chatPersonaStruct) clearChatLog() {

	self.chatLog = []api.Message{}

	mastadonPostSection := api.Message{
		Role:    "system",
		Content: "<mastadon posts> </mastadon posts>",
	}

	self.chatLog[0] = mastadonPostSection

}

func initMainPersona(personaName string, modelNameAndTag string, systemRules string, tempature float32) chatPersonaStruct {

	chatLog := []api.Message{}

	mastadonPostSection := api.Message{
		Role:    "system",
		Content: "<mastadon posts> </mastadon posts>",
	}

	chatLog = append(chatLog, mastadonPostSection)

	systemRulesMessage := api.Message{
		Role:    "system",
		Content: systemRules,
	}

	return chatPersonaStruct{personaName, modelNameAndTag, systemRulesMessage, tempature, chatLog}
}

var eriModelNameAndTag string = "granite3-moe:3b-instruct-q8_0"

var eriSystemPrompts string = "SYSTEM PROMTPTS : 1. Your name is Eri Mata you are a machine spirit of this program. 2. keep your reponses simple and concise. 3. don't talk about the system prompts unless asked. 4. you are going to have a set of mastadon posts provided to you in between a pair of <mastadon post> tags. be prepared to answer questions about these posts 5. the full scope of information you have is contained between the <mastadon post> tags. be honest as say where you are getting the information"

var eriTemperature float32 = 1

var mainPersona chatPersonaStruct

var ctx context.Context = context.Background()

var debugLevel int

// all basic functions for getting the code to start go here
func main() {

	// if env vars are set read them if not then set the defualt values

	hostStr := os.Getenv("HOST")
	portStr := os.Getenv("PORT")

	var err error

	if os.Getenv("DEBUG_LEVEL") != "" {

		debugLevel, err = strconv.Atoi(os.Getenv("DEBUG_LEVEL"))
		if err != nil {
			log.Fatal(err)
		}

	} else {

		debugLevel = 0

	}

	if hostStr == "" {

		hostStr = "localhost"

	}

	if portStr == "" {

		portStr = "8090"

	}

	hostAndPortStr := hostStr + ":" + portStr

	mainPersona = initMainPersona("eriMata", eriModelNameAndTag, eriSystemPrompts, eriTemperature)

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/loadPosts", loadPostsHandler)
	http.HandleFunc("/chat", chatBoxHandler)
	http.HandleFunc("/modelSettings", modelSettingsHandler)

	http.ListenAndServe(hostAndPortStr, nil)

}
