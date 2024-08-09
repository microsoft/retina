# Retina AI

## Usage

- Change into this *ai/* folder.
- `go mod tidy ; go mod vendor`
- Modify the `defaultConfig` values in *main.go*
- If using Azure OpenAI:
  - Make sure you're logged into your account/subscription in your terminal.
  - Specify environment variables for Deployment name and Endpoint URL. Get deployment from e.g. [https://oai.azure.com/portal/deployment](https://oai.azure.com/portal/deployment) and Endpoint from e.g. Deployment > Playground > Code.
    - Linux:
      - `read -p "Enter AOAI_COMPLETIONS_ENDPOINT: " AOAI_COMPLETIONS_ENDPOINT && export AOAI_COMPLETIONS_ENDPOINT=$AOAI_COMPLETIONS_ENDPOINT`
      - `read -p "Enter AOAI_DEPLOYMENT_NAME: " AOAI_DEPLOYMENT_NAME && export AOAI_DEPLOYMENT_NAME=$AOAI_DEPLOYMENT_NAME`
    - Windows:
      - `$env:AOAI_COMPLETIONS_ENDPOINT = Read-Host 'Enter AOAI_COMPLETIONS_ENDPOINT'`
      - `$env:AOAI_DEPLOYMENT_NAME = Read-Host 'Enter AOAI_DEPLOYMENT_NAME'`
- `go run main.go`
