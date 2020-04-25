package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/viper"
	"github.com/tidwall/gjson"
)

const BASEURL = "https://api.twitch.tv/helix/"
const DESCRIPTION = "A CLI tool to list which twitch channels you follow are currently live."

type OutputFormat string

const (
	OutputFormatBasic OutputFormat = "basic"
	OutputFormatTable              = "table"
	OutputFormatJson               = "json"
)

type liveChannelInfo struct {
	user_name      string
	title          string
	viewer_count   int
	started_at     time.Time
	formatted_time string
}

type jsonContainer struct {
	Channels []liveChannelJson `json:"channels"`
}

// a class to allow json encoding without the started_at field
type liveChannelJson struct {
	User_name    string `json:"user_name"`
	Title        string `json:"title"`
	Viewer_count int    `json:"viewer_count"`
	Time         string `json:"time"`
}

// Configuration passed from user using flags and config file
type config struct {
	clientId          string
	user_name         string
	delimiter         string
	output_format     OutputFormat
	timestamp         bool
	timestamp_seconds bool
}

// validates if the OutputFormat string is one of the allowed values
func parseOutputFormat(format *string) (OutputFormat, error) {
	passedFormat := OutputFormat(*format)
	switch passedFormat {
	case
		OutputFormatBasic,
		OutputFormatTable,
		OutputFormatJson:
		return passedFormat, nil
	}
	return OutputFormatBasic, fmt.Errorf("Could not find '%s' in allowed output formats. Run %s -h for a full list.", (*format), os.Args[0])
}

// read the configuration from command line flags
// and the configuration file
func GetConfig() *config {

	// customize flag usage prefix message to include a description message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n\nUsage for %s:\n", DESCRIPTION, os.Args[0])
		flag.PrintDefaults()
	}
	// define command line flags
	delimiter := flag.String("delimiter", " @@@ ", "string to separate entires when printing")
	username := flag.String("username", "", "specify user to get live channels for")
	output_format_str := flag.String("output-format", "basic", "possible values: 'basic', 'table', 'json'")
	timestamp := flag.Bool("timestamp", false, "print unix timestamp instead of stream duration")
	timestamp_seconds := flag.Bool("timestamp-seconds", false, "print seconds since epoch instead of unix timestamp")

	// parse command line flags
	flag.Parse()

	// validate output format
	output_format, err := parseOutputFormat(output_format_str)
	if err != nil {
		log.Fatalf("%s\n", err)
	}

	// TODO: add json output, and a nicer table output as flags
	// read configuration file
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$XDG_CONFIG_HOME/twitchlive")
	viper.AddConfigPath("$HOME/.config/twitchlive")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %s\n", err)
	}
	// default to username from config file if one wasnt set
	if *username == "" {
		(*username) = viper.GetString("username")
	}
	return &config{
		clientId:          viper.GetString("client_id"),
		user_name:         *username,
		delimiter:         *delimiter,
		output_format:     output_format,
		timestamp:         *timestamp,
		timestamp_seconds: *timestamp_seconds,
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
func getLiveUsers(conf *config, followedUsers []string) []liveChannelInfo {

	// instantiate return array
	liveChannels := make([]liveChannelInfo, 0)
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
			liveChannels = append(liveChannels, liveChannelInfo{
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

	// parse configuration
	conf := GetConfig()

	// make requests to twitch API
	userId := getUserId(conf)
	followedUsers := getFollowingChannels(conf, userId, nil, make([]string, 0))
	liveUsers := getLiveUsers(conf, followedUsers)

	// format output according to flags
	for index, live_user := range liveUsers {
		if conf.timestamp_seconds {
			liveUsers[index].formatted_time = strconv.Itoa(int(live_user.started_at.Unix()))
		} else if conf.timestamp {
			liveUsers[index].formatted_time = live_user.started_at.Format(time.UnixDate)
		} else {
			// default, display how long they've been in live
			timeDiff := time.Now().Sub(live_user.started_at)
			// format into HH:MM
			hours := timeDiff / time.Hour
			timeDiff -= hours * time.Hour
			minutes := timeDiff / time.Minute
			liveUsers[index].formatted_time = fmt.Sprintf("%02d:%02d", hours, minutes)
		}
	}

	switch conf.output_format {
	case OutputFormatBasic:
		for _, live_user := range liveUsers {
			fmt.Println(strings.Join([]string{
				live_user.user_name,
				live_user.formatted_time,
				strconv.Itoa(live_user.viewer_count),
				live_user.title},
				(*conf).delimiter))
		}
	case OutputFormatJson:
		liveUsersJson := make([]liveChannelJson, len(liveUsers))
		for index, live_user := range liveUsers {
			liveUsersJson[index] = liveChannelJson{
				User_name:    live_user.user_name,
				Title:        live_user.title,
				Viewer_count: live_user.viewer_count,
				Time:         live_user.formatted_time,
			}
		}
		jsonBytes, err := json.Marshal(&jsonContainer{Channels: liveUsersJson})
		if err != nil {
			log.Fatalf("Error encoding to JSON: %s\n", err)
		}
		fmt.Printf(string(jsonBytes))
	case OutputFormatTable:
		tableData := make([][]string, len(liveUsers))
		for index, live_user := range liveUsers {
			tableData[index] = []string{
				live_user.user_name,
				live_user.formatted_time,
				strconv.Itoa(live_user.viewer_count),
				live_user.title,
			}
		}
		table := tablewriter.NewWriter(os.Stdout)
		header := []string{"User", "Uptime", "Viewer Count", "Stream Title"}
		if conf.timestamp_seconds || conf.timestamp {
			header[1] = "Live Since"
		}
		table.SetHeader(header)
		table.AppendBulk(tableData)
		table.Render()

	}
}
