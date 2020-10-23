package main

import (
	"fmt"
	"net/http"
	"strings"

	"os"
	"sync"
	"time"

	"github.com/caser/gophernews"
	"github.com/jzelinskie/geddit"
)

const (
	SOURCE_HACKERNEWS = "HackerNews"
	SOURCE_REDDIT     = "Reddit"
)

type Story struct {
	Title  string
	Author string
	Url    string
	Source string
}

var stories []Story

var redditSession *geddit.LoginSession
var hackerNewsClient *gophernews.Client

func init() {

	hackerNewsClient = gophernews.NewClient()

	var err error

	redditSession, err = geddit.NewLoginSession("username_does_not_exists","nor_does_the_password","gopherbot agent01")

	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func main() {
	go manageStories()

	http.HandleFunc("/",all)
	http.HandleFunc("/search",search)

	if err := http.ListenAndServe(":8080",nil) ; err != nil {
		panic(err)
	}
}

func manageStories() {
	for {
		hnChannel 			:= make(chan Story, 8)
		redditChannel    	:= make(chan Story, 8)
		storyWritterChannel := make(chan Story, 8)

		go hnStories(hnChannel)
		go redditStories(redditChannel)
		go writeToStories(storyWritterChannel)

		hnOpen     := true
		redditOpen := true

		for hnOpen || redditOpen {
			select {
			case story, open := <-hnChannel:
				if open {
					storyWritterChannel <- story
				} else {
					hnOpen = false
				}
			case story, open := <-redditChannel:
				if open {
					storyWritterChannel <- story
				} else {
					redditOpen = false
				}
			}
		}

		time.Sleep(20 * time.Second)
	}
}


func hnStories(c chan<- Story) {

	defer close(c)

	changes, err := hackerNewsClient.GetChanges()

	if err != nil {
		fmt.Println(err.Error())
		return
	}

	var wg sync.WaitGroup

	for _, id := range changes.Items {
		wg.Add(1)

		go getHnStory(id, c, &wg)
	}

	wg.Wait()
}

func getHnStory(id int, c chan<- Story, wg *sync.WaitGroup) {
	defer wg.Done()

	story, err := hackerNewsClient.GetStory(id)

	if err != nil {
		return
	}
	newStory := Story{
		Title:  story.Title,
		Url:    story.URL,
		Author: story.By,
		Source: SOURCE_HACKERNEWS,
	}

	c <- newStory
}


func redditStories(c chan<- Story) {
	defer close(c)

	sort := geddit.PopularitySort(geddit.NewSubmissions)

	var listingOptions geddit.ListingOptions
	listingOptions.Limit = 100

	submissions, err := redditSession.SubredditSubmissions("programming", sort, listingOptions)

	if err != nil {
		fmt.Print(err.Error())
		return
	}

	for _, s := range submissions {
		newStory := Story{
			Title:  s.Title,
			Url:    s.URL,
			Author: s.Author,
			Source: SOURCE_REDDIT,
		}

		c <- newStory
	}
}

func writeToStories(c <-chan Story) {
	for {
		stories = append(stories, <- c)
	}
}

func searchInStories(query string) []Story {
	var foundStories []Story

	for _, story := range stories {
		if strings.Contains(strings.ToUpper(story.Title), strings.ToUpper(query)) {
			foundStories = append(foundStories, story)
		}
	}

	return foundStories
}

func search(w http.ResponseWriter, r *http.Request) {
	query := r.FormValue("q")

	if query == "" {
		http.Error(w,"Search parameter was not given", http.StatusNotAcceptable)
	}

	w.Write(htmlHead())

	s := searchInStories(query)

	if len(s) == 0 {
		w.Write([]byte(
			fmt.Sprintf("<div class='p-5 bg-red-300 text-white'>Sorry, no results with '%s'\n.</div>",query)))
	} else {
		for _, story := range s {
			w.Write(htmlStory(story))
		}
	}
	w.Write([]byte("<br><a href='../'>Back</a>"))
	w.Write(htmlTail())
}

func topTen(w http.ResponseWriter, r *http.Request) {
	w.Write(htmlHead())

	w.Write(htmlForm())

	for i := len(stories) - 1; i >= 0 && len(stories) - i < 10; i-- {
		w.Write(htmlStory(stories[i]))
	}

	w.Write(htmlTail())
}

func all(w http.ResponseWriter, r *http.Request) {
	w.Write(htmlHead())

	w.Write(htmlForm())

	for i := len(stories) - 1; i >= 0 ; i-- {
		w.Write(htmlStory(stories[i]))
	}

	w.Write(htmlTail())
}

func htmlHead() []byte {
	head := `<html><head><title>Go ❤️‍🔥 </title><link href='https://unpkg.com/tailwindcss@^1.0/dist/tailwind.min.css' rel='stylesheet'></head><body><div class='w-1/2 mx-auto p-4 mt-10'>
	<div class='p-5'>
		Salam, this is a demo app, a reddit-hackernews client with go and tailwind.
		Enjoy 🥳
	</div>`

	return []byte(head)
}

func htmlTail() []byte {
	return []byte(`</div></body></html>`)
}

func htmlForm() []byte {
	return []byte(`<form action='search' class='w-full'>
	<div class='flex items-center border-b border-gray-500 py-2'>
		<input name='q' class='appearance-none bg-transparent border-none w-full text-gray-700 mr-3 py-1 px-2 leading-tight focus:outline-none' type='text' placeholder='Looking for a story at HackerNews or Reddit?' aria-label='Search' required>
		<button class='flex-shrink-0 bg-gray-500 hover:bg-gray-700 border-gray-500 hover:border-gray-700 text-sm border-4 text-white py-1 px-2 rounded' type='submit'>
			Search
		</button>
	</div>
</form>`)
}

func htmlStory(story Story) []byte {
	html := fmt.Sprintf(`<div class='shadow-md border-b p-4'>
	<span class='font-bold'>#%s | </span> <a href='%s' target='_blank' class='text-gray-600'>%s</a>
	<br>
	- <span class='font-light'>%s</span>
</div>`,story.Source,story.Url,story.Title,story.Author)

	return []byte(html)
}