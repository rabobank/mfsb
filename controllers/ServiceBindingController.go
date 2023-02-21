package controllers

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/rabobank/mfsb/db"
	"github.com/rabobank/mfsb/model"
	"github.com/rabobank/mfsb/util"
	"net/http"
	"strings"
)

func GetServiceBinding(w http.ResponseWriter, r *http.Request) {
	serviceInstanceId := mux.Vars(r)["service_instance_guid"]
	serviceBindingId := mux.Vars(r)["service_binding_guid"]
	fmt.Printf("get service binding %s for service instance %s...\n", serviceBindingId, serviceInstanceId)
	serviceInstance := db.GetServiceInstanceByInstanceId(serviceInstanceId)
	if serviceInstance.Id == 0 {
		util.WriteHttpResponse(w, http.StatusNotFound, fmt.Sprintf("ServiceInstance %s not found", serviceInstanceId))
	} else {
		creds := getCredentialsForBinding(serviceBindingId)
		response := model.CreateServiceBindingResponse{Credentials: &creds}
		util.WriteHttpResponse(w, http.StatusOK, response)
	}
}

func CreateServiceBinding(w http.ResponseWriter, r *http.Request) {
	serviceInstanceId := mux.Vars(r)["service_instance_guid"]
	serviceBindingId := mux.Vars(r)["service_binding_guid"]
	fmt.Printf("create service binding %s for service instance %s...\n", serviceBindingId, serviceInstanceId)
	serviceBinding := db.GetServiceBindingByBindingId(serviceBindingId)
	if serviceBinding.ServiceBindingId == "" {
		_, err := db.InsertServiceBinding(db.ServiceBinding{
			ServiceBindingId:  serviceBindingId,
			ServiceInstanceId: serviceInstanceId,
		})
		if err != nil {
			util.WriteHttpResponse(w, http.StatusBadRequest, err.Error())
		} else {
			creds := getCredentialsForBinding(serviceBindingId)
			response := model.CreateServiceBindingResponse{Credentials: &creds}
			util.WriteHttpResponse(w, http.StatusCreated, response)
		}
	} else {
		creds := getCredentialsForBinding(serviceBindingId)
		response := model.CreateServiceBindingResponse{Credentials: &creds}
		util.WriteHttpResponse(w, http.StatusOK, response)
	}
}

func DeleteServiceBinding(w http.ResponseWriter, r *http.Request) {
	serviceInstanceId := mux.Vars(r)["service_instance_guid"]
	serviceBindingId := mux.Vars(r)["service_binding_guid"]
	fmt.Printf("delete service binding %s for service instance %s...\n", serviceBindingId, serviceInstanceId)
	db.DeleteServiceBinding(db.GetServiceBindingByBindingId(serviceBindingId).Id)
	util.WriteHttpResponse(w, http.StatusOK, db.ServiceBinding{}) // TODO how do we return an empty array object {} (without quotes around it)
}

func getCredentialsForBinding(id string) model.Credentials {
	iaasInstance := db.GetIaaSInstanceByBindingId(id)
	url := iaasInstance.ServiceUrl
	// parse   mysql://mysqluser:pass@mysqlhost:3306/dbname
	var username, password, host, dbname, port string
	parts1 := strings.Split(url, ":")
	username = strings.ReplaceAll(parts1[1], "/", "")
	parts2 := strings.Split(parts1[2], "@")
	password = parts2[0]
	host = parts2[1]
	parts3 := strings.Split(parts1[3], "/")
	port = parts3[0]
	dbname = parts3[1]
	creds := model.Credentials{
		Uri:      url,
		UserName: username,
		Password: password,
		Host:     host,
		Port:     port,
		Database: dbname,
	}
	return creds
}
