package flows

const (
	systemPrompt = `You are an assistant with expertise in Kubernetes Networking. The user is debugging networking issues on their Pods and/or Nodes. Provide a succinct summary identifying any issues in the "summary of network flow logs" provided by the user.`

	// first parameter is the user's question
	// second parameter is the user's network flow logs
	messagePromptTemplate = `%s

"summary of network flow logs":
%s
`
)
