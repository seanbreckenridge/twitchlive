package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
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
	User_name      string `json:"username"`
	Title          string `json:"title"`
	Viewer_count   int    `json:"viewer_count"`
	started_at     time.Time
	Formatted_time string `json:"time"`
}

// Configuration passed from user using flags and config file
// and additional metadata (user id) pass around with requests
type config struct {
	client_id         string
	bearer_token      string
	user_name         string
	user_id           string
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
	return OutputFormatBasic, fmt.Errorf("Could not find '%s' in allowed output formats. Run %s -h for a full list.", *format, os.Args[0])
}

// read the configuration from command line flags
// and the configuration file
func getConfig() *config {

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
		client_id:         viper.GetString("client_id"),
		bearer_token:      viper.GetString("token"),
		user_name:         *username,
		delimiter:         *delimiter,
		output_format:     output_format,
		timestamp:         *timestamp,
		timestamp_seconds: *timestamp_seconds,
	}
}

// twitch API can return banned users, make sure there are no dupliates
// https://www.reddit.com/r/golang/comments/5ia523/idiomatic_way_to_remove_duplicates_in_a_slice/db6qa2e/
func SliceUniqMap(s []string) []string {
	seen := make(map[string]struct{}, len(s))
	j := 0
	for _, v := range s {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		s[j] = v
		j++
	}
	return s[:j]
}

// makes an HTTP request and returns the response and body, as long as its valid
func makeRequest(request *http.Request, client *http.Client) (*http.Response, string) {
	// make request
	// fmt.Println(request.URL.String())

	response, err := client.Do(request)
	if err != nil {
		log.Fatalf("Error making HTTP request: %s\n", err)
	}
	defer response.Body.Close()

	// read response
	scanner := bufio.NewScanner(response.Body)
	scanner.Split(bufio.ScanRunes)
	var buf bytes.Buffer
	for scanner.Scan() {
		buf.WriteString(scanner.Text())
	}
	respBody := buf.String()
	// println(respBody)

	// dump information to screen and exit if it failed
	if response.StatusCode >= 400 {
		log.Printf("Requesting %s failed with status code %d\n", request.URL.String(), response.StatusCode)
		log.Println(respBody)
		os.Exit(1)
	}
	return response, respBody
}

// get the twitch user id for a twitch user_name
func getUserId(conf *config, client *http.Client) string {
	req, _ := http.NewRequest("GET", BASEURL+"users", nil)
	// set client header
	req.Header.Set("Client-Id", conf.client_id)
	req.Header.Set("Authorization", "Bearer "+conf.bearer_token)
	// create query string
	q := req.URL.Query()
	q.Add("login", conf.user_name)
	req.URL.RawQuery = q.Encode()

	_, respBody := makeRequest(req, client)

	// get userIdStr from JSON response
	return gjson.Get(respBody, "data.0.id").String()
}

// get which channels this user is following
// puts response into followedUsers
func getFollowingChannels(conf *config, client *http.Client, paginationCursor *string, followedUsers []string) []string {
	// create request
	req, _ := http.NewRequest("GET", BASEURL+"users/follows", nil)
	req.Header.Set("Client-Id", conf.client_id)
	req.Header.Set("Authorization", "Bearer "+conf.bearer_token)

	// create query
	q := req.URL.Query()
	q.Add("from_id", conf.user_id)
	q.Add("first", "100")
	// if this has been called recursively, set the pagination cursor
	// to get the next page of results
	if paginationCursor != nil {
		q.Add("after", *paginationCursor)
	}
	req.URL.RawQuery = q.Encode()

	// make request and get response body
	_, respBody := makeRequest(req, client)

	// get number of channels this user follows
	followCount := int(gjson.Get(respBody, "total").Float())
	// add all the channel ids to the slice
	for _, id := range gjson.Get(respBody, "data.#.to_id").Array() {
		followedUsers = append(followedUsers, id.String())
	}

	// if we havent got all of the items yet, do a recursive call
	if len(followedUsers) < followCount {
		cursor := gjson.Get(respBody, "pagination.cursor").String()
		followedUsers = getFollowingChannels(conf, client, &cursor, followedUsers)
	}

	return followedUsers
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// truncates strings more than 30 characters
// This is used to truncate titles,
// so it doesnt break table formatting
func truncate(title string) string {
	var buffer strings.Builder
	parts := strings.Split(title, " ")
	for _, token := range parts {
		if len(token) > 30 {
			buffer.WriteString(token[0:28])
			buffer.WriteString("--")
		} else {
			buffer.WriteString(token)
		}
		buffer.WriteString(" ")
	}
	return strings.TrimSpace(buffer.String())
}

// create the giant URL to request currently live users for getLiveUsers
func createLiveUsersURL(conf *config, followedUsers []string, startAt int, endAt int) (*http.Request, int) {

	// create the URL
	req, _ := http.NewRequest("GET", BASEURL+"streams", nil)
	req.Header.Set("Client-Id", conf.client_id)
	req.Header.Set("Authorization", "Bearer "+conf.bearer_token)
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
func getLiveUsers(conf *config, client *http.Client, followedUsers []string) []liveChannelInfo {

	// instantiate return array
	liveChannels := make([]liveChannelInfo, 0)
	curAt := 0 // where the current index in the followedUsers list is
	var req *http.Request
	for loopCond := curAt < len(followedUsers); loopCond; loopCond = curAt < len(followedUsers) {
		req, curAt = createLiveUsersURL(conf, followedUsers, curAt, curAt+100)
		// make the request for this chunk of IDs
		_, requestBody := makeRequest(req, client)
		liveChannelData := gjson.Parse(requestBody).Get("data").Array()
		// grab information from each of items in the array
		for _, lc := range liveChannelData {
			lc_time, _ := time.Parse(time.RFC3339, lc.Get("started_at").String())
			liveChannels = append(liveChannels, liveChannelInfo{
				User_name:    lc.Get("user_name").String(),
				Title:        lc.Get("title").String(),
				Viewer_count: int(lc.Get("viewer_count").Float()),
				started_at:   lc_time,
			})
		}
	}

	return liveChannels
}

func main() {

	conf := getConfig()

	// make requests to twitch API
	client := &http.Client{}
	conf.user_id = getUserId(conf, client)
	followedUsers := getFollowingChannels(conf, client, nil, make([]string, 0))
	liveUsers := getLiveUsers(conf, client, SliceUniqMap(followedUsers))

	// format output according to flags
	now := time.Now()
	for index, live_user := range liveUsers {
		if conf.timestamp_seconds {
			liveUsers[index].Formatted_time = strconv.Itoa(int(live_user.started_at.Unix()))
		} else if conf.timestamp {
			liveUsers[index].Formatted_time = live_user.started_at.Format(time.UnixDate)
		} else {
			// default, display how long they've been in live
			timeDiff := now.Sub(live_user.started_at)
			// format into HH:MM
			hours := timeDiff / time.Hour
			timeDiff -= hours * time.Hour
			minutes := timeDiff / time.Minute
			liveUsers[index].Formatted_time = fmt.Sprintf("%02d:%02d", hours, minutes)
		}
	}

	// print, according to output format
	switch conf.output_format {
	case OutputFormatBasic:
		for _, live_user := range liveUsers {
			fmt.Println(strings.Join([]string{
				live_user.User_name,
				live_user.Formatted_time,
				strconv.Itoa(live_user.Viewer_count),
				live_user.Title},
				(*conf).delimiter))
		}
	case OutputFormatJson:
		jsonBytes, err := json.Marshal(liveUsers)
		if err != nil {
			log.Fatalf("Error encoding to JSON: %s\n", err)
		}
		fmt.Printf(string(jsonBytes))
	case OutputFormatTable:
		tableData := make([][]string, len(liveUsers))
		for index, live_user := range liveUsers {
			tableData[index] = []string{
				live_user.User_name,
				live_user.Formatted_time,
				strconv.Itoa(live_user.Viewer_count),
				truncate(live_user.Title),
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
