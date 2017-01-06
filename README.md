# go-oncall-agenda

Generate the on-call agenda for the week in Confluence Wiki format.

## Getting Started

### Download Go

Installation instructions [here](https://golang.org/doc/install).

### Clone this Repo

	cd $GOPATH
	mkdir -p github.dev.meetup.com/rich
	git clone git@github.dev.meetup.com:rich/go-oncall-agenda.git
	
## Configuration

You will need an access token from PagerDuty for this to work correctly. With this access token, create a file `~/pd.yml`.

Go to [https://meetup.pagerduty.com/api_keys](https://meetup.pagerduty.com/api_keys) and click `Create New API Key`. Select API version `v2 Current` and check `Read-only API Key`. 

`~/pd.yml`:

	---
	authtoken: <pagerduty-api-v2-authtoken>

## Running

### Executing the Script

	go install github.dev.meetup.com/rich/go-oncall-agenda
	cd $GOPATH/src/github.dev.meetup.com/rich/go-oncall-agenda
	$GOPATH/bin/go-oncall-agenda

This will generate the Confluence Wiki output to `stdout`. On a Mac, you can pipe this to the clipboard by using `pbcopy`:

	$GOPATH/bin/go-oncall-agenda | pbcopy
	
## Troubleshooting

### panic: open confluence_wiki.template: no such file or directory

The script needs to read `confluence_wiki.template` in the repo. If the file isn't in the current directory, you'll see an error.**