package conf

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/docdb"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/cloudfoundry-community/go-cfenv"
	"github.com/rabobank/mfsb/model"
	"os"
	"strconv"
)

var (
	AWSSession  *session.Session
	RDSClient   *rds.RDS
	DOCDBClient *docdb.DocDB
	IAMClient   *iam.IAM
	Catalog     model.Catalog
	ListenPort  int
	Debug       = false

	DebugStr              = os.Getenv("MFSB_DEBUG")
	IaaS                  = os.Getenv("MFSB_IAAS")
	BrokerUser            = os.Getenv("MFSB_BROKER_USER")
	BrokerDBUser          = os.Getenv("MFSB_BROKER_DB_USER")
	BrokerDBName          = os.Getenv("MFSB_BROKER_DB_NAME")
	BrokerDBHost          = os.Getenv("MFSB_BROKER_DB_HOST")
	CatalogDir            = os.Getenv("MFSB_CATALOG_DIR")
	ListenPortStr         = os.Getenv("MFSB_LISTEN_PORT")
	CfEnv                 = os.Getenv("MFSB_CF_ENV")
	RDSSubnetGrp          = os.Getenv("MFSB_RDS_SUBNETGRP")
	DOCDBSubnetGrp        = os.Getenv("MFSB_DOCDB_SUBNETGRP")
	RDSSecGrpId           = os.Getenv("MFSB_RDS_SECGRP_ID")
	DOCDBSecGrpId         = os.Getenv("MFSB_DOCDB_SECGRP_ID")
	AWSRegion             = os.Getenv("MFSB_AWS_REGION")
	PermissionBoundaryARN = os.Getenv("MFSB_PERMISSION_BOUNDARY_ARN")
	PolicyARN             = os.Getenv("MFSB_POLICY_ARN")

	BrokerPassword   string
	BrokerDBPassword string
	EncryptKey       string

	// a map of rds database classes keyed by planname
	RDSDBClasses = make(map[string]string)
	DOCDBClasses = make(map[string]string)

	az1 = "eu-west-1a"
	az2 = "eu-west-1b"
	az3 = "eu-west-1c"
	AZS = []*string{&az1, &az2, &az3}

	AssumeRolePolicyDoc = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::@@AWSACCT@@:role/@@AWSROLE@@"
      },
      "Action": [
        "sts:AssumeRole",
        "sts:TagSession"
      ]
    }
  ]
  }`
)

const BasicAuthRealm = "MFSB - Multi Foundation Service Broker"

func EnvironmentComplete() {
	// setup plan to database class // TODO should be externally configurable (envvars or something else)
	RDSDBClasses["micro"] = "db.t3.micro"
	RDSDBClasses["small"] = "db.t3.small"
	RDSDBClasses["medium"] = "db.t3.medium"

	DOCDBClasses["micro"] = "db.t3.medium"
	DOCDBClasses["small"] = "db.r5.large"
	DOCDBClasses["medium"] = "db.r5.xlarge"

	envComplete := true
	if DebugStr == "true" {
		Debug = true
	}
	if IaaS == "" {
		envComplete = false
		fmt.Println("missing envvar: MFSB_IAAS")
	}
	if BrokerUser == "" {
		envComplete = false
		fmt.Printf("missing envvar: MFSB_BROKER_USER")
	}
	if BrokerDBUser == "" {
		envComplete = false
		fmt.Println("missing envvar: MFSB_BROKER_DB_USER")
	}
	if BrokerDBName == "" {
		BrokerDBName = "mfsbdb"
	}
	if BrokerDBHost == "" {
		BrokerDBHost = "localhost"
	}
	if CatalogDir == "" {
		CatalogDir = "catalog"
	}
	if ListenPortStr == "" {
		ListenPort = 8080
	} else {
		var err error
		ListenPort, err = strconv.Atoi(ListenPortStr)
		if err != nil {
			fmt.Printf("failed reading envvar MFSB_LISTEN_PORT, err: %s\n", err)
			envComplete = false
		}
	}
	if CfEnv == "" {
		envComplete = false
		fmt.Println("missing envvar: MFSB_CF_ENV")
	}
	if RDSSubnetGrp == "" {
		envComplete = false
		fmt.Println("missing envvar: MFSB_RDS_SUBNETGRP")
	}
	if RDSSecGrpId == "" {
		envComplete = false
		fmt.Println("missing envvar: MFSB_RDS_SECGRP_ID")
	}
	if AWSRegion == "" {
		envComplete = false
		fmt.Println("missing envvar: MFSB_AWS_REGION")
	}
	if DOCDBSubnetGrp == "" {
		envComplete = false
		fmt.Println("missing envvar: MFSB_DOCDB_SUBNETGRP")
	}
	if DOCDBSecGrpId == "" {
		envComplete = false
		fmt.Println("missing envvar: MFSB_DOCDB_SECGRP_ID")
	}
	if PermissionBoundaryARN == "" {
		envComplete = false
		fmt.Println("missing envvar: MFSB_PERMISSION_BOUNDARY_ARN")
	}
	if PolicyARN == "" {
		envComplete = false
		fmt.Println("missing envvar: MFSB_POLICY_ARN")
	}

	if !envComplete {
		fmt.Println("one or more required envvars missing, aborting...")
		os.Exit(8)
	}

	initCredentials()
}

// initCredentials - Get the credentials from credhub (VCAP_SERVICES envvar)
func initCredentials() {
	fmt.Println("getting credentials from credhub...")
	if appEnv, err := cfenv.Current(); err == nil {
		services, err := appEnv.Services.WithLabel("credhub")
		if err == nil {
			if len(services) != 1 {
				fmt.Printf("we expected exactly one bound credhub service instance, but found %d\n", len(services))
			} else {
				EncryptKey = fmt.Sprint(services[0].Credentials["MFSB_ENCRYPT_KEY"])
				BrokerPassword = fmt.Sprint(services[0].Credentials["MFSB_BROKER_PASSWORD"])
				BrokerDBPassword = fmt.Sprint(services[0].Credentials["MFSB_BROKER_DB_PASSWORD"])
				allVarsFound := true
				if EncryptKey == "" {
					fmt.Printf("credhub variable MFSB_ENCRYPT_KEY is missing")
					allVarsFound = false
				}
				if BrokerPassword == "" {
					fmt.Printf("credhub variable MFSB_BROKER_PASSWORD is missing")
					allVarsFound = false
				}
				if BrokerDBPassword == "" {
					fmt.Printf("credhub variable MFSB_BROKER_DB_PASSWORD is missing")
					allVarsFound = false
				}
				if !allVarsFound {
					os.Exit(8)
				}
			}
		} else {
			fmt.Printf("failed getting services from cf env: %s\n", err)
			os.Exit(8)
		}
	} else {
		fmt.Printf("failed to get the current cf env: %s\n", err)
		os.Exit(8)
	}
}
