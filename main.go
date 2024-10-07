package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"text/template"
)

func indexFunc(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		log.Fatalln(err)
	}
	tmpl.Execute(w, "")

}

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
		postMap := map[string]string{
			"id":         post["id"].(string),
			"created_at": post["created_at"].(string),
			"content":    post["content"].(string),
		}
		postArray = append(postArray, postMap)
	}

	return postArray
}

func loadPostsFunc(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles("templates/posts.html")
	if err != nil {
		log.Fatalln(err)
	}

	mastadonPostArray := mastadonPublicPosts()

	tmpl.Execute(w, mastadonPostArray)

}

var mainLog []map[string]string = make([]map[string]string, 0)

func chatFunc(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles("templates/chatMessages.html")
	if err != nil {
		log.Fatalln(err)
	}

	if r.Method == "GET" {

		tmpl.Execute(w, mainLog)

	} else if r.Method == "POST" {

		r.ParseForm()
		content := strings.Join(r.Form["content"], " ")

		mainLog = append(mainLog, map[string]string{"role": "user", "content": content})
		mainLog = append(mainLog, map[string]string{"role": "assistant", "content": "hi im eri mata"})

		tmpl.Execute(w, mainLog)

	}
}

func main() {

	mainLog = append(mainLog, map[string]string{"role": "assistant", "content": "hi im eri mata"})

	http.HandleFunc("/", indexFunc)
	http.HandleFunc("/loadPosts", loadPostsFunc)
	http.HandleFunc("/chat", chatFunc)
	http.ListenAndServe(":8090", nil)

}
