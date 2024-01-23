package ai

import (
	"fmt"
	"strings"

	"github.com/microsoft/retina/cli/cmd/parse"
	"github.com/spf13/cobra"
)

const (
	defaultPcapQuestion  = "Can you summarize these connections?"
	defaultTraceQuestion = "which pods are dropping packets?"
	analyzeTemp          = 0.3
	analyzeMaxTokens     = 800
)

var (
	ErrMustSpecifyFile      = fmt.Errorf("must specify either --pcap-file or --trace-file")
	ErrCantSupportBothFiles = fmt.Errorf("can't support both --pcap-file and --trace-file")
	ErrUnsupportedTraceFile = fmt.Errorf("trace files are not yet supported")
)

func Analyze() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "answers any question about a capture file",
		RunE: func(cmd *cobra.Command, args []string) error {
			gpt := NewGPT(analyzeTemp, 0.95, 0, 0, analyzeMaxTokens)
			if err := gpt.Init(); err != nil {
				return err
			}

			prompt, err := analyzePrompt(cmd, args)
			if err != nil {
				return err
			}

			debug, _ := cmd.Flags().GetBool("debug")
			if debug {
				fmt.Println("Prompt:", prompt)
			}

			resp, err := gpt.Ask(prompt)
			if err != nil {
				return err
			}

			fmt.Printf("Answer: ")
			fmt.Println(resp)
			return nil
		},
	}

	cmd.Flags().StringP("question", "q", "", "question to ask")
	cmd.Flags().String("pcap-file", "", "pcap file to parse")
	cmd.Flags().String("trace-file", "", "trace file to parse")
	cmd.Flags().Bool("json", false, "use json format for the parsed result sent to GPT")
	cmd.Flags().Bool("numeric", false, "when true, will NOT enrich IPs with Node/Pod/Service information")
	cmd.Flags().Bool("show-all", false, "show result of parsing the pcap or trace file")
	cmd.Flags().Bool("debug", false, "show verbose logging")
	return cmd
}

func analyzePrompt(cmd *cobra.Command, args []string) (string, error) {
	question, _ := cmd.Flags().GetString("question")
	pcapFile, _ := cmd.Flags().GetString("pcap-file")
	traceFile, _ := cmd.Flags().GetString("trace-file")
	isJson, _ := cmd.Flags().GetBool("json")
	numeric, _ := cmd.Flags().GetBool("numeric")
	showAll, _ := cmd.Flags().GetBool("show-all")
	debug, _ := cmd.Flags().GetBool("debug")

	if pcapFile == "" && traceFile == "" {
		return "", ErrMustSpecifyFile
	}

	if pcapFile != "" && traceFile != "" {
		return "", ErrCantSupportBothFiles
	}

	if question == "" {
		if pcapFile != "" {
			question = defaultPcapQuestion
		}

		if traceFile != "" {
			question = defaultTraceQuestion
		}

		fmt.Println("no question provided, using default question:", question)
		fmt.Println()

	} else {
		fmt.Println("Question:", question)
	}

	cfg := &parse.ParserConfig{
		Format:          "pretty",
		Enrich:          !numeric,
		Debug:           false,
		IPIncludeFilter: []string{
			// "10.224.0.252",
			// "10.224.1.122",
			// "10.224.0.216",
		},
	}

	if isJson {
		cfg.Format = "json"
	}

	var data string
	if pcapFile != "" {
		p := parse.NewPcapParser()
		if err := p.Parse(pcapFile, cfg); err != nil {
			return "", err
		}

		data = p.Result(cfg)
	} else {
		t := parse.NewTraceParser()
		if err := t.Parse(traceFile, cfg); err != nil {
			return "", fmt.Errorf("error parsing trace file: %v", err)
		}

		data = t.Result(cfg)
	}

	if debug {
		fmt.Println("Parsed data:", data)
	}

	if showAll {
		fmt.Println(data)
	}

	sb := strings.Builder{}
	sb.WriteString("You are an expert at troubleshooting network issues based on 5-tuple connection data on UDP and TCP connections. Read the following summary of all connections:\n")
	sb.WriteString(data)
	sb.WriteString("\nBased on the above summary of all connections, provide insights to the customer's question. Your response should be as brief as possible while still providing both a clear explanation and potential reasons.\n\n")
	sb.WriteString(fmt.Sprintf("Customer's Question: %s\nInsights: ", question))
	return sb.String(), nil
}
