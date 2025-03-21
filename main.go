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

	tmpl, err := template.ParseFiles("templates/index.html", "templates/templates.html", "templates/forms.html")
	if err != nil {
		log.Fatalln(err)
	}

	if debugLevel >= 1 {

		fmt.Println("Template Name:", tmpl.Name)
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

		fmt.Println(postArray)
	}

	return postArray
}

func loadMastadonModelData(postArray []map[string]string) {

	// don't know of a less ugly was to do this yet
	// but basically this loads a template from disk
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

var mastadonPostArray []map[string]string = []map[string]string{}

func loadPostsHandler(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles("templates/fragments.html")
	if err != nil {
		log.Fatalln(err)
	}

	if debugLevel >= 1 {

		fmt.Println("Template Name:", tmpl.Name)
	}

	mastadonPostArray := mastadonPublicPosts()

	//loadMastadonModelData(mastadonPostArray)

	tmpl.ExecuteTemplate(w, "mastadonPostFragment", mastadonPostArray)

}

/*
this block is for managing all things involving the Main llm persona

expects three diffrent requests.

Gets from the message history box

Puts from the text box

Posts from the clear button

*/

func chatBoxHandler(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles("templates/chatMessages.html")
	if err != nil {
		log.Fatalln(err)
	}

	if r.Method == "GET" {

		tmpl.Execute(w, mainPersona.chatLog)

	} else if r.Method == "POST" {

		r.ParseForm()

		userMessageContent := strings.Join(r.Form["userMessageBox"], " ")

		// not sure if this should go into a
		// seperate helper function, but I see no issues with this so far
		go mainPersona.basicChatFunc(userMessageContent)

		fmt.Fprint(w, "")

		/*
			r.ParseForm()

			err := r.ParseForm()
			if err != nil {
				fmt.Println(err)

			}

			mainPersona = initMainPersona("eriMata", eriModelNameAndTag, eriSystemPrompts, eriTemperature)
		*/
	}

}

//TODO redo settings page
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

			tmpl, err := template.ParseFiles("templates/settings.html", "templates/forms.html")
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

func getToolStruct() []api.Tool {

	var postSearchTool api.ToolFunction = api.ToolFunction{}

	postSearchTool.Name = "make_search_query"
	postSearchTool.Description = "Makes a search query to a semantic seaerch database"
	postSearchTool.Parameters.Type = "object"
	postSearchTool.Parameters.Required = []string{"query", "number_of_posts"}

	postSearchTool.Parameters.Properties = make(map[string]struct {
		Type        string   `json:"type"`
		Description string   `json:"description"`
		Enum        []string `json:"enum,omitempty"`
	})

	property := postSearchTool.Parameters.Properties["query"]
	property.Type = "string"
	property.Description = "a full sentence querry to a semantic search database"
	postSearchTool.Parameters.Properties["query"] = property

	property = postSearchTool.Parameters.Properties["number_of_posts"]
	property.Type = "int"
	property.Description = "the maximum number of posts to return from the search engine"
	postSearchTool.Parameters.Properties["querymaximum"] = property

	var getPostTool api.ToolFunction = api.ToolFunction{}

	getPostTool.Name = "get_posts"
	getPostTool.Description = "give a number and return some posts from mastadon"
	getPostTool.Parameters.Type = "object"
	getPostTool.Parameters.Required = []string{"number_of_posts"}

	getPostTool.Parameters.Properties = make(map[string]struct {
		Type        string   `json:"type"`
		Description string   `json:"description"`
		Enum        []string `json:"enum,omitempty"`
	})

	property = getPostTool.Parameters.Properties["number_of_posts"]
	property.Type = "int"
	property.Description = "the number of posts to get back from mastadon. if the user doesn't say how many just make a reasonable guess of how many to get"
	getPostTool.Parameters.Properties["querymaximum"] = property

	return []api.Tool{api.Tool{Type: "function", Function: postSearchTool}, api.Tool{Type: "function", Function: getPostTool}}

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
		Tools:    getToolStruct(),
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
	messageQueue    []string
}

func (self *chatPersonaStruct) startQueueLoop() {

	for {

		if len(self.messageQueue) != 0 {

			incomingMessage := api.Message{Role: "user", Content: self.messageQueue[0]}

			self.messageQueue = self.messageQueue[1:]

			self.chatLog = append(self.chatLog, incomingMessage)

			responseString := sendToOllama(self.chatLog, self.modelNameAndTag)

			self.chatLog = append(self.chatLog, api.Message{Role: "assistant", Content: responseString})

			if debugLevel >= 2 {

				fmt.Println("Latest User Message", self.chatLog[len(self.chatLog)-2])
				fmt.Println("Latest Bot Message", self.chatLog[len(self.chatLog)-1])

			}

		}
	}
}

func (self *chatPersonaStruct) basicChatFunc(inputText string) {

	self.messageQueue = append(self.messageQueue, inputText)
}

func initMainPersona(personaName string, modelNameAndTag string, systemRules string, tempature float32) chatPersonaStruct {

	chatLog := []api.Message{}

	systemRulesMessage := api.Message{
		Role:    "system",
		Content: systemRules,
	}

	chatLog = append(chatLog, systemRulesMessage)

	return chatPersonaStruct{personaName, modelNameAndTag, systemRulesMessage, tempature, chatLog, []string{}}
}

var eriModelNameAndTag string = "granite3.1-moe:3b-instruct-q8_0"

var eriSystemPrompts string = "SYSTEM PROMTPTS : Your name is mothera you are the chat user interface for this web app. your job is to make conversation and tool calls to complete certain tasks"

var eriTemperature float32 = 1

var mainPersona chatPersonaStruct

var ctx context.Context = context.Background()

var debugLevel int

/*

to make the running of multiple presonas more cordinated, and to make the display of messages/website content
less depended on whenever ollama returns anything.

basically instead of calling the persona directly each function call is wrapped in another function and added to a queue to
be executed later

*/

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

	fmt.Println(hostAndPortStr)

	mainPersona = initMainPersona("eriMata", eriModelNameAndTag, eriSystemPrompts, eriTemperature)

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/loadPosts", loadPostsHandler)
	http.HandleFunc("/chat", chatBoxHandler)
	//http.HandleFunc("/modelSettings", modelSettingsHandler)

	go mainPersona.startQueueLoop()

	http.ListenAndServe(hostAndPortStr, nil)

}
