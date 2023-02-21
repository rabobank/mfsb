package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/docdb"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/rds"
	aws2 "github.com/rabobank/mfsb/aws"
	"github.com/rabobank/mfsb/conf"
	"github.com/rabobank/mfsb/db"
	"github.com/rabobank/mfsb/server"
	"os"
)

func main() {
	fmt.Printf("mfsb starting for cf env %s, Commit:%s\n", conf.VERSION, conf.COMMIT)

	conf.EnvironmentComplete()

	initialize()

	// test the db access to tables
	fmt.Printf("found %d service instances\n", len(db.GetServiceInstances(0)))
	fmt.Printf("found %d iaas instances\n", len(db.GetIaaSInstances(0)))
	fmt.Printf("found %d service bindings\n", len(db.GetServiceBindings(0)))

	server.StartServer()
}

// initialize mfsb:
//   - read catalog file
//   - login to IaaS
//   - test database
//   - startup updating "in progress" IaaSInstances
func initialize() {
	catalogFile := fmt.Sprintf("%s/%s.json", conf.CatalogDir, conf.IaaS)
	file, err := os.ReadFile(catalogFile)
	if err != nil {
		fmt.Printf("failed reading catalog file %s: %s\n", catalogFile, err)
		os.Exit(8)
	}
	err = json.Unmarshal(file, &conf.Catalog)
	if err != nil {
		fmt.Printf("failed unmarshalling json from file %s, error: %s\n", catalogFile, err)
		os.Exit(8)
	}

	conf.AWSSession, err = session.NewSession(&aws.Config{Region: aws.String(conf.AWSRegion)})
	if err != nil {
		fmt.Printf("failed to create new AWS Session, error: %s\n", err)
		os.Exit(8)
	}
	if conf.Debug {
		fmt.Println("AWS session created")
	}
	conf.RDSClient = rds.New(conf.AWSSession)
	if conf.Debug {
		fmt.Println("AWS RDS client created")
	}
	conf.DOCDBClient = docdb.New(conf.AWSSession)
	if conf.Debug {
		fmt.Println("AWS DocumentDB client created")
	}
	conf.IAMClient = iam.New(conf.AWSSession)
	if conf.Debug {
		fmt.Println("AWS IAM client created")
	}

	// test if the DB can be opened
	database := db.GetDB()
	defer database.Close()

	aws2.UpdateInProgressStatus()
}
