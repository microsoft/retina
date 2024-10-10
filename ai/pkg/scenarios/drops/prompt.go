package drops

// TODO implement below analysis logic in code and/or LM prompt

/*
	DROPS ANALYSIS LOGIC

	Primary questions:
	- Is there dropped traffic? What are the source, destination, protocol, and port?
	- Are there TCP SYNs missing SYN-ACKs? What are the source, destination, and port?
	- Same for TCP resets.
	- Which connections are successful? Compare this to above.
	- Are Nodes experiencing NIC issues?
*/

const (
	systemPrompt = `You are an assistant with expertise in Kubernetes Networking. The user is debugging networking issues on their Pods and/or Nodes. Provide a succinct summary identifying any issues in the "summary of network flow logs" provided by the user.`

	// first parameter is the user's question
	// second parameter is the user's network flow logs
	messagePromptTemplate = `%s

"summary of network flow logs":
%s
`
)
