package aws

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/rabobank/mfsb/conf"
	"github.com/rabobank/mfsb/db"
	"github.com/rabobank/mfsb/model"
	"github.com/rabobank/mfsb/util"
	"strings"
)

const (
	KeepBackupsDefault       = false
	SkipFinalSnapshotDefault = false
	NumInstancesDOCDBDefault = 1
)

// fixed values
var schemas = make(map[string]string)
var auditLog = "audit"
var docdbLogs = []*string{&auditLog}
var errorLog = "error"
var postgresqlLog = "postgresql"

func init() {
	schemas["mysql"] = "mysql://%[1]s:%[2]s@%[3]s:%[4]d/%[5]s"
	schemas["postgres"] = "postgresql://%[1]s:%[2]s@%[3]s:%[4]d/%[5]s"
	schemas["mariadb"] = "mariadb://%[1]s:%[2]s@%[3]s:%[4]d/%[5]s"
	schemas["docdb"] = "mongodb://%[1]s:%[2]s@%[3]s:%[4]d/"
	//schemas["docdb"] = "mongodb://%[1]s:%[2]s@%[3]s:%[4]d/?%[5]s"
	// mongodb://docdbadmin:<insertYourPassword>@docdb-2021-05-18-15-51-41.cluster-ced1datu3hwp.eu-west-1.docdb.amazonaws.com:27017/?ssl=true&ssl_ca_certs=rds-combined-ca-bundle.pem&replicaSet=rs0&readPreference=secondaryPreferred&retryWrites=false
}

func SubmitProvisioning(iaasInstanceId int64) error {
	serviceInstance := db.GetServiceInstanceByEnvAndIaaSId(conf.CfEnv, iaasInstanceId)
	serviceName := util.GetServiceById(serviceInstance.ServiceId).Name
	iaasInstance := db.GetIaaSInstances(iaasInstanceId)[0]
	if strings.HasPrefix(serviceName, "rds-service") {
		return SubmitProvisionRDSDB(iaasInstance, serviceInstance)
	}
	if strings.HasPrefix(serviceName, "documentdb-service") {
		return SubmitProvisionDOCDB(iaasInstance, serviceInstance)
	}
	return errors.New(fmt.Sprintf("service %s is not supported", serviceName))
}

func StartPollForStatus(iaasInstanceId int64) {
	serviceInstance := db.GetServiceInstanceByEnvAndIaaSId(conf.CfEnv, iaasInstanceId)
	serviceName := util.GetServiceById(serviceInstance.ServiceId).Name
	iaasInstance := db.GetIaaSInstances(iaasInstanceId)[0]
	if strings.HasPrefix(serviceName, "rds-service") {
		StartPollForStatusRDSDB(iaasInstance)
		return
	}
	if strings.HasPrefix(serviceName, "documentdb-service") {
		StartPollForStatusDOCDB(iaasInstance)
		return
	}
	fmt.Printf("no polling available for service %s", serviceName)
}

// UpdateInProgressStatus When a database creation or deletion is in progress and mfsb is restarted, then the goroutine(s) that update the status are no longer running, that's why we start it here.
func UpdateInProgressStatus() {
	serviceInstances := db.GetServiceInstances(0)
	for _, serviceInstance := range serviceInstances {
		if serviceInstance.Status == db.StatusInProgress && serviceInstance.Env == conf.CfEnv {
			serviceName := util.GetServiceById(serviceInstance.ServiceId).Name
			if strings.HasPrefix(serviceName, "rds-service") {
				StartPollForStatusRDSDB(db.GetIaaSInstances(serviceInstance.IaaSInstanceId)[0])
			} else if strings.HasPrefix(serviceName, "documentdb-service") {
				StartPollForStatusDOCDB(db.GetIaaSInstances(serviceInstance.IaaSInstanceId)[0])
			} else {
				fmt.Printf("service %s is not supported\n", serviceName)
			}
		}
	}
}

func LogAwsError(err error) {
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case rds.ErrCodeDBInstanceAlreadyExistsFault:
			fmt.Printf("AWS error code: %s, error: %s\n", rds.ErrCodeDBInstanceAlreadyExistsFault, aerr.Error())
		case rds.ErrCodeInsufficientDBInstanceCapacityFault:
			fmt.Printf("AWS error code: %s, error: %s\n", rds.ErrCodeInsufficientDBInstanceCapacityFault, aerr.Error())
		case rds.ErrCodeDBParameterGroupNotFoundFault:
			fmt.Printf("AWS error code: %s, error: %s\n", rds.ErrCodeDBParameterGroupNotFoundFault, aerr.Error())
		case rds.ErrCodeDBSecurityGroupNotFoundFault:
			fmt.Printf("AWS error code: %s, error: %s\n", rds.ErrCodeDBSecurityGroupNotFoundFault, aerr.Error())
		case rds.ErrCodeInstanceQuotaExceededFault:
			fmt.Printf("AWS error code: %s, error: %s\n", rds.ErrCodeInstanceQuotaExceededFault, aerr.Error())
		case rds.ErrCodeStorageQuotaExceededFault:
			fmt.Printf("AWS error code: %s, error: %s\n", rds.ErrCodeStorageQuotaExceededFault, aerr.Error())
		case rds.ErrCodeDBSubnetGroupNotFoundFault:
			fmt.Printf("AWS error code: %s, error: %s\n", rds.ErrCodeDBSubnetGroupNotFoundFault, aerr.Error())
		case rds.ErrCodeDBSubnetGroupDoesNotCoverEnoughAZs:
			fmt.Printf("AWS error code: %s, error: %s\n", rds.ErrCodeDBSubnetGroupDoesNotCoverEnoughAZs, aerr.Error())
		case rds.ErrCodeInvalidDBClusterStateFault:
			fmt.Printf("AWS error code: %s, error: %s\n", rds.ErrCodeInvalidDBClusterStateFault, aerr.Error())
		case rds.ErrCodeInvalidSubnet:
			fmt.Printf("AWS error code: %s, error: %s\n", rds.ErrCodeInvalidSubnet, aerr.Error())
		case rds.ErrCodeInvalidVPCNetworkStateFault:
			fmt.Printf("AWS error code: %s, error: %s\n", rds.ErrCodeInvalidVPCNetworkStateFault, aerr.Error())
		case rds.ErrCodeProvisionedIopsNotAvailableInAZFault:
			fmt.Printf("AWS error code: %s, error: %s\n", rds.ErrCodeProvisionedIopsNotAvailableInAZFault, aerr.Error())
		case rds.ErrCodeOptionGroupNotFoundFault:
			fmt.Printf("AWS error code: %s, error: %s\n", rds.ErrCodeOptionGroupNotFoundFault, aerr.Error())
		case rds.ErrCodeDBClusterNotFoundFault:
			fmt.Printf("AWS error code: %s, error: %s\n", rds.ErrCodeDBClusterNotFoundFault, aerr.Error())
		case rds.ErrCodeStorageTypeNotSupportedFault:
			fmt.Printf("AWS error code: %s, error: %s\n", rds.ErrCodeStorageTypeNotSupportedFault, aerr.Error())
		case rds.ErrCodeAuthorizationNotFoundFault:
			fmt.Printf("AWS error code: %s, error: %s\n", rds.ErrCodeAuthorizationNotFoundFault, aerr.Error())
		case rds.ErrCodeKMSKeyNotAccessibleFault:
			fmt.Printf("AWS error code: %s, error: %s\n", rds.ErrCodeKMSKeyNotAccessibleFault, aerr.Error())
		case rds.ErrCodeDomainNotFoundFault:
			fmt.Printf("AWS error code: %s, error: %s\n", rds.ErrCodeDomainNotFoundFault, aerr.Error())
		case rds.ErrCodeBackupPolicyNotFoundFault:
			fmt.Printf("AWS error code: %s, error: %s\n", rds.ErrCodeBackupPolicyNotFoundFault, aerr.Error())
		default:
			fmt.Printf("AWS error of unknown code, error: %s\n", aerr.Error())
		}
	} else {
		// Print the error, cast err to awserr.Error to get the Code and
		// Message from an error.
		fmt.Printf("AWS error of unknown code, error: %s\n", aerr.Error())
	}
}

// go over the given parameters and override the default values
func processParameters(serviceInstance db.ServiceInstance) (string, model.Parameters, error) {
	var parameters model.Parameters
	err := json.Unmarshal([]byte(serviceInstance.Parameters), &parameters)
	userName := UserNameDefault
	if err == nil {
		// RDS parameters
		allocatedStorageGB = AllocatedStorageGBDefault
		if parameters.AllocatedStorageGB != 0 {
			allocatedStorageGB = parameters.AllocatedStorageGB
			fmt.Printf("parameter override for AllocatedStorageGB:%d\n", allocatedStorageGB)
		}
		engine = EngineDefault
		logs = nil
		if parameters.Engine != "" {
			engine = parameters.Engine
			fmt.Printf("parameter override for Engine:%s\n", engine)
			switch engine {
			case "postgres":
				logs = append(logs, &postgresqlLog)
				userName = "postgres"
			default:
				logs = append(logs, &errorLog, &auditLog)
			}
		}
		dbName = DBNameDefault
		if parameters.DBName != "" {
			dbName = parameters.DBName
			fmt.Printf("parameter override for DBName:%s\n", dbName)
		}
		multiAZ = MultiAZDefault
		if parameters.MultiAZ {
			multiAZ = parameters.MultiAZ
			fmt.Printf("parameter override for MultiAZ:%t\n", multiAZ)
		}
		keepBackups = KeepBackupsDefault
		if parameters.KeepBackups {
			keepBackups = parameters.KeepBackups
			fmt.Printf("parameter override for KeepBackups:%t\n", keepBackups)
		}
		skipFinalSnapshot = SkipFinalSnapshotDefault
		if parameters.MakeFinalSnapshot != nil && !*parameters.MakeFinalSnapshot {
			skipFinalSnapshot = true
			fmt.Printf("parameter override for skipFinalSnapshot:%t\n", skipFinalSnapshot)
		}
		if parameters.RetentionDays != 0 {
			fmt.Printf("parameter override for RetentionDays:%d\n", parameters.RetentionDays)
		}
		autoMinorVersionUpgrade = AutoMinorVersionUpgradeDefault
		if parameters.AutoMinorVersionUpgrade {
			autoMinorVersionUpgrade = parameters.AutoMinorVersionUpgrade
			fmt.Printf("parameter override for AutoMinorVersionUpgrade:%t\n", autoMinorVersionUpgrade)
		}
		if parameters.RestoreFromSnapshot != "" {
			restoreFromSnapshot = parameters.RestoreFromSnapshot
			fmt.Printf("parameter override for RestoreFromSnapshot:%s\n", restoreFromSnapshot)
		}

		// DocDB parameters
		if parameters.NumDBInstances != 0 {
			if parameters.NumDBInstances > 3 {
				err = errors.New(fmt.Sprintf("you requested %d DocumentDB instances, the allowed maximum = 3", parameters.NumDBInstances))
			} else {
				fmt.Printf("parameter override for NumDBInstances:%d\n", parameters.NumDBInstances)
			}
		}
	}
	if err != nil {
		db.UpdateStatusServiceInstance(serviceInstance, db.StatusFailed)
		db.UpdateStatusIaaSInstance(db.GetIaaSInstances(serviceInstance.IaaSInstanceId)[0], db.StatusCreateFailed, err.Error())
	}
	return userName, parameters, err
}

func GetIAMTagsForServiceInstance(serviceInstance db.ServiceInstance) []*iam.Tag {
	var (
		tagList             []*iam.Tag
		serviceInstanceId   = "ServiceInstanceId"
		serviceInstanceName = "ServiceInstanceName"
		spaceName           = "SpaceName"
		orgName             = "OrganizationName"
		env                 = "CreatedFromEnv"
		serviceName         = "ServiceName"
		planName            = "PlanName"
		createdBy           = "CreatedBy"
		mfsb                = "mfsb"
	)
	tagList = append(tagList, &iam.Tag{Key: &serviceInstanceId, Value: &serviceInstance.ServiceId})
	tagList = append(tagList, &iam.Tag{Key: &serviceInstanceName, Value: &serviceInstance.InstanceName})
	tagList = append(tagList, &iam.Tag{Key: &spaceName, Value: &serviceInstance.SpaceName})
	tagList = append(tagList, &iam.Tag{Key: &orgName, Value: &serviceInstance.OrganizationName})
	tagList = append(tagList, &iam.Tag{Key: &env, Value: &serviceInstance.Env})
	tagList = append(tagList, &iam.Tag{Key: &createdBy, Value: &mfsb})
	plan := util.GetPlan(serviceInstance.ServiceId, serviceInstance.PlanId)
	tagList = append(tagList, &iam.Tag{Key: &planName, Value: &plan.Name})
	service := util.GetServiceById(serviceInstance.ServiceId)
	tagList = append(tagList, &iam.Tag{Key: &serviceName, Value: &service.Name})
	return tagList
}

// createIAMRoleIfNotExists - If the parameter AuthorizedAWSAccount was given, we create the required IAM role if it does not yet exist
func createIAMRoleIfNotExists(iaasInstanceP *db.IaaSInstance, serviceInstanceP *db.ServiceInstance) error {
	var err error
	iaasInstance := *iaasInstanceP
	serviceInstance := *serviceInstanceP
	var parameters model.Parameters
	err = json.Unmarshal([]byte(serviceInstance.Parameters), &parameters)
	if err != nil {
		return err
	}
	if parameters.AuthorizedAWSAccount != "" {
		// check if the role already exists
		listRolesOutput, err := conf.IAMClient.ListRoles(&iam.ListRolesInput{})
		if err != nil {
			return err
		}
		for _, roleP := range listRolesOutput.Roles {
			role := *roleP
			if *role.RoleName == iaasInstance.InternalId {
				return errors.New(fmt.Sprintf("IAM role %s already exists", iaasInstance.InternalId))
			}
		}

		// role doesn't exit yet, start creating it
		roleDescription := fmt.Sprintf("Allow limited access to docdb cluster %s for account %s", iaasInstance.InternalId, parameters.AuthorizedAWSAccount)
		roleName := fmt.Sprintf("mfsb-%s-%s", iaasInstance.InternalId, parameters.AuthorizedAWSAccount)
		var maxSessionDuration int64 = 7200
		doc := strings.ReplaceAll(conf.AssumeRolePolicyDoc, "@@AWSACCT@@", parameters.AuthorizedAWSAccount)
		role := "adfsdevadmin"
		if strings.HasPrefix(conf.CfEnv, "p") {
			role = "adfsoperator"
		}
		doc = strings.ReplaceAll(doc, "@@AWSROLE@@", role)
		createRoleInput := iam.CreateRoleInput{
			AssumeRolePolicyDocument: &doc,
			Description:              &roleDescription,
			MaxSessionDuration:       &maxSessionDuration,
			PermissionsBoundary:      &conf.PermissionBoundaryARN,
			RoleName:                 &roleName,
			Tags:                     GetIAMTagsForServiceInstance(serviceInstance),
		}
		createRoleOutput, err := conf.IAMClient.CreateRole(&createRoleInput)
		if err != nil {
			return err
		}
		fmt.Printf("created IAM role %s\n", *createRoleOutput.Role.RoleName)
		attachRolePolicyInput := iam.AttachRolePolicyInput{PolicyArn: &conf.PolicyARN, RoleName: &roleName}
		_, err = conf.IAMClient.AttachRolePolicy(&attachRolePolicyInput)
		if err != nil {
			return err
		}
		fmt.Printf("attached IAM role %s\n", roleName)
	}
	return err
}

// deleteIAMRoleIfExists - If the parameter AuthorizedAWSAccount was given, we delete the IAM role if it exists
func deleteIAMRoleIfExists(iaasInstanceP *db.IaaSInstance, serviceInstanceP *db.ServiceInstance) error {
	var err error
	iaasInstance := *iaasInstanceP
	serviceInstance := *serviceInstanceP
	var parameters model.Parameters
	err = json.Unmarshal([]byte(serviceInstance.Parameters), &parameters)
	if err != nil {
		return err
	}
	roleName := fmt.Sprintf("mfsb-%s-%s", iaasInstance.InternalId, parameters.AuthorizedAWSAccount)
	detachRolePolicyInput := iam.DetachRolePolicyInput{PolicyArn: &conf.PolicyARN, RoleName: &roleName}
	_, err = conf.IAMClient.DetachRolePolicy(&detachRolePolicyInput)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case iam.ErrCodeNoSuchEntityException:
				fmt.Printf("rolePolicy %s did not exist, not deleting it\n", conf.PolicyARN)
				return nil
			default:
				return err
			}
		}
	}
	fmt.Printf("detached IAM role %s from policy %s\n", roleName, conf.PolicyARN)

	deleteRoleInput := iam.DeleteRoleInput{RoleName: &roleName}
	_, err = conf.IAMClient.DeleteRole(&deleteRoleInput)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case iam.ErrCodeNoSuchEntityException:
				fmt.Printf("role %s did not exist, not deleting it\n", roleName)
				return nil
			default:
				return err
			}
		}
	}

	fmt.Printf("deleted IAM role %s\n", roleName)
	return err
}
