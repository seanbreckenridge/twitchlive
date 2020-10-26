# twitchlive

A CLI tool to list which [twitch](https://www.twitch.tv/) channels you follow are currently live.

### Setup

Twitch API for CLI applications WeirdChamp

- Twitch API
  - Go to the [twitch developer console](https://dev.twitch.tv/console/apps) and create a application; set the callback URL to `http://localhost`.
  - Click 'Manage' and save your `ClientID`.
  - Download `config.yaml.example` to `$HOME/.config/twitchlive/config`, and modify so that it has your twitch `username`/`client_id`
  - Go to `https://id.twitch.tv/oauth2/authorize?redirect_uri=http://localhost&response_type=token&scope=&client_id=<YOUR_CLIENT_ID>`, replacing `YOUR_CLIENT_ID` with yours. That should redirect to you to localhost; the URL contains a query parameter with the `access_token`. Copy that into the `token` field in your config file.
  - You can test this with the following: `curl --verbose -H "Client-Id <CLIENT ID>" -H "Authorization: Bearer <ACCESS TOKEN>" "https://api.twitch.tv/helix/games/top"`
- Go/Install `twitchlive`
  - Install [go](https://golang.org/) if you haven't already, make sure your `$GOPATH` and `$GOBIN` environment variables are set.
  - Run: `go get -u "gitlab.com/seanbreckenridge/twitchlive"`

## Run

Basic Text Output:

```
$ twitchlive # uses username in ~/.config/twitchlive/config
```

You can use the `-delimeter` flag to specify what to separate each field with.

Table Output:

```
twitchlive -username=<some_other_username> -output-format=table
+---------------+--------+--------------+-------------------------------------+
|     USER      | UPTIME | VIEWER COUNT |            STREAM TITLE             |
+---------------+--------+--------------+-------------------------------------+
| nl_Kripp      | 05:27  |        14683 | Chill BG Night | Twitter            |
|               |        |              | @Kripparrian                        |
| sodapoppin    | 06:42  |        14003 | serkfgjhlbnlsebfoldtghnodilurngudrg |
| LilyPichu     | 04:01  |         7676 | hhiiiii                             |
| Mizkif        | 08:47  |         6742 | YO GET IN HERE                      |
| Trainwreckstv | 00:49  |         3337 | recap + ban appeals | !twitter      |
|               |        |              | | !podcast                          |
| Greekgodx     | 07:34  |         2868 | @Greekgodx on Twitter               |
| SirhcEz       | 01:44  |         1430 | SINGEEDDDDDD | SirhcEz cafe &       |
|               |        |              | chill | #LeaguePartner              |
+---------------+--------+--------------+-------------------------------------+
```

JSON:

As an example use case, get average viewer count of channels I follow which are currently live:

```
$ twitchlive -output-format=json \
| jq -r '.[] | "\(.viewer_count)"'\
| awk '{total += $0} END { print int(total/NR) }'
5611
```

... or check if a particular channel is live, by `grep`ing against the output.

```
if twitchlive -output-format=json | jq -r '.[] | "\(.username)"' | grep -ixq xqcow; then
    echo "Pepega is live."
fi
```

### Usage

```
Usage for twitchlive:
  -delimiter string
    	string to separate entires when printing (default " @@@ ")
  -output-format string
    	possible values: 'basic', 'table', 'json' (default "basic")
  -timestamp
    	print unix timestamp instead of stream duration
  -timestamp-seconds
    	print seconds since epoch instead of unix timestamp
  -username string
    	specify user to get live channels for
```

Passing the `username` flag overrides the `username` set in `~/.config/twitchlive/config`

#### TODO:

* The access token seems to expire every 2 months. Its possible to add a server to receive this using a template/JS

