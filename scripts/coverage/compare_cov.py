import requests
import os
import json

# Set up authentication credentials
token = os.environ.get("GITHUB_TOKEN")
headers = {"Authorization": f"Bearer {token}"}

pr_num = os.environ.get("PULL_REQUEST_NUMBER")
print("PR number is", pr_num)
# Set repository information
owner = "azure"
repo = "retina"

current_branch_file = "coverageexpanded.out"
main_branch_file = "maincoverageexpanded.out"

if not pr_num:
    print("No PR number found")
    exit(0)

if not token:
    print("No token found")
    exit(0)


def getAvgPerFile(given_dict):
    for key in given_dict:
        if key == "total":
            given_dict[key] = float(given_dict[key]["(statements)"])
            continue
        total = 0
        for subkey in given_dict[key]:
            total += float(given_dict[key][subkey])
        avg = total / len(given_dict[key])
        given_dict[key] = round(avg, 2)
    return given_dict


current_branch_lines = None
current_parsed_data = {}
# read the current branch coverage file
with open(current_branch_file, "r") as f:
    current_branch_lines = f.readlines()
    if current_branch_lines is None:
        print("No coverage data found for current branch")
        exit(1)
    for line in current_branch_lines:
        # Split the string into separate items
        items = line.split("\t")
        # Split each item into four parts and add them to a list
        funcname = ""
        for item in items:
            if item == "":
                continue

            if ":" in item:
                parts = item.split(":")
                path = parts[0]
                if path not in current_parsed_data:
                    current_parsed_data[path] = {}
                continue
            if "%" in item:
                parts = item.split("%")
                percentage = parts[0]
                current_parsed_data[path][funcname] = percentage
                continue
            funcname = item

current_parsed_data = getAvgPerFile(current_parsed_data)

main_branch_lines = None
main_parsed_data = {}
# read the main branch coverage file
with open(main_branch_file, "r") as f:
    main_branch_lines = f.readlines()
    if main_branch_lines is None:
        print("No coverage data found for main branch")
        exit(1)
    for line in main_branch_lines:
        # Split the string into separate items
        items = line.split("\t")
        # Split each item into four parts and add them to a list
        funcname = ""
        for item in items:
            if item == "":
                continue

            if ":" in item:
                parts = item.split(":")
                path = parts[0]
                if path not in main_parsed_data:
                    main_parsed_data[path] = {}
                continue
            if "%" in item:
                parts = item.split("%")
                percentage = parts[0]
                main_parsed_data[path][funcname] = percentage
                continue
            funcname = item

new_main_parsed_data = getAvgPerFile(main_parsed_data)

calculated_diff = {
    "increased": {},
    "decreased": {},
    "added": {},
    "removed": {},
    "nochange": {},
    "total": {}
}


''' This one is to compare with symbols
def compare_dicts(main_dict, cur_dict):
    """
    Recursively compare two dictionaries and print the differences.
    """
    for key in set(main_dict.keys()) | set(cur_dict.keys()):
        if key not in main_dict:
            for subkey in main_dict[key]:
                calculated_diff["added"][key] = subkey
        elif key not in cur_dict:
            for subkey in cur_dict[key]:
                calculated_diff["removed"][key] = subkey
        elif key == "total":
            if float(main_dict[key]['(statements)']) < float(cur_dict[key]['(statements)']):
                calculated_diff[key] = "increased"
            elif float(main_dict[key]['(statements)']) > float(cur_dict[key]['(statements)']):
                calculated_diff[key] = "decreased"
            else:
                calculated_diff[key] = "no change"
        else:
            for subkey in set(main_dict[key].keys()) | set(cur_dict[key].keys()):
                if subkey not in main_dict[key]:
                    calculated_diff["added"][key] = subkey
                elif subkey not in cur_dict[key]:
                    calculated_diff["removed"][key] = subkey
                else:
                    if float(main_dict[key][subkey]) < float(cur_dict[key][subkey]):
                        calculated_diff["increased"][key] = subkey
                    elif float(main_dict[key][subkey]) > float(cur_dict[key][subkey]):
                        calculated_diff["decreased"][key] = subkey
                    else:
                        calculated_diff["nochange"][key] = subkey


compare_dicts(main_parsed_data, current_parsed_data)
'''


def compare_dicts(main_dict, cur_dict):
    """
    Recursively compare two dictionaries and print the differences.
    """
    for key in set(main_dict.keys()) | set(cur_dict.keys()):
        if key not in main_dict:
            calculated_diff["added"][key] = f"`0%` ... `{cur_dict[key]}%`"
        elif key not in cur_dict:
            calculated_diff["removed"][key] = f"`{main_dict[key]}%` ... `0%`"
        elif key == "total":
            if float(main_dict[key]) < float(cur_dict[key]):
                calculated_diff[key] = "increased"
            elif float(main_dict[key]) > float(cur_dict[key]):
                calculated_diff[key] = "decreased"
            else:
                calculated_diff[key] = "no change"
        else:
            if float(main_dict[key]) < float(cur_dict[key]):
                calculated_diff["increased"][
                    key] = f"`{main_dict[key]}%` ... `{cur_dict[key]}%` (`{ round(cur_dict[key] - main_dict[key],2) }%`)"
            elif float(main_dict[key]) > float(cur_dict[key]):
                calculated_diff["decreased"][
                    key] = f"`{main_dict[key]}%` ... `{cur_dict[key]}%` (`{ round(cur_dict[key] - main_dict[key],2)}%`)"
            else:
                calculated_diff["nochange"][key] = f"`{main_dict[key]}%` ... `{cur_dict[key]}%`"


compare_dicts(main_parsed_data, current_parsed_data)

body_of_comment = "# Retina Code Coverage Report\n\n"

if calculated_diff["total"] == "increased":
    body_of_comment += "## Total coverage increased from `" + \
        str(main_parsed_data["total"]) + "%` to `" + \
        str(current_parsed_data["total"]) + \
        "%`  :white_check_mark:\n\n"

elif calculated_diff["total"] == "decreased":
    body_of_comment += "## Total coverage decreased from `" + \
        str(main_parsed_data["total"]) + "%` to `" + \
        str(current_parsed_data["total"]) + \
        "%`  :x:\n\n"
else:
    body_of_comment += "## Total coverage no change\n\n"


if len(calculated_diff["increased"]) > 0:
    body_of_comment += "Increased diff\n"
    body_of_comment += "| Impacted Files | Coverage | |\n"
    body_of_comment += "| --- | --- | --- |\n"
    for key in calculated_diff["increased"]:
        body_of_comment += "| " + key.replace("github.com/microsoft/retina/", "") + " | " + \
            calculated_diff["increased"][key] + " | :arrow_up: |\n"
    body_of_comment += "\n"

if len(calculated_diff["decreased"]) > 0:
    body_of_comment += "Decreased diff \n"
    body_of_comment += "| Impacted Files | Coverage | |\n"
    body_of_comment += "| --- | --- | --- |\n"
    for key in calculated_diff["decreased"]:
        body_of_comment += "| " + key.replace("github.com/microsoft/retina/", "") + " | " + \
            calculated_diff["decreased"][key] + \
            " | :arrow_down: |\n"
    body_of_comment += "\n"


print(body_of_comment)

# check if the PR is raised by depandabot, if yes ignore posting a comment on the PR
issue_url = f"https://api.github.com/repos/{owner}/{repo}/issues/{pr_num}"
# Make the API call to get the comments on the pull request
issue_response = requests.get(issue_url, headers=headers)
if issue_response.status_code != 200:
    print(
        f"Failed to get PR with url {issue_url}", issue_response.content)
    exit(1)

# Parse the JSON response to get the comments as a list of Python dictionaries
issues_data = json.loads(issue_response.content)

if issues_data["user"]["login"] == "dependabot[bot]":
    print("PR raised by dependabot, ignoring posting a comment on the PR")
    exit(0)


# Set the URL for the API call to get the comments on the pull request
comments_url = f"https://api.github.com/repos/{owner}/{repo}/issues/{pr_num}/comments"

# Make the API call to get the comments on the pull request
comments_response = requests.get(comments_url, headers=headers)
if comments_response.status_code != 200:
    print(
        f"Failed to get comments of the PR with url {comments_url}", comments_response.content)
    exit(1)

# Parse the JSON response to get the comments as a list of Python dictionaries
comments_data = json.loads(comments_response.content)

# Check if there is an existing comment that starts with "Coverage"
existing_comment = None
for comment in comments_data:
    if comment["body"].startswith("# Retina Code Coverage Report"):
        existing_comment = comment
        break

# If there is an existing comment, update it
if existing_comment:
    # Set the URL for the API call to update the comment
    update_url = f"https://api.github.com/repos/{owner}/{repo}/issues/comments/{existing_comment['id']}"

    # Set the payload for the API call to update the comment
    payload = {
        "body": body_of_comment
    }

    # Make the API call to update the comment
    update_response = requests.patch(update_url, headers=headers, json=payload)
    if update_response.status_code != 200:
        print(
            f"Failed to update the comment with url {update_url}", update_response.content)
        exit(1)

    # Print the response to confirm that the comment was updated successfully
    print(update_response.content)

# If there is no existing comment, add a new comment
else:
    # Set the payload for the API call to add the new comment
    payload = {
        "body": body_of_comment
    }

    # Make the API call to add the new comment
    add_response = requests.post(comments_url, headers=headers, json=payload)
    if add_response.status_code != 201:
        print(
            f"Failed to add the comment with url {comments_url}", add_response.content)
        exit(1)

    # Print the response to confirm that the comment was added successfully
    print(add_response.content)
