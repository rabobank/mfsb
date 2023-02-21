package model

type Catalog struct {
	Services []Service `json:"services"`
}

type Service struct {
	Name            string        `json:"name"`
	Id              string        `json:"id"`
	Description     string        `json:"description"`
	Bindable        bool          `json:"bindable"`
	MaxPollInterval int           `json:"maximum_polling_duration"`
	PlanUpdateable  bool          `json:"plan_updateable,omitempty"`
	Tags            []string      `json:"tags,omitempty"`
	Requires        []string      `json:"requires,omitempty"`
	Metadata        interface{}   `json:"metadata,omitempty"`
	Plans           []ServicePlan `json:"plans"`
	DashboardClient interface{}   `json:"dashboard_client"`
}

type ServicePlan struct {
	Name        string      `json:"name"`
	Id          string      `json:"id"`
	Description string      `json:"description"`
	Metadata    interface{} `json:"metadata,omitempty"`
	Free        bool        `json:"free,omitempty"`
}
