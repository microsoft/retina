# Overview

We use GITHUB_TOKEN to get the following:

1. Workflow ID with the retina test yaml
2. Get the latest main branch successful workflow run with that id
3. Get the artifacts from that run and download them

## Testing Locally

1. Set your PAT from github developer settings as `GITHUB_TOKEN` env variable
https: //docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token#creating-a-personal-access-token-classic
2. Set a test PR number as `PULL_REQUEST_NUMBER` env variable
3. generate coverage.out for local branch with `go test -coverprofile=coverage.out ./...`
4. run `make coverage` on your local branch and not main.
