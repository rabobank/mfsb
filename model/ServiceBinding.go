package model

type ServiceBinding struct {
	ServiceId         string        `json:"service_id"`
	PlanId            string        `json:"plan_id"`
	AppGuid           string        `json:"app_guid"`
	ServiceInstanceId string        `json:"service_instance_id"`
	BindResource      *BindResource `json:"bind_resource"`
	Context           *Context      `json:"context"`
}

type BindResource struct {
	AppGuid   string `json:"app_guid"`
	SpaceGuid string `json:"space_guid"`
}

type CreateServiceBindingResponse struct {
	// SyslogDrainUrl string      `json:"syslog_drain_url, omitempty"`
	Credentials *Credentials `json:"credentials"`
}

type Credentials struct {
	Uri      string `json:"uri"`
	UserName string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	Database string `json:"database"`
}
