package aws

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/service/docdb"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/rabobank/mfsb/conf"
	"github.com/rabobank/mfsb/db"
	"github.com/rabobank/mfsb/util"
)

const (
	RetentionDaysDOCDBDefault = 7
)

var userNameDOCDB = "docdbadmin"
var DOCDBEngineDefault = "docdb"
var deleteProtection = false

func SubmitProvisionDOCDB(iaasInstance db.IaaSInstance, serviceInstance db.ServiceInstance) error {
	_, parameters, err := processParameters(serviceInstance)
	if err != nil {
		return err
	}
	retentionDays := parameters.RetentionDays
	numInstancesDOCDB := parameters.NumDBInstances
	if numInstancesDOCDB == 0 {
		numInstancesDOCDB = NumInstancesDOCDBDefault
	}
	// set the proper default for documentdb
	if retentionDays == 0 {
		retentionDays = RetentionDaysDOCDBDefault
	}
	vpcSecGrpIds = append(vpcSecGrpIds, &conf.DOCDBSecGrpId)
	iaasInstance.ServiceUser = userNameDOCDB
	iaasInstance.ServicePassword = util.SafeSubstring(fmt.Sprintf("pw%s", util.GenerateGUID()), 40)
	plan := util.GetPlan(serviceInstance.ServiceId, serviceInstance.PlanId)
	dbInstanceClass := conf.DOCDBClasses[plan.Name]
	if dbInstanceClass == "" {
		msg := fmt.Sprintf("could not find database instance class for plan %s", plan.Name)
		fmt.Println(msg)
		db.UpdateStatusIaaSInstance(iaasInstance, db.StatusCreateFailed, fmt.Sprintf("Database creation failed, error: %s", msg))
		serviceInstance.Status = db.StatusFailed
		_ = db.UpdateServiceInstance(serviceInstance)
		return errors.New(msg)
	}

	// first create the cluster (and after we create the instance(s))
	createClusterInput := &docdb.CreateDBClusterInput{
		AvailabilityZones:           conf.AZS,
		BackupRetentionPeriod:       &retentionDays,
		DBClusterIdentifier:         &iaasInstance.InternalId,
		DBSubnetGroupName:           &conf.DOCDBSubnetGrp,
		DeletionProtection:          &deleteProtection,
		EnableCloudwatchLogsExports: docdbLogs,
		Engine:                      &DOCDBEngineDefault,
		MasterUserPassword:          &iaasInstance.ServicePassword,
		MasterUsername:              &iaasInstance.ServiceUser,
		StorageEncrypted:            &storageEncrypted,
		Tags:                        getTagsForServiceInstanceDOCDB(serviceInstance),
		VpcSecurityGroupIds:         vpcSecGrpIds,
	}

	createDBClusterOutput, err := conf.DOCDBClient.CreateDBCluster(createClusterInput)

	if err != nil {
		msg := fmt.Sprintf("could not create cluster %s: %s", serviceInstance.InstanceName, err)
		fmt.Println(msg)
		db.UpdateStatusIaaSInstance(iaasInstance, db.StatusCreateFailed, fmt.Sprintf("Database creation failed, error: %s", msg))
		serviceInstance.Status = db.StatusFailed
		_ = db.UpdateServiceInstance(serviceInstance)
		return errors.New(msg)
	}
	fmt.Printf("docdb cluster %s created\n", *createDBClusterOutput.DBCluster.DBClusterIdentifier)

	for ix := 0; ix < int(numInstancesDOCDB); ix++ {
		az := *util.GetNextAZ()
		instanceIdentifier := fmt.Sprintf("%s-%s", iaasInstance.InternalId, az)
		fmt.Printf("creating docdb instance %s for cluster %s...\n", instanceIdentifier, *createDBClusterOutput.DBCluster.DBClusterIdentifier)
		createDBInstanceInput := &docdb.CreateDBInstanceInput{
			AutoMinorVersionUpgrade: &autoMinorVersionUpgrade,
			AvailabilityZone:        &az,
			DBClusterIdentifier:     &iaasInstance.InternalId,
			DBInstanceClass:         &dbInstanceClass,
			DBInstanceIdentifier:    &instanceIdentifier,
			Engine:                  &DOCDBEngineDefault,
			Tags:                    getTagsForServiceInstanceDOCDB(serviceInstance),
		}

		// do the actual AWS call to create the DB
		createDBInstanceOutput, err := conf.DOCDBClient.CreateDBInstance(createDBInstanceInput)

		if err != nil {
			LogAwsError(err)
			db.UpdateStatusServiceInstance(serviceInstance, db.StatusFailed)
			db.UpdateStatusIaaSInstance(iaasInstance, db.StatusCreateFailed, strings.ReplaceAll(err.Error(), "\n", ""))
		} else {
			msg := fmt.Sprintf("DOCDB Database instance %s is being created", iaasInstance.InternalId)
			fmt.Println(msg)
			if conf.Debug {
				fmt.Println(createDBInstanceOutput)
			}
			db.UpdateStatusServiceInstance(serviceInstance, db.StatusInProgress)
			db.UpdateStatusIaaSInstance(iaasInstance, db.StatusCreateInProgress, msg)
			StartPollForStatusDOCDB(iaasInstance)
		}
	}
	return err
}

func SubmitDeletionDOCDB(iaasInstance db.IaaSInstance, serviceInstance db.ServiceInstance) error {
	var err error
	if _, _, err = processParameters(serviceInstance); err != nil {
		return err
	}
	fmt.Printf("deleting docdb cluster %s...\n", iaasInstance.InternalId)
	db.UpdateStatusServiceInstance(serviceInstance, db.StatusInProgress)
	db.UpdateStatusIaaSInstance(iaasInstance, db.StatusDeleteInProgress, "delete in progress")

	var snapshotIdentifier = ""
	if !skipFinalSnapshot {
		snapshotIdentifier = iaasInstance.InternalId
	}

	// actual delete
	fmt.Printf("deleting docdb cluster with parameters skipFinalSnapshot:%t and snapshotIdentifier:%s\n", skipFinalSnapshot, snapshotIdentifier)

	describeClusterOutput, err := conf.DOCDBClient.DescribeDBClusters(&docdb.DescribeDBClustersInput{DBClusterIdentifier: &iaasInstance.InternalId})
	if err != nil {
		msg := fmt.Sprintf("could not describe cluster %s: %s", iaasInstance.InternalId, err)
		fmt.Println(msg)
		return errors.New(msg)
	}
	for ix, instance := range describeClusterOutput.DBClusters[0].DBClusterMembers {
		fmt.Printf("deleting docdb instance (%d) %s\n", ix, *instance.DBInstanceIdentifier)
		_, err = conf.DOCDBClient.DeleteDBInstance(&docdb.DeleteDBInstanceInput{DBInstanceIdentifier: instance.DBInstanceIdentifier})
		if err != nil {
			msg := fmt.Sprintf("failed to delete docdb instance (%d) %s, err: %s\n", ix, iaasInstance.InternalId, err)
			db.UpdateStatusServiceInstance(serviceInstance, db.StatusFailed)
			db.UpdateStatusIaaSInstance(iaasInstance, db.StatusDeleteFailed, err.Error())
			fmt.Println(msg)
			return errors.New(msg)
		}
	}

	fmt.Printf("starting to delete docdb cluster %s...\n", iaasInstance.InternalId)
	_, err = conf.DOCDBClient.DeleteDBCluster(&docdb.DeleteDBClusterInput{DBClusterIdentifier: &iaasInstance.InternalId, FinalDBSnapshotIdentifier: &snapshotIdentifier, SkipFinalSnapshot: &skipFinalSnapshot})
	if err != nil {
		fmt.Printf("failed to delete docdb cluster %s, err: %s\n", iaasInstance.InternalId, err)
		db.UpdateStatusServiceInstance(serviceInstance, db.StatusFailed)
		db.UpdateStatusIaaSInstance(iaasInstance, db.StatusDeleteFailed, err.Error())
	}

	StartPollForStatusDOCDB(iaasInstance)
	return err
}

func StartPollForStatusDOCDB(iaasInstance db.IaaSInstance) {
	serviceInstance := db.GetServiceInstanceByEnvAndIaaSId(conf.CfEnv, iaasInstance.Id)
	go func() {
		channel := time.Tick(20 * time.Second)
		for range channel {
			// start polling for the result
			output, err := conf.DOCDBClient.DescribeDBClusters(&docdb.DescribeDBClustersInput{DBClusterIdentifier: &iaasInstance.InternalId})
			if err != nil {
				aerr, _ := err.(awserr.Error)
				if aerr.Code() == docdb.ErrCodeDBClusterNotFoundFault {
					// this should only happen when a database cluster deletion ended
					fmt.Printf("docdb cluster %s is gone, %s\n", iaasInstance.InternalId, aerr.Message())
					db.DeleteServiceInstanceByServiceInstanceId(serviceInstance.InstanceId)
					db.UpdateStatusIaaSInstance(iaasInstance, db.StatusDeleteSucceeded, fmt.Sprintf("docdb cluster %s is gone", iaasInstance.InternalId))
					err = deleteIAMRoleIfExists(&iaasInstance, &serviceInstance)
					if err != nil {
						fmt.Printf("failed to delete the IAM role for %s: %s", iaasInstance.InternalId, err)
						iaasInstance.LastMessage = fmt.Sprintf("docdb cluster %s successfully deleted, IAM role delete failed (%s)", iaasInstance.InternalId, err)
						_ = db.UpdateStatusServiceInstanceForIaaSId(serviceInstance.IaaSInstanceId, db.StatusSucceeded)
						_ = db.UpdateIaaSInstance(iaasInstance)
					}
				} else {
					msg := fmt.Sprintf("failed to describe docdb cluster %s, code : %s error: %s", iaasInstance.InternalId, aerr.Code(), err)
					fmt.Println(msg)
					db.UpdateStatusServiceInstance(serviceInstance, db.StatusFailed)
					db.UpdateStatusIaaSInstance(iaasInstance, db.StatusNotFound, msg)
				}
				break
			} else {
				dbClusters := output.DBClusters
				if len(dbClusters) > 0 {
					dbCluster := dbClusters[0]
					dbStatus := dbCluster.Status
					msg := fmt.Sprintf("docdb cluster %s : %s", *dbClusters[0].DBClusterIdentifier, *dbStatus)
					fmt.Println(msg)
					if *dbStatus == "available" {
						schema := schemas[*dbClusters[0].Engine]
						// DB cluster is ready (but DB instances not yet)
						iaasInstance.ServiceUrl = fmt.Sprintf(schema, iaasInstance.ServiceUser, iaasInstance.ServicePassword, *dbClusters[0].Endpoint, *dbClusters[0].Port)
						iaasInstance.Status = db.StatusCreateInProgress
						iaasInstance.LastStatusUpdate = time.Now()
						iaasInstance.LastMessage = fmt.Sprintf("documentdb cluster %s created, db instance(s) creation in progress", iaasInstance.InternalId)
						_ = db.UpdateStatusServiceInstanceForIaaSId(serviceInstance.IaaSInstanceId, db.StatusInProgress)
						_ = db.UpdateIaaSInstance(iaasInstance)

						// now check the DB instances
						allInstancesAvailable := true
						for _, member := range dbCluster.DBClusterMembers {
							instances, err := conf.DOCDBClient.DescribeDBInstances(&docdb.DescribeDBInstancesInput{DBInstanceIdentifier: member.DBInstanceIdentifier})
							if err != nil {
								fmt.Printf("failed describing docdb instances  %s: %s", *member.DBInstanceIdentifier, err)
							} else {
								status := instances.DBInstances[0].DBInstanceStatus
								fmt.Printf("docdb instance %s : %v\n", *instances.DBInstances[0].DBInstanceIdentifier, *status)
								if *status != "available" {
									allInstancesAvailable = false
								}
							}
						}
						if allInstancesAvailable {
							iaasInstance.Status = db.StatusCreateSucceeded
							iaasInstance.LastStatusUpdate = time.Now()
							iaasInstance.LastMessage = fmt.Sprintf("docdb cluster %s successfully created", iaasInstance.InternalId)
							_ = db.UpdateStatusServiceInstanceForIaaSId(serviceInstance.IaaSInstanceId, db.StatusSucceeded)
							_ = db.UpdateIaaSInstance(iaasInstance)
							err = createIAMRoleIfNotExists(&iaasInstance, &serviceInstance)
							if err != nil {
								fmt.Printf("failed to create the IAM role for %s: %s", iaasInstance.InternalId, err)
								iaasInstance.LastMessage = fmt.Sprintf("docdb cluster %s successfully created, IAM role creation failed (%s)", iaasInstance.InternalId, err)
								_ = db.UpdateStatusServiceInstanceForIaaSId(serviceInstance.IaaSInstanceId, db.StatusSucceeded)
								_ = db.UpdateIaaSInstance(iaasInstance)
							}
							break
						}
					}
				}
			}
		}
	}()
}

func getTagsForServiceInstanceDOCDB(serviceInstance db.ServiceInstance) []*docdb.Tag {
	var (
		tagList             []*docdb.Tag
		serviceInstanceId   = "ServiceInstanceId"
		serviceInstanceName = "ServiceInstanceName"
		spaceName           = "SpaceName"
		orgName             = "OrganizationName"
		env                 = "CreatedFromEnv"
		createdBy           = "CreatedBy"
		serviceName         = "ServiceName"
		planName            = "PlanName"
		mfsb                = "mfsb"
	)
	tagList = append(tagList, &docdb.Tag{Key: &serviceInstanceId, Value: &serviceInstance.ServiceId})
	tagList = append(tagList, &docdb.Tag{Key: &serviceInstanceName, Value: &serviceInstance.InstanceName})
	tagList = append(tagList, &docdb.Tag{Key: &spaceName, Value: &serviceInstance.SpaceName})
	tagList = append(tagList, &docdb.Tag{Key: &orgName, Value: &serviceInstance.OrganizationName})
	tagList = append(tagList, &docdb.Tag{Key: &env, Value: &serviceInstance.Env})
	tagList = append(tagList, &docdb.Tag{Key: &createdBy, Value: &mfsb})
	plan := util.GetPlan(serviceInstance.ServiceId, serviceInstance.PlanId)
	tagList = append(tagList, &docdb.Tag{Key: &planName, Value: &plan.Name})
	service := util.GetServiceById(serviceInstance.ServiceId)
	tagList = append(tagList, &docdb.Tag{Key: &serviceName, Value: &service.Name})
	return tagList
}
