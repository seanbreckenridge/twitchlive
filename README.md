# twitchlive

A CLI tool to list which [twitch](https://www.twitch.tv/) channels you follow are currently live.

### Setup

Go to [the dev console](https://dev.twitch.tv/console/apps) and create a application, you can use `http://localhost` as the callback URL, it won't be used for this application.

Click 'Manage' and save your `ClientID`.

Install the binary:

`go get -u "github.com/seanbreckenridge/twitchlive"`

Download `config.yaml.example` to `$HOME/.config/twitchlive/config`

`curl --output "$HOME/.config/twitchlive/config" --create-dirs "https://raw.githubusercontent.com/seanbreckenridge/twitchlive/master/config.yaml.example"`

... and modify it so that it has your twitch `user_name`/`client_id`

### Run

`twitchlive`

### Dependencies:

[go](https://golang.org/), make sure your `$GOPATH` and `$GOBIN` environment variables are set.

