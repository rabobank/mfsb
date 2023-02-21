package model

// Context The context inside the ServiceInstance and ServiceBinding request
type Context struct {
	Platform     string `json:"platform"`
	OrgName      string `json:"organization_name"`
	SpaceName    string `json:"space_name"`
	InstanceName string `json:"instance_name"`
}

type LastOperation struct {
	State       string `json:"state"`
	Description string `json:"description"`
}
