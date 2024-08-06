package dns

// TODO implement below analysis logic in code and/or LM prompt

/*
	DNS ANALYSIS LOGIC

	Primary questions:
	- Do any queries have failing responses? Which?
	- Do any queries have no responses? Which?
	- Which Pods are impacted by above? Which are not?
	- Which core-dns Pods are impacted by above? Which are not?
	- Is "reserved:world" responding with errors or responding at all?

	More questions:
	- What kind of queries do we see (qualitatively)?
	- Do we see any issue by DNS record type?
	- Does number of IPs matter??
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
