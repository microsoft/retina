package ai

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func Capture() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "capture",
		Short: "converts natural language into a \"kubectl-retina capture create\" command",
		RunE: func(cmd *cobra.Command, args []string) error {
			gpt := NewGPT(0.3, 0.95, 0, 0, 100)
			if err := gpt.Init(); err != nil {
				return err
			}

			prompt, err := capturePrompt(cmd, args)
			if err != nil {
				return err
			}

			resp, err := gpt.Ask(prompt)
			if err != nil {
				return err
			}

			fmt.Println(resp)
			return nil
		},
	}

	return cmd
}

/*
Example
User Input: capture traffic on Windows nodes with a maximum file size of 10MB
GPT Answer: kubectl retina capture --node-selectors=kubernetes.io/os=windows --max-size=10MB
*/
func capturePrompt(cmd *cobra.Command, args []string) (string, error) {
	sb := strings.Builder{}
	sb.WriteString("you are a kubernetes expert, use the following documentation to learn about the kubectl retina capture command:")
	// err := cmd.Root().GenBashCompletion()
	// err := cmd.Root().GenBashCompletion(io.Writer(&sb))
	// if err != nil {
	// 	return "", err
	// }
	// err = doc.GenMarkdownTree(cmd.Root(), "./")
	// if err != nil {
	// 	return "", err
	// }

	sb.WriteString(`capture create --help
	create a Retina Capture
	
	Usage:
	   capture create [flags]
	
	Examples:
	  # Capture network packets on the node selected by node names and copy the artifacts to the node host path /mnt/capture
	  kubectl retina capture create --host-path /mnt/capture --namespace capture --node-names "aks-nodepool1-41844487-vmss000000,aks-nodepool1-41844487-vmss000001"
	  
	  # Capture network packets on the coredns pods determined by pod-selectors and namespace-selectors
	  kubectl retina capture create --host-path /mnt/capture --namespace capture --pod-selectors="k8s-app=kube-dns" --namespace-selectors="kubernetes.io/metadata.name=kube-system"
	  
	  # Capture network packets on nodes with label "agentpool=agentpool" and "version:v20"
	  kubectl retina capture create --host-path /mnt/capture --node-selectors="agentpool=agentpool,version:v20"
	  
	  # Capture network packets on nodes using node-selector with duration 10s
	  kubectl retina capture create --host-path=/mnt/capture --node-selectors="agentpool=agentpool" --duration=10s
	  
	  # Capture network packets on nodes using node-selector and upload the artifacts to blob storage with SAS URL https://testaccount.blob.core.windows.net/<token>
	  kubectl retina capture create --node-selectors="agentpool=agentpool" --blob-upload=https://testaccount.blob.core.windows.net/<token>
	
	Flags:
		  --as string                      Username to impersonate for the operation. User could be a regular user or a service account in a namespace.
		  --as-group stringArray           Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
		  --as-uid string                  UID to impersonate for the operation.
		  --blob-upload string             Blob SAS URL with write permission to upload capture files
		  --cache-dir string               Default cache directory (default "/home/azureuser/.kube/cache")
		  --certificate-authority string   Path to a cert file for the certificate authority
		  --client-certificate string      Path to a client certificate file for TLS
		  --client-key string              Path to a client key file for TLS
		  --cluster string                 The name of the kubeconfig cluster to use
		  --context string                 The name of the kubeconfig context to use
		  --debug                          When debug is true, a customized retina-agent image, determined by the environment variable RETINA_AGENT_IMAGE, is set
		  --disable-compression            If true, opt-out of response compression for all requests to the server
		  --duration duration              Duration of capturing packets (default 1m0s)
		  --exclude-filter string          A comma-separated list of IP:Port pairs that are excluded from capturing network packets. Supported formats are IP:Port, IP, Port, *:Port, IP:*
	  -h, --help                           help for create
		  --host-path string               HostPath of the node to store the capture files
		  --include-filter string          A comma-separated list of IP:Port pairs that are used to filter capture network packets. Supported formats are IP:Port, IP, Port, *:Port, IP:*
		  --include-metadata               If true, collect static network metadata into capture file (default true)
		  --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
		  --job-num-limit int              The maximum number of jobs can be created for each capture. 0 means no limit
		  --kubeconfig string              Path to the kubeconfig file to use for CLI requests.
		  --max-size int                   Limit the capture file to MB in size which works only for Linux (default 100)
	  -n, --namespace string               Namespace to host capture job (default "default")
		  --namespace-selectors string     A comma-separated list of namespace labels together with pod-selector to select pods on which the network capture will be performed. By default, the pod namespace is specified by the flag namespace
		  --no-wait                        Do not wait for the long-running capture job to finish (default true)
		  --node-names string              A comma-separated list of node names to select nodes on which the network capture will be performed
		  --node-selectors string          A comma-separated list of node labels to select nodes on which the network capture will be performed
		  --packet-size int                Limits the each packet to bytes in size which works only for Linux
		  --pod-selectors string           A comma-separated list of pod labels together with namespace-selector to select pods on which the network capture will be performed
		  --pvc string                     PersistentVolumeClaim under the specified or default namespace to store capture files
		  --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
	  -s, --server string                  The address and port of the Kubernetes API server
		  --tcpdump-filter string          Raw tcpdump flags which works only for Linux
		  --tls-server-name string         Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
		  --token string                   Bearer token for authentication to the API server
		  --user string                    The name of the kubeconfig user to use`)

	// sb.WriteString(". Now generate a kubectl-retina capture command for the following question, and only give the command without any description:")
	sb.WriteString("\nGiven the above auto-completion script, provide the command for the following task. Your response should contain only the command.")
	sb.WriteString("\n\nTASK: all windows Pods\n")
	sb.WriteString("RESPONSE: kubectl retina capture create --node-selectors=kubernetes.io/os=windows")
	sb.WriteString("\n\nTASK: pods with label key1=val1 or key2=val2 in the default namespace\n")
	sb.WriteString("RESPONSE: kubectl retina capture create --namespace-selectors=kubernetes.io/metadata.name=default --pod-selectors=pod=a,pod=b")

	sb.WriteString("\n\nTASK: ")

	for _, str := range args {
		sb.WriteString(str)
		sb.WriteString(" ")
	}

	sb.WriteString("\nRESPONSE: ")

	return sb.String(), nil
}
