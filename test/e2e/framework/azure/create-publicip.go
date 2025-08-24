package azure

type CreatePublicIp struct {
	SubscriptionID    string
	ResourceGroupName string
	Location          string
	PublicIpName      string
	IPTagType         string
	Tag               string
}

func (c *CreatePublicIp) Run() error {
	return nil
}

func (c *CreatePublicIp) Prevalidate() error {
	return nil
}

func (c *CreatePublicIp) Stop() error {
	return nil
}
