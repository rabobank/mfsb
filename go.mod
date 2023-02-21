module github.com/rabobank/mfsb

go 1.20

require (
	github.com/aws/aws-sdk-go v1.44.134
	github.com/cloudfoundry-community/go-cfenv v1.18.0
	github.com/go-sql-driver/mysql v1.7.0
	github.com/gorilla/mux v1.8.0
)

require (
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/mitchellh/mapstructure v1.1.2 // indirect
	golang.org/x/text v0.5.0 // indirect
)

exclude (
	golang.org/x/net v0.1.0
	golang.org/x/text v0.3.0
	golang.org/x/text v0.3.3
	golang.org/x/text v0.3.7
	gopkg.in/yaml.v2 v2.2.1
)
