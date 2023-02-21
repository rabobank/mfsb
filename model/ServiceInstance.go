package model

type ServiceInstance struct {
	ServiceId        string      `json:"service_id"`
	PlanId           string      `json:"plan_id"`
	OrganizationGuid string      `json:"organization_guid"`
	SpaceGuid        string      `json:"space_guid"`
	Context          *Context    `json:"context"`
	Parameters       *Parameters `json:"parameters,omitempty"`
}

type CreateServiceInstanceResponse struct {
	ServiceId     string         `json:"service_id"`
	PlanId        string         `json:"plan_id"`
	DashboardUrl  string         `json:"dashboard_url"`
	LastOperation *LastOperation `json:"last_operation,omitempty"`
}

type DeleteServiceInstanceResponse struct {
	Result string `json:"result,omitempty"`
}

// Parameters This is a superset of all potential parameters that can be given on the -c parameter of "cf create-service"
type Parameters struct {
	// common parameters
	AuthorizedAWSAccount string `json:"AuthorizedAWSAccount,omitempty"`
	RetentionDays        int64  `json:"RetentionDays,omitempty"`
	KeepBackups          bool   `json:"KeepBackups,omitempty"`
	// RDS parameters
	AllocatedStorageGB      int64  `json:"AllocatedStorageGB,omitempty"`
	Engine                  string `json:"Engine,omitempty"`
	DBName                  string `json:"DBName,omitempty"`
	MultiAZ                 bool   `json:"MultiAZ,omitempty"`
	MakeFinalSnapshot       *bool  `json:"MakeFinalSnapshot,omitempty"` // we make this a pointer so we can detect the diff between absence and value "false"
	AutoMinorVersionUpgrade bool   `json:"AutoMinorVersionUpgrade,omitempty"`
	// DOCDB parameters:
	NumDBInstances      int64  `json:"NumDBInstances,omitempty"`
	RestoreFromSnapshot string `json:"RestoreFromSnapshot,omitempty"`
}
