# twitchlive

A CLI tool to list which [twitch](https://www.twitch.tv/) channels you follow are currently live.

### Setup

- Setup the [`twitch-cli`](https://dev.twitch.tv/docs/api/) tool, that handles storing and refreshing the token to a file. That expires every 2 months, can be refreshed by running `twitch token`
- Go/Install `twitchlive`
  - Install [go](https://golang.org/) if you haven't already, make sure your `$GOPATH` and `$GOBIN` environment variables are set.
  - Run: `go install "github.com/seanbreckenridge/twitchlive@latest"`

## Run

Basic Text Output:

```
$ twitchlive
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
A CLI tool to list which twitch channels you follow are currently live.

Usage for twitchlive:
  -delimiter string
    	string to separate entires when printing (default " | ")
  -output-format string
    	possible values: 'basic', 'table', 'json' (default "basic")
  -timestamp
    	print unix timestamp instead of stream duration
  -timestamp-seconds
    	print seconds since epoch instead of unix timestamp
  -twitch-cli-env-path string
    	path to the twitch-cli config file (default "/home/sean/.config/twitch-cli/.twitch-cli.env")
  -username string
    	specify user to get live channels for
```
