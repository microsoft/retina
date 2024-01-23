import requests
import zipfile
import os

# Set up authentication credentials
token = os.environ.get("GITHUB_TOKEN")
headers = {"Authorization": f"Bearer {token}"}

pr_num = os.environ.get("PULL_REQUEST_NUMBER")
print("PR number is", pr_num)

if not pr_num:
    print("No PR number found")
    exit(0)

if not token:
    print("No token found")
    exit(0)

# Set repository information
owner = "azure"
repo = "retina"

ut_workflow_yaml = "retina-test.yaml"

# Get the id of UT workflow
wf_url = f"https://api.github.com/repos/{owner}/{repo}/actions/workflows/{ut_workflow_yaml}"
response = requests.get(wf_url, headers=headers)
response.raise_for_status()
wf_id = response.json()["id"]

# Get the latest completed workflow run on the main branch
runs_url = f"https://api.github.com/repos/{owner}/{repo}/actions/workflows/{wf_id}/runs"
params = {"branch": "main", "status": "completed", "per_page": 10}
response = requests.get(runs_url, headers=headers, params=params)
response.raise_for_status()
artifacts_url = response.json()["workflow_runs"][0]["artifacts_url"]

# Create the main branch folder for coverage
folder_name = "mainbranchcoverage"
file_name = "coverage.out"
if not os.path.exists(folder_name):
    os.makedirs(folder_name)

for wf in response.json()["workflow_runs"]:
    artifacts_url = wf["artifacts_url"]
    # Get any artifacts named "coverage" for the specified workflow run
    # artifacts_url = f"https://api.github.com/repos/{owner}/{repo}/actions/runs/{run_id}/artifacts"
    response = requests.get(artifacts_url, headers=headers)
    response.raise_for_status()
    artifacts = response.json()["artifacts"]
    found_files = False

    # Process the list of artifacts as needed
    for artifact in artifacts:
        if "coverage" in artifact["name"]:
            print("Downloading artifacts from main branch",
                  artifact["name"], artifact["archive_download_url"])
            response = requests.get(
                artifact["archive_download_url"], headers=headers)
            response.raise_for_status()
            with open("mainbranchcov.zip", "wb") as f:
                f.write(response.content)
            # Unzip the coverage file

            with zipfile.ZipFile("mainbranchcov.zip", 'r') as zip_ref:
                zip_ref.extractall(folder_name)

            if os.path.isfile(f'./{folder_name}/{file_name}'):
                found_files = True
                break

    if found_files:
        break
