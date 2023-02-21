package controllers

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/rabobank/mfsb/aws"
	"github.com/rabobank/mfsb/conf"
	"github.com/rabobank/mfsb/db"
	"github.com/rabobank/mfsb/model"
	"github.com/rabobank/mfsb/util"
	"net/http"
	"strings"
	"time"
)

func Catalog(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("get service broker catalog from %s...\n", r.RemoteAddr)
	util.WriteHttpResponse(w, http.StatusOK, conf.Catalog)
}

func GetServiceInstance(w http.ResponseWriter, r *http.Request) {
	serviceInstanceId := mux.Vars(r)["service_instance_guid"]
	fmt.Printf("get service instance for %s...\n", serviceInstanceId)
	serviceInstance := db.GetServiceInstanceByInstanceId(serviceInstanceId)
	if serviceInstance.InstanceName == "" {
		util.WriteHttpResponse(w, http.StatusNotFound, fmt.Sprintf("service instance with guid %s not found", serviceInstanceId))
	} else {
		iaasInstance := db.GetIaaSInstances(serviceInstance.IaaSInstanceId)[0]
		lastOperation := &model.LastOperation{
			State:       serviceInstance.Status,
			Description: iaasInstance.LastMessage,
		}
		response := model.CreateServiceInstanceResponse{
			ServiceId:     serviceInstance.ServiceId,
			PlanId:        serviceInstance.PlanId,
			DashboardUrl:  "gibt_s_nicht",
			LastOperation: lastOperation,
		}
		util.WriteHttpResponse(w, http.StatusOK, response)
	}
}

func GetServiceInstanceLastOperation(w http.ResponseWriter, r *http.Request) {
	serviceInstanceId := mux.Vars(r)["service_instance_guid"]
	fmt.Printf("get service instance LastOperation for %s...\n", serviceInstanceId)
	serviceInstance := db.GetServiceInstanceByInstanceId(serviceInstanceId)
	if serviceInstance.InstanceName == "" {
		response := &model.LastOperation{
			State:       db.StatusSucceeded,
			Description: fmt.Sprintf("service instance with guid %s not found", serviceInstanceId),
		}
		util.WriteHttpResponse(w, http.StatusOK, response)
	} else {
		iaasInstance := db.GetIaaSInstances(serviceInstance.IaaSInstanceId)[0]
		response := &model.LastOperation{
			State:       serviceInstance.Status,
			Description: iaasInstance.LastMessage,
		}
		util.WriteHttpResponse(w, http.StatusOK, response)
	}
}

func CreateServiceInstance(w http.ResponseWriter, r *http.Request) {
	serviceInstanceId := mux.Vars(r)["service_instance_guid"]
	fmt.Printf("create service instance for %s...\n", serviceInstanceId)
	var err error
	var serviceInstance model.ServiceInstance
	err = util.ProvisionObjectFromRequest(r, &serviceInstance)
	if err != nil {
		util.WriteHttpResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	// read the supported parameters (they have to be stored in the db)
	parmsBA, err := json.Marshal(serviceInstance.Parameters)
	if err != nil {
		util.WriteHttpResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	parameters := string(parmsBA)
	fmt.Printf("got parameters: %s\n", parameters)
	if len(parameters) > 2048 {
		util.WriteHttpResponse(w, http.StatusBadRequest, "The given parameter string is more than 2048 chars")
		return
	}
	serviceName := util.GetServiceById(serviceInstance.ServiceId).Name
	if !strings.HasPrefix(serviceName, "rds-service") && !strings.HasPrefix(serviceName, "documentdb-service") {
		util.WriteHttpResponse(w, http.StatusBadRequest, fmt.Sprintf("service %s is not supported", serviceName))
		return
	}
	var lastOperation *model.LastOperation
	serviceInstancesInProgress := db.GetServicesInstanceByNameAndStatus(serviceInstance.Context.OrgName, serviceInstance.Context.SpaceName, serviceInstance.Context.InstanceName, db.StatusInProgress)
	serviceInstancesCreated := db.GetServicesInstanceByNameAndIaaSStatus(serviceInstance.Context.OrgName, serviceInstance.Context.SpaceName, serviceInstance.Context.InstanceName, db.StatusCreateSucceeded)
	if (len(serviceInstancesInProgress) > 0 && serviceInstancesInProgress[0].PlanId != serviceInstance.PlanId) || (len(serviceInstancesCreated) > 0 && serviceInstancesCreated[0].PlanId != serviceInstance.PlanId) {
		util.WriteHttpResponse(w, http.StatusBadRequest, fmt.Sprintf("requested plan Id (%s) is not equal to plan Id of already existing service", serviceInstance.PlanId))
		return
	}
	if len(serviceInstancesInProgress) == 0 {
		// there is nothing in progress for the requested database
		if len(serviceInstancesCreated) != 0 {
			// the database was already successfully created from another foundation
			serviceInstanceDB := db.ServiceInstance{
				ServiceId:        serviceInstance.ServiceId,
				InstanceId:       serviceInstanceId,
				PlanId:           serviceInstance.PlanId,
				Parameters:       parameters,
				Env:              conf.CfEnv,
				OrganizationName: serviceInstance.Context.OrgName,
				SpaceName:        serviceInstance.Context.SpaceName,
				InstanceName:     serviceInstance.Context.InstanceName,
				IaaSInstanceId:   serviceInstancesCreated[0].IaaSInstanceId,
				Status:           db.StatusSucceeded,
			}
			_, err := db.InsertServiceInstance(serviceInstanceDB)
			if err != nil {
				util.WriteHttpResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			lastOperation = &model.LastOperation{State: "succeeded", Description: "database was created already"}
			response := model.CreateServiceInstanceResponse{LastOperation: lastOperation}
			util.WriteHttpResponse(w, http.StatusCreated, response)
			return
		}
		// nothing created or in progress for this database, so we create it here and now
		iaasInstance := db.IaaSInstance{
			InternalId:       "s" + strings.ReplaceAll(time.Now().Format("20060102T150405.999"), ".", "-"),
			Status:           "preparing for create",
			LastStatusUpdate: time.Now(),
			LastMessage:      "no last message yet",
			ServiceUrl:       "no service URL",
			ServiceUser:      "no user yet",
			ServicePassword:  "no password yet",
		}

		iaasInstanceId, err := db.InsertIaaSInstance(iaasInstance)
		if err != nil {
			util.WriteHttpResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		serviceInstance := db.ServiceInstance{
			ServiceId:        serviceInstance.ServiceId,
			InstanceId:       serviceInstanceId,
			PlanId:           serviceInstance.PlanId,
			Parameters:       parameters,
			Env:              conf.CfEnv,
			OrganizationName: serviceInstance.Context.OrgName,
			SpaceName:        serviceInstance.Context.SpaceName,
			InstanceName:     serviceInstance.Context.InstanceName,
			IaaSInstanceId:   iaasInstanceId,
			Status:           db.StatusInProgress,
		}
		_, err = db.InsertServiceInstance(serviceInstance)
		if err != nil {
			util.WriteHttpResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		// fire up the provisioning in the background
		err = aws.SubmitProvisioning(iaasInstanceId)

		if err == nil {
			lastOperation = &model.LastOperation{State: "in progress", Description: "creating service instance..."}
			response := model.CreateServiceInstanceResponse{LastOperation: lastOperation}
			util.WriteHttpResponse(w, http.StatusAccepted, response)
			return
		}
		util.WriteHttpResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	// an operation (create or delete) is already in progress from another foundation, only return current status
	iaasInstance := db.GetIaaSInstances(serviceInstancesInProgress[0].IaaSInstanceId)[0]
	if iaasInstance.Status == db.StatusCreateInProgress {
		serviceInstance := db.ServiceInstance{
			ServiceId:        serviceInstance.ServiceId,
			InstanceId:       serviceInstanceId,
			PlanId:           serviceInstance.PlanId,
			Env:              conf.CfEnv,
			OrganizationName: serviceInstance.Context.OrgName,
			SpaceName:        serviceInstance.Context.SpaceName,
			InstanceName:     serviceInstance.Context.InstanceName,
			IaaSInstanceId:   iaasInstance.Id,
			Status:           db.StatusInProgress,
		}
		_, err := db.InsertServiceInstance(serviceInstance)
		if err != nil {
			util.WriteHttpResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		lastOperation = &model.LastOperation{State: "in progress", Description: fmt.Sprintf("service instance create is in progress from foundation %s...", serviceInstancesInProgress[0].Env)}
		aws.StartPollForStatus(iaasInstance.Id)
		response := model.CreateServiceInstanceResponse{LastOperation: lastOperation}
		util.WriteHttpResponse(w, http.StatusAccepted, response)
		return
	}
	util.WriteHttpResponse(w, http.StatusBadRequest, fmt.Sprintf("a DELETE request is already in progress from foundation %s", serviceInstancesInProgress[0].Env))
}

func DeleteServiceInstance(w http.ResponseWriter, r *http.Request) {
	serviceInstanceId := mux.Vars(r)["service_instance_guid"]
	fmt.Printf("delete service instance %s...\n", serviceInstanceId)
	serviceInstance := db.GetServiceInstanceByInstanceId(serviceInstanceId)
	serviceName := util.GetServiceById(serviceInstance.ServiceId).Name
	if serviceInstance.InstanceName == "" {
		util.WriteHttpResponse(w, http.StatusGone, fmt.Sprintf("service instance with guid %s not found", serviceInstanceId))
		return
	}
	iaasInstance := db.GetIaaSInstances(serviceInstance.IaaSInstanceId)[0]
	if iaasInstance.Status == db.StatusCreateInProgress {
		response := model.DeleteServiceInstanceResponse{Result: fmt.Sprint("There is still a create in progress (from another foundation)")}
		util.WriteHttpResponse(w, http.StatusBadRequest, response)
		return
	}
	if iaasInstance.Status == db.StatusDeleteInProgress {
		db.UpdateStatusServiceInstance(serviceInstance, db.StatusInProgress)
		response := model.DeleteServiceInstanceResponse{Result: fmt.Sprint("There is still a delete in progress (from another foundation)")}
		aws.StartPollForStatus(iaasInstance.Id)
		util.WriteHttpResponse(w, http.StatusAccepted, response)
		return
	}
	if iaasInstance.Status == db.StatusDeleteSucceeded {
		response := model.DeleteServiceInstanceResponse{Result: fmt.Sprint("The database was already deleted (from another foundation)")}
		db.DeleteServiceInstanceByServiceInstanceId(serviceInstanceId)
		util.WriteHttpResponse(w, http.StatusOK, response)
		return
	}
	// all looks good so far, now check if we are the last service instance for the same iaas_instance, if so then do the actual delete of the service, if not, respond with StatusDeleteSucceeded.
	if !db.IsLastServiceInstanceForIaaS(iaasInstance.Id) {
		response := model.DeleteServiceInstanceResponse{Result: fmt.Sprint("The physical database was not yet deleted (still in use by another foundation)")}
		db.DeleteServiceInstanceByServiceInstanceId(serviceInstanceId)
		util.WriteHttpResponse(w, http.StatusOK, response)
		return
	}

	// submit the actual delete
	var err error
	if strings.HasPrefix(serviceName, "rds-service") {
		err = aws.SubmitDeletionRDSDB(iaasInstance, serviceInstance)
	}
	if strings.HasPrefix(serviceName, "documentdb-service") {
		err = aws.SubmitDeletionDOCDB(iaasInstance, serviceInstance)
	}
	if err != nil {
		response := model.DeleteServiceInstanceResponse{Result: fmt.Sprintf("Delete failed, error: %s", err)}
		util.WriteHttpResponse(w, http.StatusBadRequest, response)
		return
	}
	response := model.DeleteServiceInstanceResponse{Result: fmt.Sprintf("Delete of %s in progress...", iaasInstance.InternalId)}
	util.WriteHttpResponse(w, http.StatusAccepted, response)
}
