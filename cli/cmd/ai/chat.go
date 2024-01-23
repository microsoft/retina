package ai

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/microsoft/retina/cli/cmd/capture"
	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"k8s.io/apimachinery/pkg/util/wait"
	utilexec "k8s.io/utils/exec"
)

const (
	debug                  = true
	skipCapture            = false
	captureIsActuallyTrace = true

	// e.g. 5 --> "#####"

	Retina = "RETINA🔍"
	User   = "USER🕵️"
)

var (
	allowedToPipeToCommands = []string{
		"wc",
		"grep",
		"sort",
		"uniq",
		"head",
		"tail",
		"awk",
		"sed",
		"tr",
		"jq",
	}

	// don't allow these characters in the command
	badChars = []string{
		"&",
		"||",
		";",
		">",
		"<",
	}
)

func chatDivider() string {
	width, _, err := term.GetSize(0)
	if err != nil {
		fmt.Println(err)
	}
	return strings.Repeat("#", width)
}

func responseDivider() string {
	width, _, err := term.GetSize(0)
	if err != nil {
		fmt.Println(err)
	}
	return strings.Repeat("-", width)
}

type ChatAgent struct {
	AutoRun     bool
	Captured    bool
	Traced      bool
	CaptureFile string
	TraceFile   string
	Exec        utilexec.Interface
	*Chat
}

// func chatDivider() string {

// 	width, _, err := term.GetSize(0)

// 	if err != nil {

// 		fmt.Println(err)

// 	}

// 	return strings.Repeat("#", width)

// }

// func responseDivider() string {

// 	width, _, err := term.GetSize(0)

// 	if err != nil {

// 		fmt.Println(err)

// 	}

// 	return strings.Repeat("-", width)

// }

func NewChatAgent(gpt *GPT) *ChatAgent {
	return &ChatAgent{
		Exec: utilexec.New(),
		Chat: &Chat{
			GPT: gpt,
		},
	}
}

func (c *ChatAgent) Loop(cmd *cobra.Command) error {
	fmt.Println(chatDivider())
	fmt.Println()
	fmt.Printf("%s: Hello!👋 I'm an AI tool that can help you explore your AKS Cluster and answer your questions.\n\n", Retina)
	// fmt.Println("NOTE: This agent will be able to run \"kubectl get\" commands as well as all Retina commands. It will also be able to pipe \"kubectl get\" commands into the following commands:")
	// for _, command := range allowedToPipeToCommands {
	// 	fmt.Println(command)
	// }
	fmt.Println()
	fmt.Println("There will be a prompt before running each command.")
	fmt.Print("Would you like to auto-run commands instead? ")
	val, err := c.ReadExpectedValues("y", "n")
	if err != nil {
		return err
	}

	c.AutoRun = val == "y"
	fmt.Println()
	fmt.Println("Okay, let's get started!")
	fmt.Println(chatDivider())
	fmt.Println()

	c.PrintAndStoreMessage(Retina, "What would you like to know?")
	fmt.Println(responseDivider())

	// chat loop
	var userInput string
	for {
		// prompt user input
		userInput, err = c.ReadAndStoreUserInput()
		if err != nil {
			return err
		}
		fmt.Println(responseDivider())

		// triage the ask
		prompt := c.chatTriagePrompt(userInput)

		b := wait.Backoff{
			Steps:    5,
			Duration: 1 * time.Second,
			Factor:   2,
			Jitter:   0,
		}

		var resp string
		err = wait.ExponentialBackoff(b, func() (bool, error) {
			resp, err = c.GPT.Ask(prompt)
			if err != nil {
				fmt.Println("Retrying ChatGPT prompt...")
				return false, err
			}
			if resp == "" {
				fmt.Println("Retrying ChatGPT prompt...")
				return false, nil
			}
			return true, nil
		})
		// resp, err := c.GPT.Ask(prompt)
		// if err != nil {
		// 	return err
		// }

		if debug {
			fmt.Println("[DEBUG] [TRIAGE]", resp)
		}

		// execute and respond to triaged task
		if strings.Contains(resp, "UNKNOWN-TASK") {
			c.PrintAndStoreMessage(Retina, "Cannot perform this action.")
			fmt.Println(responseDivider())
			continue
		}

		// 1. Kubectl get command
		if strings.Contains(resp, "KUBECTL-BASH-COMMAND") {
			prompt := c.kubectlBashPrompt(userInput)
			resp, err := c.GPT.Ask(prompt)
			if err != nil {
				return err
			}

			// execute kubectl command
			resp = strings.TrimSpace(resp)
			c.PrintAndStoreMessage(Retina, fmt.Sprintf("Will execute below command:\n%s", resp))
			splitResp := strings.Split(resp, " ")
			isBadChar := false
			for _, s := range badChars {
				if strings.Contains(resp, s) {
					isBadChar = true
					fmt.Println("bad char:", s)
					break
				}
			}

			if isBadChar || len(splitResp) < 2 || splitResp[0] != "kubectl" {
				return fmt.Errorf("invalid kubectl/bash command: %s", resp)
			}

			jqIndex := strings.Index(resp, "| jq ")
			jqIndexEnd := jqIndex + 1
			jqExists := jqIndex != -1
			jqCount := 0
			for i := 5; i < len(resp); i++ {
				if resp[i-5:i] == "| jq " {
					jqCount++
				}
			}
			if jqCount > 1 {
				return fmt.Errorf("only one jq command is allowed")
			}
			if jqExists {
				// jq strings are delineated by apostrophes
				j := jqIndex + 5
				if resp[j] != '\'' {
					return fmt.Errorf("unsure how to parse jq string")
				}
				success := false
				j++
				for ; j < len(resp); j++ {
					if resp[j] == '\'' {
						jqIndexEnd = j
						success = true
						break
					}
				}
				if !success {
					return fmt.Errorf("reached end of jq string. unsure how to parse")
				}
			}

			pipeIndices := []int{}
			for i := 0; i < len(resp); i++ {
				if resp[i] == '|' {
					pipeIndices = append(pipeIndices, i)
				}
			}
			commandsWithArgs := strings.Split(resp, "|")
			for i, commandWithArgs := range commandsWithArgs {
				if i == 0 {
					// kubectl get ... |
					continue
				}

				if jqExists && jqIndex <= pipeIndices[i-1] && pipeIndices[i-1] <= jqIndexEnd {
					// don't check jq command
					continue
				}

				command := strings.Split(strings.TrimSpace(commandWithArgs), " ")[0]

				allowed := false
				for _, allowedCommand := range allowedToPipeToCommands {
					if command == allowedCommand {
						allowed = true
						break
					}
				}
				if !allowed {
					return fmt.Errorf("command from GPT is not allowed: %s", command)
				}
			}

			if !c.AutoRun {
				fmt.Print("Execute? ")
				val, err = c.ReadExpectedValues("y", "n")
				if err != nil {
					return err
				}

				if val == "n" {
					c.PrintAndStoreMessage(Retina, "Aborting.")
					fmt.Println(responseDivider())
					continue
				}
			}

			bashArgs := []string{"-c", resp}
			fmt.Println(chatDivider())
			fmt.Println()
			cmd := c.Exec.Command("bash", bashArgs...)
			// cmd.SetEnv([]string{"RETINA_AGENT_IMAGE=acnpublic.azurecr.io/retina-agent:linux-amd64-v0.0.11-33-g34dfb20"})
			output, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Println("failed:", err.Error())
				return fmt.Errorf("error executing command: %w", err)
			}
			c.PrintAndStoreMessage("OUTPUT", string(output))
			// grep returns exit code 1 when it doesn't find anything
			// if err != nil {
			// 	return fmt.Errorf("error executing command: %w", err)
			// }
			fmt.Println(chatDivider())
			fmt.Println()
			continue
		}

		// 2. packet capture or trace
		if strings.Contains(resp, "PACKET-CAPTURE-OR-TRACE") {
			c.PrintAndStoreMessage(Retina, "Let's gather some network data with a packet capture or packet trace.")
			fmt.Println()
			fmt.Println("A packet capture is a file that contains network traffic. From it we can analyze e.g.:")
			fmt.Println("- number of connections per Pod.")
			fmt.Println("- state of TCP/UDP connections.")
			fmt.Println("- which network entities are communicating.")
			fmt.Println("- long-running connections.")
			fmt.Println()
			fmt.Println("A packet trace installs an eBPF program to collect Flows. From it we can analyze e.g.:")
			fmt.Println("- packet drops in iptables (firewall).")
			fmt.Println("- etc.")
			fmt.Println()
			// fmt.Print("Would you like to perform a capture or trace? ")
			// val, err = c.ReadExpectedValues("c", "t", "neither")
			// if err != nil {
			// 	return err
			// }

			// if val == "neither" {
			// 	c.PrintAndStoreMessage(Retina, "Aborting.")
			// 	fmt.Println(responseDivider())
			// 	continue
			// }

			fmt.Println(responseDivider())
			c.PrintAndStoreMessage(Retina, "Describe which nodes to examine, or which pods and namespaces to examine.")
			fmt.Println(responseDivider())

			// prompt user input
			userInput, err = c.ReadAndStoreUserInput()
			if err != nil {
				return err
			}
			fmt.Println(responseDivider())

			// if val == "c" {
			// capture
			c.Traced = false

			for {
				c.PrintAndStoreMessage(Retina, "Will perform packet capture...")

				prompt, err := capturePrompt(cmd, strings.Split(userInput, " "))
				if err != nil {
					return err
				}

				if c.Memory[len(c.Memory)-3].Text == "Aborting Capture." && strings.Contains(c.Memory[len(c.Memory)-4].Text, "Will execute below command:\n") {
					sb := strings.Builder{}
					sb.WriteString("This is the previous command you suggested: \"")
					sb.WriteString(c.Memory[len(c.Memory)-4].Text[len("Will execute below command:\n"):])
					sb.WriteString("\". Modify this previous command based on the new TASK below.\n")
					for i, msg := range c.Memory {
						if i == len(c.Memory)-1 {
							// skip last user message so we can display it later
							break
						}
						sb.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Text))
					}
					sb.WriteString("\n")
					sb.WriteString("Given the above chat history as context, consider this new ask from USER: ")
					sb.WriteString(userInput)
					sb.WriteString("\n")

					prompt = sb.String() + prompt
				}

				resp, err := c.GPT.Ask(prompt)
				if err != nil {
					return err
				}

				// execute capture command
				resp = strings.TrimSpace(resp)
				if !strings.Contains(resp, "--no-wait") {
					resp += " --no-wait=false"
				}

				//if !strings.Contains(resp, "--blob-upload") {
				//		resp += " --blob-upload=\"<BLOB_URL>\""
				//}

				c.PrintAndStoreMessage(Retina, fmt.Sprintf("Will execute below command:\n%s", resp))

				splitResp := strings.Split(resp, " ")
				if len(splitResp) < 4 || splitResp[0] != "kubectl" || splitResp[1] != "retina" || splitResp[2] != "capture" || splitResp[3] != "create" {
					return fmt.Errorf("invalid capture command: %s", resp)
				}

				if !c.AutoRun {
					fmt.Print("Execute? ")
					val, err = c.ReadExpectedValues("y", "n")
					if err != nil {
						return err
					}

					if val == "n" {
						c.PrintAndStoreMessage(Retina, "Aborting Capture.")
						fmt.Println(responseDivider())

						// prompt user input
						fmt.Println("hint: to quit out of capture loop, say \"quit\"")
						userInput, err = c.ReadAndStoreUserInput()
						if err != nil {
							return err
						}
						fmt.Println(responseDivider())

						if userInput == "quit" {
							break
						}

						continue
					}
				}

				// splitResp[len(splitResp)-1] = strings.Replace(splitResp[len(splitResp)-1], "<BLOB_URL>", c.BlobURL, -1)
				splitResp = append(splitResp, "--namespace=kube-system", "--debug")
				// fmt.Printf("splitresp: %+v", splitResp)

				fmt.Println("Running...")
				fmt.Println(chatDivider())
				fmt.Println()

				var cap *retinav1alpha1.Capture
				if !skipCapture {
					// very hacky way to get capture name!

					image := "acnpublic.azurecr.io/retina-agent:linux-amd64-v0.0.11-44-g25bc9fa"
					args := []string{}
					args = append(args, splitResp[2:]...)
					args = append(args, fmt.Sprintf("--blob-upload=%s", c.BlobURL))
					os.Setenv("RETINA_AGENT_IMAGE", image)
					fmt.Println("passing args", args)

					capture.CaptureCmd(true)
					createCmd := capture.CaptureCmdCreate()
					createCmd.Flags().Parse(args)

					cap, err = capture.CaptureCreate(cmd, args)

					cmd := exec.Command("kubectl-retina", args...)
					cmd.Env = os.Environ()
					cmd.Env = append(cmd.Env, fmt.Sprintf("RETINA_AGENT_IMAGE=%s", image))

					output, _ := cmd.CombinedOutput()
					fmt.Println("OUTPUT: ")
					fmt.Println(string(output))
				}

				if err != nil {
					fmt.Println("failed:", err.Error())
				} else {
					fmt.Println("success")
				}
				fmt.Println(chatDivider())
				fmt.Println()

				if err != nil {
					// prompt user input
					fmt.Println("hint: to quit out of capture loop, say \"quit\"")
					userInput, err = c.ReadAndStoreUserInput()
					if err != nil {
						return err
					}
					fmt.Println(responseDivider())

					if userInput == "quit" {
						break
					}

					continue
				}

				captureName := "retina-capture-jgggm"
				if !skipCapture {
					captureName = cap.Name
				}
				c.PrintAndStoreMessage(Retina, fmt.Sprintf("Downloading capture %s from blob...", captureName))
				fmt.Println(responseDivider())

				// TODO(optimization) allow user [y/n] option unless there's AutoRun
				fmt.Println(chatDivider())
				fmt.Println()
				blobName, err := capture.Download(cmd, &captureName)
				if err != nil {
					fmt.Println("failed:", err.Error())
				} else {
					fmt.Println("success")
				}
				fmt.Println(chatDivider())
				fmt.Println()

				if err != nil {
					fmt.Print("Retry download? ")
					val, err = c.ReadExpectedValues("y", "n")
					if err != nil {
						return err
					}

					if val == "n" {
						break
					}

					continue
				}

				c.Captured = true

				// unzip
				// TODO(optimization) allow user [y/n] option unless there's AutoRun
				fmt.Println(chatDivider())
				fmt.Println()
				index := strings.Index(blobName, ".tar.gz")
				if index == -1 {
					fmt.Println("failed: unable to find .tar.gz in blob name")
					break
				}

				folder := "captures/" + blobName[:index]
				_, _ = c.Exec.Command("mkdir", "-p", folder).CombinedOutput()
				_, err = c.Exec.Command("tar", "-xvf", blobName, "-C", folder).CombinedOutput()
				if err != nil {
					fmt.Println("failed to unzip (maybe it was already unzipped):", err.Error())
					// c.PrintAndStoreMessage("OUTPUT", string(output))
				} else {
					c.PrintAndStoreMessage("OUTPUT", "success")
				}

				// keep track of capture file
				// use "grep flow" for trace
				keyword := ".pcap"
				if captureIsActuallyTrace {
					keyword = "flow"
				}
				output, err := c.Exec.Command("bash", "-c", fmt.Sprintf("ls %s | grep %s", folder, keyword)).CombinedOutput()
				if err != nil {
					return fmt.Errorf("error finding file: %w", err)
				}
				c.CaptureFile = folder + "/" + strings.TrimSpace(string(output))
				fmt.Println("capture file:", c.CaptureFile)
				output, _ = c.Exec.Command("pwd").CombinedOutput()
				fmt.Println(string(output))
				fmt.Println(chatDivider())
				fmt.Println()

				c.PrintAndStoreMessage(Retina, "Let's analyze the result. What should we look into?")
				fmt.Println(responseDivider())

				if captureIsActuallyTrace {
					c.Traced = true
					c.TraceFile = c.CaptureFile
					c.Captured = false
					c.CaptureFile = ""
				}

				break
			}

			continue
			// }

			// if val == "t" {
			// 	//trace
			// 	c.Captured = false
			// 	for {
			// 		fmt.Println("TRACE UNIMPLEMENTED. Need to copy/paste from capture logic")
			// 		// TODO copy paste and make two modifications:
			// 		// use trace prompt/command instead
			// 		// search for "flow" instead of ".pcap" and save that into the trace file

			// 		break

			// 		// would need to set these before breaking
			// 		c.Traced = true
			// 		c.TraceFile = "TODO"
			// 	}
			// 	continue
			// }

			// neither
			continue
		}

		// 3. parse
		// if strings.Contains(resp, "PARSE") {
		// 	if (c.CaptureFile == "" && c.TraceFile == "") || (!c.Captured && !c.Traced) {
		// 		c.PrintAndStoreMessage(Retina, "No capture or trace file to parse.")
		// 		fmt.Println(responseDivider())
		// 		continue
		// 	}

		// 	c.PrintAndStoreMessage(Retina, "Parsing file...")
		// 	fmt.Println(chatDivider())
		// 	if c.Captured && c.CaptureFile != "" {
		// 		output, err := c.Exec.Command("kubectl-retina", "parse", "--pcap-file", c.CaptureFile, "--enrich").CombinedOutput()
		// 		fmt.Println(string(output))
		// 		if err != nil {
		// 			return fmt.Errorf("error executing command: %w", err)
		// 		}
		// 	} else if c.Traced && c.TraceFile != "" {
		// 		output, err := c.Exec.Command("kubectl-retina", "parse", "--trace-file", c.TraceFile, "--enrich").CombinedOutput()
		// 		fmt.Println(string(output))
		// 		if err != nil {
		// 			return fmt.Errorf("error executing command: %w", err)
		// 		}
		// 	}

		// 	fmt.Println(chatDivider())
		// 	continue
		// }

		// 4. analyze
		if strings.Contains(resp, "ANALYZE") || strings.Contains(resp, "PARSE") {
			if (c.CaptureFile == "" && c.TraceFile == "") || (!c.Captured && !c.Traced) {
				c.PrintAndStoreMessage(Retina, "No capture or trace file to analyze.")
				continue
			}

			c.PrintAndStoreMessage(Retina, "Parsing file...")
			fmt.Println(chatDivider())
			fmt.Println()
			var cmd utilexec.Cmd
			if c.Captured && c.CaptureFile != "" {
				// output, err := c.Exec.Command("kubectl-retina", "ai", "analyze", "--pcap-file", c.CaptureFile, "--question", userInput).CombinedOutput()
				// fmt.Println(string(output))
				// if err != nil {
				// 	return fmt.Errorf("error executing command: %w", err)
				// }
				cmd = c.Exec.Command("kubectl-retina", "ai", "analyze", "--pcap-file", c.CaptureFile, "--question", userInput)
			} else if c.Traced && c.TraceFile != "" {
				// output, err := c.Exec.Command("kubectl-retina", "ai", "analyze", "--trace-file", c.TraceFile, "--question", userInput).CombinedOutput()
				// fmt.Println(string(output))
				// if err != nil {
				// 	return fmt.Errorf("error executing command: %w", err)
				// }
				cmd = c.Exec.Command("kubectl-retina", "ai", "analyze", "--trace-file", c.TraceFile, "--question", userInput)
			}
			out := retry(cmd)
			fmt.Println(out)

			fmt.Println(chatDivider())
			fmt.Println()
			continue
		}

		// bad response from GPT
		c.PrintAndStoreMessage(Retina, "Unable to triage request.")
		fmt.Println(responseDivider())
	}
}

func retry(cmd utilexec.Cmd) (res string) {
	b := wait.Backoff{
		Steps:    5,
		Duration: 1 * time.Second,
		Factor:   2,
		Jitter:   0,
	}
	wait.ExponentialBackoff(b, func() (bool, error) {
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Println("Retrying...")
			return false, err
		}
		if string(output) == "" {
			fmt.Println("Retrying...")
			return false, nil
		}
		res = string(output)
		return true, nil
	})
	return
}

func (c *ChatAgent) ReadAndStoreUserInput() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s : ", User)
	for {
		// must hit enter to continue
		userInput, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("error reading input: %w", err)
		}

		userInput = strings.TrimSpace(userInput)
		if userInput == "" {
			continue
		}

		c.Memory = append(c.Memory, &ChatMessage{
			Role: User,
			Text: userInput,
		})
		return userInput, nil
	}
}

func (c *ChatAgent) ReadExpectedValues(vals ...string) (string, error) {
	if len(vals) == 0 {
		return "", fmt.Errorf("no expected values given")
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("[%s]: ", strings.Join(vals, "/"))
		// must hit enter to continue
		userInput, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("error reading input: %w", err)
		}

		userInput = strings.TrimSpace(userInput)
		for _, val := range vals {
			if userInput == val {
				return userInput, nil
			}
		}

		// retry
		reader.Reset(os.Stdin)
		fmt.Printf("invalid choice, please try again\n")
	}
}

func (c *ChatAgent) chatTriagePrompt(resp string) string {
	sb := strings.Builder{}
	sb.WriteString("You are RETINA. You are an expert at Kubernetes, bash commands, and network diagnostics.")
	sb.WriteString("Determine which task RETINA should perform.")

	if !c.Captured && !c.Traced {
		sb.WriteString("There are three kinds of tasks: 1) UNKNOWN-TASK 2) KUBECTL-BASH-COMMAND, 3) PACKET-CAPTURE-OR-TRACE.")
		sb.WriteString("Follow this logic:")
		sb.WriteString("- If the USER's problem/question is related to trace, capture, packets or communication between network entities, then perform \"PACKET-CAPTURE-OR-TRACE\".")
		sb.WriteString("- If the USER's problem/question requires a kubectl command, then perform \"KUBECTL-BASH-COMMAND\".")
		sb.WriteString("- Lastly, if you think the USER's statement is not relevant to the numbered tasks above, then you should choose \"1) UNKNOWN-TASK\".")
	} else {
		sb.WriteString("There are five kinds of tasks: 1) UNKNOWN-TASK 2) KUBECTL-BASH-COMMAND, 3) PARSE, 4) ANALYZE, OR 5) NEW-PACKET-CAPTURE-OR-TRACE.")
		sb.WriteString("Follow this logic:")
		sb.WriteString("- If the USER is asking for a new packet capture or a new trace, then perform \"NEW-PACKET-CAPTURE-OR-TRACE\".")
		sb.WriteString("- If the USER's problem/question is related to packets or communication between network entities, then perform \"ANALYZE\".")
		sb.WriteString("- If the USER's asks to parse the file, then perform \"PARSE\".")
		sb.WriteString("- If the USER's problem/question requires a kubectl command, then perform \"KUBECTL-BASH-COMMAND\".")
		sb.WriteString("- Lastly, if you think the USER's statement is not relevant to the numbered tasks above, then you should choose \"1) UNKNOWN-TASK\".")
	}

	sb.WriteString("\n")
	sb.WriteString("Chat History:")
	for _, msg := range c.Memory {
		sb.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Text))
	}
	sb.WriteString("\n")
	sb.WriteString("Read the bottom message in the chat again. Which of the capitalized tasks above should RETINA perform for the bottom message? To arrive at your answer, include your reasoning in a very short sentence.")
	return sb.String()
}

func (c *ChatAgent) kubectlBashPrompt(userInput string) string {
	sb := strings.Builder{}
	sb.WriteString("You are RETINA. You are an expert at Kubernetes and bash commands.")

	sb.WriteString("Provide a \"kubectl get\" command that will help answer the bottom message.")
	sb.WriteString("You may pipe outputs into other commands. In all, you may only use the following commands:\n")
	for _, command := range allowedToPipeToCommands {
		sb.WriteString(command)
		sb.WriteString("\n")
	}
	sb.WriteString("Your answer should be only the bash command (piped to other commands as necessary).\n")
	sb.WriteString("QUESTION: How many Pods are in the test namespace?\n")
	sb.WriteString("ANSWER: kubectl get pods -n test -o name | wc -l\n")
	sb.WriteString("QUESTION: Which Pods are not Running?\n")
	sb.WriteString("ANSWER: kubectl get pods --all-namespaces -o json | jq '.items[] | select(.status.phase != \"Running\") | .metadata.namespace + \"/\" + .metadata.name'\n")
	sb.WriteString("QUESTION: NetPols selecting key1=val1 in default namespace\n")
	sb.WriteString("ANSWER: kubectl get netpol -n default -o json | jq '.items[] | select(.spec.podSelector.matchLabels.key1 == \"val1\") | .metadata.namespace + \"/\" + .metadata.name'\n")
	sb.WriteString("QUESTION: Linux nodes\n")
	sb.WriteString("ANSWER: kubectl get nodes --show-labels | grep linux\n")

	sb.WriteString("\n")
	sb.WriteString("Chat History:")
	for i, msg := range c.Memory {
		if i == len(c.Memory)-1 {
			// skip last user message so we can display it later
			break
		}
		sb.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Text))
	}
	sb.WriteString("\n")

	sb.WriteString("Given the context in the Chat History, provide a command for the USER that will help answer the below QUESTION, responding in the same format as the above examples.\n")
	sb.WriteString(fmt.Sprintf("QUESTION: %s\n", userInput))
	sb.WriteString("ANSWER: ")

	return sb.String()
}

type Chat struct {
	GPT     *GPT
	BlobURL string
	Memory  []*ChatMessage
}

func (c *Chat) Init() error {
	s := os.Getenv(capture.BLOB_URL)
	// fmt.Println("blob url:", s)
	purl, err := url.Parse(s)
	if err != nil {
		return fmt.Errorf("error parsing blob url: %w", err)
	}
	c.BlobURL = purl.String()
	if c.BlobURL == "" {
		return capture.ErrEmptyBlobURL
	}

	return nil
}

func (c *Chat) PrintAndStoreMessage(role, text string) {
	c.Memory = append(c.Memory, &ChatMessage{
		Role: role,
		Text: text,
	})

	fmt.Printf("%s: %s\n", role, text)
}

type ChatMessage struct {
	Role string
	Text string
}

func ChatCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chat",
		Short: "chat bot providing insights/automation for debugging network issues and cluster state",
		RunE: func(cmd *cobra.Command, args []string) error {
			gpt := NewGPT(analyzeTemp, 0.95, 0, 0, analyzeMaxTokens)
			if err := gpt.Init(); err != nil {
				return err
			}

			chatAgent := NewChatAgent(gpt)
			if err := chatAgent.Init(); err != nil {
				return err
			}

			if err := chatAgent.Loop(cmd); err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}
