package db

import (
	"database/sql"
	"fmt"
)

const (
	StatusInProgress = "in progress"
	StatusFailed     = "failed"
	StatusSucceeded  = "succeeded"
)

type ServiceInstance struct {
	Id               int64
	ServiceId        string
	PlanId           string
	Parameters       string
	InstanceId       string
	Env              string
	OrganizationName string
	SpaceName        string
	InstanceName     string
	IaaSInstanceId   int64
	Status           string
}

func (si ServiceInstance) String() string {
	return fmt.Sprintf("ServiceInstance: Id:%d, ServiceId:%s, PlanId:%s, InstanceId:%s, Env:%s, OrganizationName:%s, SpaceName:%s, InstanceName:%s, IaaSInstanceId:%d, Status:%s", si.Id, si.ServiceId, si.PlanId, si.InstanceId, si.Env, si.OrganizationName, si.SpaceName, si.InstanceName, si.IaaSInstanceId, si.Status)
}

func InsertServiceInstance(serviceInstance ServiceInstance) (int64, error) {
	var err error
	var Id int64
	db := GetDB()
	defer db.Close()
	result, err := db.Exec("insert into service_instance(service_id, instance_id, plan_id, parameters, env, organization_name, space_name, instance_name, iaas_instance_id, status) values(?,?,?,?,?,?,?,?,?,?)",
		serviceInstance.ServiceId, serviceInstance.InstanceId, serviceInstance.PlanId, serviceInstance.Parameters, serviceInstance.Env, serviceInstance.OrganizationName, serviceInstance.SpaceName, serviceInstance.InstanceName, serviceInstance.IaaSInstanceId, serviceInstance.Status)
	if err != nil {
		fmt.Printf("failed to insert %v, error: %s\n", serviceInstance, err)
	} else {
		Id, _ = result.LastInsertId()
		serviceInstance.Id = Id
		fmt.Printf("inserted %v\n", serviceInstance)
	}
	return Id, err
}

func UpdateServiceInstance(serviceInstance ServiceInstance) error {
	var err error
	db := GetDB()
	defer db.Close()
	_, err = db.Exec("update service_instance set service_id=?, instance_id=?, plan_id=?, parameters=?, env=?, organization_name=?, space_name=?, instance_name=?, iaas_instance_id=?, status=? where id=?",
		serviceInstance.ServiceId, serviceInstance.InstanceId, serviceInstance.PlanId, serviceInstance.Parameters, serviceInstance.Env, serviceInstance.OrganizationName, serviceInstance.SpaceName, serviceInstance.InstanceName, serviceInstance.IaaSInstanceId, serviceInstance.Status, serviceInstance.Id)
	if err != nil {
		fmt.Printf("failed to update %v, error: %s\n", serviceInstance, err)
	}
	return err
}

func UpdateStatusServiceInstanceForIaaSId(iaasInstanceId int64, status string) error {
	var err error
	db := GetDB()
	defer db.Close()
	_, err = db.Exec("update service_instance set status=? where iaas_instance_id=?", status, iaasInstanceId)
	if err != nil {
		fmt.Printf("failed to update status to %s, for IaaSInstanceId %d error: %s\n", status, iaasInstanceId, err)
	}
	return err
}

func UpdateStatusServiceInstance(serviceInstance ServiceInstance, status string) {
	serviceInstance.Status = status
	_ = UpdateServiceInstance(serviceInstance)
}

func GetServiceInstances(id int64) []ServiceInstance {
	var err error
	result := make([]ServiceInstance, 0)
	db := GetDB()
	defer db.Close()
	var rows *sql.Rows
	if id == 0 {
		rows, err = db.Query("select Id, service_id, instance_id, plan_id, parameters, env, organization_name, space_name, instance_name, iaas_instance_id, status from service_instance")
	} else {
		rows, err = db.Query("select Id, service_id, instance_id, plan_id, parameters, env, organization_name, space_name, instance_name, iaas_instance_id, status from service_instance where id=?", id)
	}
	if err != nil {
		fmt.Printf("failed to query the service_instances, err: %s\n", err)
	} else {
		result = getServiceInstances(rows)
	}
	return result
}

func GetServiceInstanceByInstanceId(id string) ServiceInstance {
	var err error
	var serviceInstance ServiceInstance
	result := make([]ServiceInstance, 0)
	db := GetDB()
	defer db.Close()
	var rows *sql.Rows
	rows, err = db.Query("select Id, service_id, instance_id, plan_id, parameters, env, organization_name, space_name, instance_name, iaas_instance_id, status from service_instance where instance_id=?", id)
	if err != nil {
		fmt.Printf("failed to query the service_instances for instance_id %s, err: %s\n", id, err)
	} else {
		result = getServiceInstances(rows)
	}
	if len(result) != 1 {
		fmt.Printf("no serviceinstance found for id %s\n", id)
		return serviceInstance
	}
	return result[0]
}

func GetServiceInstanceByEnvAndIaaSId(env string, iaasId int64) ServiceInstance {
	var err error
	var serviceInstance ServiceInstance
	result := make([]ServiceInstance, 0)
	db := GetDB()
	defer db.Close()
	var rows *sql.Rows
	rows, err = db.Query("select Id, service_id, instance_id, plan_id, parameters, env, organization_name, space_name, instance_name, iaas_instance_id, status from service_instance where env=? and iaas_instance_id=?", env, iaasId)
	if err != nil {
		fmt.Printf("failed to query the service_instances for env %s and iaasId %d, err: %s\n", env, iaasId, err)
	} else {
		result = getServiceInstances(rows)
	}
	if len(result) != 1 {
		fmt.Printf("no serviceinstance found for env %s and iaasId %d\n", env, iaasId)
		return serviceInstance
	}
	if len(result) == 1 {
		return result[0]
	}
	return ServiceInstance{}
}

// GetServicesInstanceByNameAndStatus We return instances for all cf envs.
func GetServicesInstanceByNameAndStatus(orgName, spaceName, instanceName, status string) []ServiceInstance {
	var err error
	result := make([]ServiceInstance, 0)
	db := GetDB()
	defer db.Close()
	var rows *sql.Rows
	rows, err = db.Query("select Id, service_id, instance_id, plan_id, parameters, env, organization_name, space_name, instance_name, iaas_instance_id, status from service_instance where status=? and organization_name=? and space_name=? and instance_name=?", status, orgName, spaceName, instanceName)
	if err != nil {
		fmt.Printf("failed to query the service_instances, err: %s\n", err)
	} else {
		result = getServiceInstances(rows)
	}
	return result
}

// GetServicesInstanceByNameAndIaaSStatus We return instances for all cf envs.
func GetServicesInstanceByNameAndIaaSStatus(orgName, spaceName, instanceName, status string) []ServiceInstance {
	var err error
	result := make([]ServiceInstance, 0)
	db := GetDB()
	defer db.Close()
	var rows *sql.Rows
	rows, err = db.Query("select s.Id, s.service_id, s.instance_id, s.plan_id, s.parameters, s.env, s.organization_name, s.space_name, s.instance_name, s.iaas_instance_id, s.status from service_instance s, iaas_instance i where s.iaas_instance_id=i.id and i.status=? and s.organization_name=? and s.space_name=? and s.instance_name=?", status, orgName, spaceName, instanceName)
	if err != nil {
		fmt.Printf("failed to query the service_instances, err: %s\n", err)
	} else {
		result = getServiceInstances(rows)
	}
	return result
}

func getServiceInstances(rows *sql.Rows) []ServiceInstance {
	result := make([]ServiceInstance, 0)
	if rows != nil {
		defer rows.Close()
		var Id, iaasInstanceId int64
		var serviceId, instanceId, planId, parameters, env, organizationName, spaceName, instanceName, status string
		for rows.Next() {
			err := rows.Scan(&Id, &serviceId, &instanceId, &planId, &parameters, &env, &organizationName, &spaceName, &instanceName, &iaasInstanceId, &status)
			if err != nil {
				fmt.Printf("failed to scan the service_instance row, error:%s\n", err)
			} else {
				result = append(result, ServiceInstance{
					Id:               Id,
					ServiceId:        serviceId,
					PlanId:           planId,
					Parameters:       parameters,
					InstanceId:       instanceId,
					Env:              env,
					OrganizationName: organizationName,
					SpaceName:        spaceName,
					InstanceName:     instanceName,
					IaaSInstanceId:   iaasInstanceId,
					Status:           status,
				})
			}
		}
	}
	return result
}

func DeleteServiceInstanceByServiceInstanceId(instanceId string) {
	var err error
	db := GetDB()
	defer db.Close()
	_, err = db.Exec("delete from service_instance where instance_id=?", instanceId)
	if err != nil {
		fmt.Printf("failed to delete ServiceInstance for ServiceInstanceId %s, error: %s\n", instanceId, err)
	}
}
