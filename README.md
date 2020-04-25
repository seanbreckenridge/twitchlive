# twitch-live

A CLI tool to list which channels you follow are currently live.

### Setup

Go to [the dev console](https://dev.twitch.tv/console/apps) and create a application, you can use `http://localhost` as the callback URL, it won't be used for this application.

Click 'Manage' and save the ClientID.

See [here](https://dev.twitch.tv/docs/api) for a more extensive tutorial.

Install the binary:

`go get -v -u "github.com/seanbreckenridge/twitch-live"`

Modify the config.example and place that at `$HOME/.config/twitch-live/config`

`curl --output "$HOME/.config/twitch-live/config" --create-dirs "https://raw.githubusercontent.com/seanbreckenridge/twitch-live/master/config.yaml.example"`

The `user_name` is your twitch username, or the user for who you want to get currently live channels for.

### Run

`twitch-live`

### Dependencies:

[go](https://golang.org/), make sure your `$GOPATH` and `$GOBIN` environment variables are set.

