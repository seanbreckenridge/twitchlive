package main

import (
	"fmt"
	"github.com/spf13/viper"
	"github.com/tidwall/gjson"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const BASEURL = "https://api.twitch.tv/helix/"

// Configuration passed from user using flags and config file
type config struct {
	port      int
	clientId  string
	user_name string
}

type liveChannel struct {
	user_name    string
	title        string
	viewer_count int
	started_at   time.Time
}

// read the configuration from command line flags
// and the configuration file
func GetConfig() *config {
	// read flags
	// TODO: add json output, and a nicer table output as flags
	// read configuration file
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$XDG_CONFIG_HOME/twitch-live")
	viper.AddConfigPath("$HOME/.config/twitch-live")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %s\n", err)
	}
	return &config{
		clientId:  viper.GetString("client_id"),
		user_name: viper.GetString("user_name"),
	}
}

// makes an HTTP request and returns the response and body, as long as its valid
func makeRequest(request *http.Request) (*http.Response, string) {
	// create client and make request
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		panic(err)
	}
	// check response
	defer response.Body.Close()
	respBytes, _ := ioutil.ReadAll(response.Body)
	respBody := string(respBytes)
	// dump information to screen and exit if it failed
	if response.StatusCode >= 400 {
		log.Printf("Requesting %s failed with status code %s", request.URL, response.StatusCode)
		log.Printf("%s\n", respBody)
		os.Exit(1)
	}
	return response, respBody
}

// get the twitch user id for a twitch user_name
func getUserId(conf *config) string {
	req, _ := http.NewRequest("GET", BASEURL+"users", nil)
	// set client header
	req.Header.Set("Client-Id", conf.clientId)
	// create query string
	q := req.URL.Query()
	q.Add("login", conf.user_name)
	req.URL.RawQuery = q.Encode()

	_, respBody := makeRequest(req)

	// get userIdStr from JSON response
	return gjson.Get(respBody, "data.0.id").String()
}

// get which channels this user is following
// puts response into followedUsers
func getFollowingChannels(conf *config, userId string, paginationCursor *string, followedUsers []string) []string {
	// create request
	req, _ := http.NewRequest("GET", BASEURL+"users/follows", nil)
	req.Header.Set("Client-Id", conf.clientId)

	// create query
	q := req.URL.Query()
	q.Add("from_id", userId)
	q.Add("first", "100")
	// if this has been called recursively, set the pagination cursor
	// to get the next page of results
	if paginationCursor != nil {
		q.Add("after", *paginationCursor)
	}
	req.URL.RawQuery = q.Encode()

	// make request and get response body
	_, respBody := makeRequest(req)

	// get number of channels this user follows
	followCount := int(gjson.Get(respBody, "total").Float())
	// add all the channel ids to the slice
	for _, id := range gjson.Get(respBody, "data.#.to_id").Array() {
		followedUsers = append(followedUsers, id.String())
	}

	// if we havent got all of the items yet, do a recursive call
	if len(followedUsers) < followCount {
		cursor := gjson.Get(respBody, "pagination.cursor").String()
		followedUsers = getFollowingChannels(conf, userId, &cursor, followedUsers)
	}

	return followedUsers
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// create the giant URL to request currently live users for getLiveUsers
func createLiveUsersURL(conf *config, followedUsers []string, startAt int, endAt int) (*http.Request, int) {

	// create the URL
	req, _ := http.NewRequest("GET", BASEURL+"streams", nil)
	req.Header.Set("Client-Id", conf.clientId)
	q := req.URL.Query()
	// specify how many values to return (all of them, if 100 streamers happened to be live)
	// if you sent 100 users and only 10 of them were live, it would only return the value
	// for those 10 streamers
	q.Add("first", "100")

	// determine whether we stop at the end of the list
	// or if the next chunk of 100 ids is still before the end of the list
	stopAtMin := min(len(followedUsers), endAt)
	// add each user to the query param, like user_id=1&user_id=2
	for i := startAt; i < stopAtMin; i++ {
		q.Add("user_id", followedUsers[i])
	}
	req.URL.RawQuery = q.Encode()

	return req, stopAtMin
}

// get currently live users from followedUsers.
// Since you can only specify 100 IDs,
// and you also return 100 IDs at a time using the 'first' param,
// pagination isnt needed on this endpoint.
func getLiveUsers(conf *config, followedUsers []string) []liveChannel {

	// instantiate return array
	liveChannels := make([]liveChannel, 0)
	curAt := 0 // where the current index in the followedUsers list is
	var req *http.Request
	for loopCond := curAt < len(followedUsers); loopCond; loopCond = curAt < len(followedUsers) {
		req, curAt = createLiveUsersURL(conf, followedUsers, curAt, curAt+100)
		// make the request for this chunk of IDs
		_, requestBody := makeRequest(req)
		liveChannelData := gjson.Parse(requestBody).Get("data").Array()
		// grab information from each of items in the array
		for _, lc := range liveChannelData {
			lc_time, _ := time.Parse(time.RFC3339, lc.Get("started_at").String())
			liveChannels = append(liveChannels, liveChannel{
				user_name:    lc.Get("user_name").String(),
				title:        lc.Get("title").String(),
				viewer_count: int(lc.Get("viewer_count").Float()),
				started_at:   lc_time,
			})
		}
	}

	return liveChannels
}

func main() {
	conf := GetConfig()
	// fmt.Printf("%+v\n", *conf)
	userId := getUserId(conf)
	followedUsers := getFollowingChannels(conf, userId, nil, make([]string, 0))
	getLiveUsers := getLiveUsers(conf, followedUsers)
	delimiter := " @@@ "
	for _, live_user := range getLiveUsers {
		fmt.Println(strings.Join([]string{
			live_user.user_name,
			live_user.title,
			strconv.Itoa(live_user.viewer_count),
			live_user.started_at.Format(time.UnixDate)}, delimiter))
	}
}
