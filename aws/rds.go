package aws

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/rabobank/mfsb/conf"
	"github.com/rabobank/mfsb/db"
	"github.com/rabobank/mfsb/util"
)

const (
	AllocatedStorageGBDefault      = 5
	EngineDefault                  = "mariadb"
	UserNameDefault                = "admin"
	DBNameDefault                  = "db"
	MultiAZDefault                 = false
	AutoMinorVersionUpgradeDefault = true
	RetentionDaysRDSDefault        = 7
)

var allocatedStorageGB int64
var storageType = "gp2"

// var iops int64 = 50
var engine string
var keepBackups bool
var skipFinalSnapshot bool
var multiAZ bool
var restoreFromSnapshot string
var autoMinorVersionUpgrade bool
var dbName string

// values that are fixed:
var publiclyAccessible = false
var storageEncrypted = true
var copyTagsToSnapshot = true

var logs []*string

var vpcSecGrpIds []*string

func SubmitProvisionRDSDB(iaasInstance db.IaaSInstance, serviceInstance db.ServiceInstance) error {
	var err error
	userName, parameters, err := processParameters(serviceInstance)
	retentionDays := parameters.RetentionDays

	if err != nil {
		return err
	}
	// set the proper default for documentdb
	if retentionDays == 0 {
		retentionDays = RetentionDaysRDSDefault
	}
	vpcSecGrpIds = append(vpcSecGrpIds, &conf.RDSSecGrpId)
	iaasInstance.ServiceUser = userName
	iaasInstance.ServicePassword = util.SafeSubstring(fmt.Sprintf("pw%s", util.GenerateGUID()), 40)
	plan := util.GetPlan(serviceInstance.ServiceId, serviceInstance.PlanId)
	dbInstanceClass := conf.RDSDBClasses[plan.Name]
	if dbInstanceClass == "" {
		msg := fmt.Sprintf("could not find database instance class for plan %s", plan.Name)
		fmt.Println(msg)
		db.UpdateStatusIaaSInstance(iaasInstance, db.StatusCreateFailed, fmt.Sprintf("Database creation failed, error: %s", msg))
		serviceInstance.Status = db.StatusFailed
		_ = db.UpdateServiceInstance(serviceInstance)
		return errors.New(msg)
	}

	var dbInstanceCreateOutput *rds.CreateDBInstanceOutput
	var dbInstanceRestoreOutput *rds.RestoreDBInstanceFromDBSnapshotOutput

	if parameters.RestoreFromSnapshot != "" {
		if !snapshotExistsAndAuthorized(restoreFromSnapshot, serviceInstance) {
			msg := fmt.Sprintf("snapshot with identifier %s was not found or requestor is not authorized", restoreFromSnapshot)
			fmt.Println(msg)
			db.UpdateStatusIaaSInstance(iaasInstance, db.StatusCreateFailed, fmt.Sprintf("Database creation failed, error: %s", msg))
			serviceInstance.Status = db.StatusFailed
			_ = db.UpdateServiceInstance(serviceInstance)
			return errors.New(msg)
		} else {
			input := &rds.RestoreDBInstanceFromDBSnapshotInput{
				AutoMinorVersionUpgrade:     &autoMinorVersionUpgrade,
				CopyTagsToSnapshot:          &copyTagsToSnapshot,
				DBInstanceClass:             &dbInstanceClass,
				DBInstanceIdentifier:        &iaasInstance.InternalId,
				DBSnapshotIdentifier:        &restoreFromSnapshot,
				DBSubnetGroupName:           &conf.RDSSubnetGrp,
				EnableCloudwatchLogsExports: logs,
				Engine:                      &engine,
				MultiAZ:                     &multiAZ,
				PubliclyAccessible:          &publiclyAccessible,
				StorageType:                 &storageType,
				Tags:                        getTagsForServiceInstanceRDS(serviceInstance),
				VpcSecurityGroupIds:         vpcSecGrpIds,
			}
			// do the actual AWS call to create the DB restoring from snapshot
			dbInstanceRestoreOutput, err = conf.RDSClient.RestoreDBInstanceFromDBSnapshot(input)
		}
	} else {
		input := &rds.CreateDBInstanceInput{
			AllocatedStorage:            &allocatedStorageGB,
			AutoMinorVersionUpgrade:     &autoMinorVersionUpgrade,
			BackupRetentionPeriod:       &retentionDays,
			CopyTagsToSnapshot:          &copyTagsToSnapshot,
			DBInstanceClass:             &dbInstanceClass,
			DBInstanceIdentifier:        &iaasInstance.InternalId,
			DBName:                      &dbName,
			DBSubnetGroupName:           &conf.RDSSubnetGrp,
			EnableCloudwatchLogsExports: logs,
			Engine:                      &engine,
			MasterUserPassword:          &iaasInstance.ServicePassword,
			MasterUsername:              &iaasInstance.ServiceUser,
			MultiAZ:                     &multiAZ,
			PubliclyAccessible:          &publiclyAccessible,
			StorageEncrypted:            &storageEncrypted,
			StorageType:                 &storageType,
			Tags:                        getTagsForServiceInstanceRDS(serviceInstance),
			VpcSecurityGroupIds:         vpcSecGrpIds,
		}

		// do the actual AWS call to create the DB
		dbInstanceCreateOutput, err = conf.RDSClient.CreateDBInstance(input)
	}
	if err != nil {
		LogAwsError(err)
		db.DeleteServiceInstanceByServiceInstanceId(serviceInstance.InstanceId)
		db.UpdateStatusIaaSInstance(iaasInstance, db.StatusCreateFailed, strings.ReplaceAll(err.Error(), "\n", ""))
	} else {
		msg := fmt.Sprintf("RDS Database %s is being created/restored", iaasInstance.InternalId)
		fmt.Println(msg)
		if conf.Debug {
			if parameters.RestoreFromSnapshot != "" {
				fmt.Println(dbInstanceRestoreOutput)
			} else {
				fmt.Println(dbInstanceCreateOutput)
			}
		}
		db.UpdateStatusServiceInstance(serviceInstance, db.StatusInProgress)
		db.UpdateStatusIaaSInstance(iaasInstance, db.StatusCreateInProgress, msg)
		StartPollForStatusRDSDB(iaasInstance)
	}
	return err
}

func snapshotExistsAndAuthorized(snapshotIdentifier string, serviceInstance db.ServiceInstance) bool {
	var err error
	input := rds.DescribeDBSnapshotsInput{DBSnapshotIdentifier: &snapshotIdentifier}
	output := &rds.DescribeDBSnapshotsOutput{}
	if output, err = conf.RDSClient.DescribeDBSnapshots(&input); err != nil {
		fmt.Printf("failed to describe rds snapshot with identifier %s: %s", snapshotIdentifier, err)
		return false
	} else {
		if len(output.DBSnapshots) == 1 {
			//  check if given snapshot has the tags that identify if it was requested from the same org, space and instance!
			var orgNameTagFound, spaceNameTagFound bool
			for _, tag := range output.DBSnapshots[0].TagList {
				if *tag.Key == "OrganizationName" && *tag.Value == serviceInstance.OrganizationName {
					orgNameTagFound = true
				}
				if *tag.Key == "SpaceName" && *tag.Value == serviceInstance.SpaceName {
					spaceNameTagFound = true
				}
			}
			if orgNameTagFound && spaceNameTagFound {
				return true
			} else {
				fmt.Printf("A restore from snapshot %s was requested, but the snapshot is missing one or more tags (OrganizationName:%s, SpaceName:%s)\n", snapshotIdentifier, serviceInstance.OrganizationName, serviceInstance.SpaceName)
				return false
			}
		} else {
			fmt.Printf("we found more than 1 (%d) snapshots for snapshot identifier %s", len(output.DBSnapshots), snapshotIdentifier)
			return false
		}
	}
}

func SubmitDeletionRDSDB(iaasInstance db.IaaSInstance, serviceInstance db.ServiceInstance) error {
	var err error
	if _, _, err = processParameters(serviceInstance); err != nil {
		return err
	}
	fmt.Printf("deleting database %s...\n", iaasInstance.InternalId)
	db.UpdateStatusServiceInstance(serviceInstance, db.StatusInProgress)
	db.UpdateStatusIaaSInstance(iaasInstance, db.StatusDeleteInProgress, "delete in progress")

	var snapshotIdentifier = ""
	if !skipFinalSnapshot {
		snapshotIdentifier = iaasInstance.InternalId
	}

	// actual delete
	fmt.Printf("delete parameters, skipFinalSnapshot:%t, snapshotIdentifier:%s\n", skipFinalSnapshot, snapshotIdentifier)
	_, err = conf.RDSClient.DeleteDBInstance(&rds.DeleteDBInstanceInput{DBInstanceIdentifier: &iaasInstance.InternalId, DeleteAutomatedBackups: &keepBackups, SkipFinalSnapshot: &skipFinalSnapshot, FinalDBSnapshotIdentifier: &snapshotIdentifier})

	if err != nil {
		fmt.Printf("failed to delete database instance %s, err: %s\n", iaasInstance.InternalId, err)
		db.UpdateStatusServiceInstance(serviceInstance, db.StatusFailed)
		db.UpdateStatusIaaSInstance(iaasInstance, db.StatusDeleteFailed, err.Error())
	}
	StartPollForStatusRDSDB(iaasInstance)
	return err
}

func StartPollForStatusRDSDB(iaasInstance db.IaaSInstance) {
	serviceInstance := db.GetServiceInstanceByEnvAndIaaSId(conf.CfEnv, iaasInstance.Id)
	go func() {
		channel := time.Tick(20 * time.Second)
		for range channel {
			// start polling for the result
			output, err := conf.RDSClient.DescribeDBInstances(&rds.DescribeDBInstancesInput{DBInstanceIdentifier: &iaasInstance.InternalId})
			if err != nil {
				aerr, _ := err.(awserr.Error)
				if aerr.Code() == rds.ErrCodeDBInstanceNotFoundFault {
					// this should only happen when a database deletion ended
					fmt.Printf("RDS DB instance %s is gone, %s\n", iaasInstance.InternalId, aerr.Message())
					db.DeleteServiceInstanceByServiceInstanceId(serviceInstance.InstanceId)
					db.UpdateStatusIaaSInstance(iaasInstance, db.StatusDeleteSucceeded, fmt.Sprintf("DB instance %s is gone", iaasInstance.InternalId))
					err = deleteIAMRoleIfExists(&iaasInstance, &serviceInstance)
					if err != nil {
						fmt.Printf("failed to delete the IAM role for %s: %s", iaasInstance.InternalId, err)
						iaasInstance.LastMessage = fmt.Sprintf("DB instance %s successfully deleted, IAM role delete failed (%s)", iaasInstance.InternalId, err)
						_ = db.UpdateStatusServiceInstanceForIaaSId(serviceInstance.IaaSInstanceId, db.StatusSucceeded)
						_ = db.UpdateIaaSInstance(iaasInstance)
					}
				} else {
					msg := fmt.Sprintf("failed to describe DB instance %s, code : %s error: %s", iaasInstance.InternalId, aerr.Code(), err)
					fmt.Println(msg)
					db.UpdateStatusServiceInstance(serviceInstance, db.StatusFailed)
					db.UpdateStatusIaaSInstance(iaasInstance, db.StatusNotFound, msg)
				}
				break
			} else {
				dbInstances := output.DBInstances
				if len(dbInstances) > 0 {
					dbStatus := dbInstances[0].DBInstanceStatus
					msg := fmt.Sprintf("rds db %s : %s", *dbInstances[0].DBInstanceIdentifier, *dbStatus)
					fmt.Println(msg)
					if *dbStatus == "available" {
						if conf.Debug {
							msg = fmt.Sprintf("RDS DB instance %s successfully created:\n%v", iaasInstance.InternalId, dbInstances[0])
						} else {
							msg = fmt.Sprintf("RDS DB instance %s successfully created", iaasInstance.InternalId)
						}
						fmt.Println(msg)
						schema := schemas[*dbInstances[0].Engine]
						iaasInstance.ServiceUrl = fmt.Sprintf(schema, iaasInstance.ServiceUser, iaasInstance.ServicePassword, *dbInstances[0].Endpoint.Address, *dbInstances[0].Endpoint.Port, *dbInstances[0].DBName)
						iaasInstance.Status = db.StatusCreateSucceeded
						iaasInstance.LastStatusUpdate = time.Now()
						iaasInstance.LastMessage = fmt.Sprintf("RDS DB instance %s successfully created", iaasInstance.InternalId)
						_ = db.UpdateStatusServiceInstanceForIaaSId(serviceInstance.IaaSInstanceId, db.StatusSucceeded)
						_ = db.UpdateIaaSInstance(iaasInstance)
						// update the database master user/password (this is actually only required during a RestoreFromSnaphot, but it is easier to do it for all cases)
						input := &rds.ModifyDBInstanceInput{DBInstanceIdentifier: dbInstances[0].DBInstanceIdentifier, MasterUserPassword: &iaasInstance.ServicePassword}
						if _, err = conf.RDSClient.ModifyDBInstance(input); err != nil {
							fmt.Printf("failed to modify master password for rds db %s: %s\n", *dbInstances[0].DBInstanceIdentifier, err)
						}

						err = createIAMRoleIfNotExists(&iaasInstance, &serviceInstance)
						if err != nil {
							fmt.Printf("failed to create the IAM role for %s: %s", iaasInstance.InternalId, err)
							iaasInstance.LastMessage = fmt.Sprintf("RDS DB instance %s successfully created, IAM role creation failed (%s)", iaasInstance.InternalId, err)
							_ = db.UpdateStatusServiceInstanceForIaaSId(serviceInstance.IaaSInstanceId, db.StatusSucceeded)
							_ = db.UpdateIaaSInstance(iaasInstance)
						}
						break
					}
				}
			}
		}
	}()
}

func getTagsForServiceInstanceRDS(serviceInstance db.ServiceInstance) []*rds.Tag {
	var (
		tagList             []*rds.Tag
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
	tagList = append(tagList, &rds.Tag{Key: &serviceInstanceId, Value: &serviceInstance.ServiceId})
	tagList = append(tagList, &rds.Tag{Key: &serviceInstanceName, Value: &serviceInstance.InstanceName})
	tagList = append(tagList, &rds.Tag{Key: &spaceName, Value: &serviceInstance.SpaceName})
	tagList = append(tagList, &rds.Tag{Key: &orgName, Value: &serviceInstance.OrganizationName})
	tagList = append(tagList, &rds.Tag{Key: &env, Value: &serviceInstance.Env})
	tagList = append(tagList, &rds.Tag{Key: &createdBy, Value: &mfsb})
	plan := util.GetPlan(serviceInstance.ServiceId, serviceInstance.PlanId)
	tagList = append(tagList, &rds.Tag{Key: &planName, Value: &plan.Name})
	service := util.GetServiceById(serviceInstance.ServiceId)
	tagList = append(tagList, &rds.Tag{Key: &serviceName, Value: &service.Name})
	return tagList
}
