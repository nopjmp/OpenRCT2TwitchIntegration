package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/nicklaw5/helix"
)

func rateLimitCallback(lastResponse *helix.Response) error {
	if lastResponse.GetRateLimitRemaining() > 0 {
		return nil
	}

	var reset64 int64
	reset64 = int64(lastResponse.GetRateLimitReset())

	currentTime := time.Now().Unix()

	if currentTime < reset64 {
		timeDiff := time.Duration(reset64 - currentTime)
		if timeDiff > 0 {
			fmt.Printf("Waiting on rate limit to pass before sending next request (%d seconds)\n", timeDiff)
			time.Sleep(timeDiff * time.Second)
		}
	}

	return nil
}

var client *helix.Client
var userLoginMap sync.Map

type response struct {
	Status int `json:"status"`
}

type viewer struct {
	Name       string `json:"name"`
	InChat     bool   `json:"inChat"`
	IsFollower bool   `json:"isFollower"`
	IsMod      bool   `json:"isMod"`
}

func joinRequest(w http.ResponseWriter, r *http.Request) {
	// ignoring for now
	js, err := json.Marshal(response{Status: 200})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

const tmiBase = "http://tmi.twitch.tv/group/user/"
const chatters = "/chatters"

// undocumented api....
func getChatters(channel string) (map[string]*viewer, error) {
	client := http.Client{
		Timeout: time.Second * 1,
	}
	req, err := http.NewRequest(http.MethodGet, tmiBase+channel+chatters, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Client-ID", os.Getenv("CLIENT_ID"))

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var tmp struct {
		Chatters struct {
			Vips       []string `json:"vips"`
			Moderators []string `json:"moderators"`
			Staff      []string `json:"staff"`
			Admins     []string `json:"admins"`
			GlobalMods []string `json:"global_mods"`
			Viewers    []string `json:"viewers"`
		} `json:"chatters"`
	}
	err = json.Unmarshal(body, &tmp)
	if err != nil {
		return nil, err
	}
	c := tmp.Chatters

	viewers := make(map[string]*viewer)
	for _, v := range c.Vips {
		key := strings.ToLower(v)

		viewers[key] = &viewer{
			Name:       v,
			IsMod:      false,
			IsFollower: true,
			InChat:     true,
		}
	}

	for _, v := range c.Moderators {
		key := strings.ToLower(v)

		viewers[key] = &viewer{
			Name:       v,
			IsMod:      true,
			IsFollower: true,
			InChat:     true,
		}
	}

	for _, v := range c.Staff {
		key := strings.ToLower(v)

		viewers[key] = &viewer{
			Name:       v,
			IsMod:      false,
			IsFollower: true,
			InChat:     true,
		}
	}

	for _, v := range c.Admins {
		key := strings.ToLower(v)

		viewers[key] = &viewer{
			Name:       v,
			IsMod:      true,
			IsFollower: true,
			InChat:     true,
		}
	}

	for _, v := range c.GlobalMods {
		key := strings.ToLower(v)

		viewers[key] = &viewer{
			Name:       v,
			IsMod:      true,
			IsFollower: true,
			InChat:     true,
		}
	}

	for _, v := range c.Viewers {
		key := strings.ToLower(v)

		viewers[key] = &viewer{
			Name:       v,
			IsMod:      false,
			IsFollower: true,
			InChat:     true,
		}
	}

	return viewers, nil
}

func audienceRequest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	viewers, err := getChatters(vars["channel"])
	if err != nil {
		js, err := json.Marshal(response{Status: 500})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
		return
	}

	result := make([]*viewer, 0, len(viewers))
	for _, v := range viewers {
		result = append(result, v)
	}
	js, err := json.Marshal(result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func unknownRequest(w http.ResponseWriter, r *http.Request) {
	js, err := json.Marshal(response{Status: 500})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func main() {
	var err error
	client, err = helix.NewClient(&helix.Options{
		ClientID:      os.Getenv("CLIENT_ID"),
		RateLimitFunc: rateLimitCallback,
	})

	if err != nil {
		panic(err)
	}

	r := mux.NewRouter()
	r.HandleFunc("/join/{channel}", joinRequest)

	r.NotFoundHandler = http.HandlerFunc(unknownRequest)
	r.HandleFunc("/channel/{channel}/audience", audienceRequest)

	fmt.Println("Listening on 3000")
	http.ListenAndServe(":3000", r)
}
