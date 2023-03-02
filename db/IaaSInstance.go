package db

import (
	"database/sql"
	"fmt"
	"github.com/rabobank/mfsb/util"
	"log"
	"time"
)

const (
	StatusCreateInProgress = "create in progress"
	StatusCreateFailed     = "create failed"
	StatusCreateSucceeded  = "create succeeded"
	StatusDeleteInProgress = "delete in progress"
	StatusDeleteFailed     = "delete failed"
	StatusDeleteSucceeded  = "delete succeeded"
	StatusNotFound         = "not found"
)

type IaaSInstance struct {
	Id               int64
	InternalId       string
	Status           string
	LastStatusUpdate time.Time
	LastMessage      string
	ServiceUrl       string
	ServiceUser      string
	ServicePassword  string
}

func (ii IaaSInstance) String() string {
	return fmt.Sprintf("IaaSInstance: Id:%d, InternalId:%s, Status:%s, LastStatusUpdate:%s, LastMessage:%s, ServiceUrl:%s, ServiceUser:%s, ServicePasswd:redacted", ii.Id, ii.InternalId, ii.Status, ii.LastStatusUpdate, ii.LastMessage, ii.ServiceUrl, ii.ServiceUser)
}

func InsertIaaSInstance(iaasInstance IaaSInstance) (int64, error) {
	var err error
	var Id int64
	db := GetDB()
	defer db.Close()
	passwordEncrypted, err := util.Encrypt(iaasInstance.ServicePassword)
	if err != nil {
		return 0, err
	}
	urlEncrypted, err := util.Encrypt(iaasInstance.ServiceUrl)
	if err != nil {
		return 0, err
	}
	result, err := db.Exec("insert into iaas_instance(internal_id, Status, last_status_update, last_message, service_url, service_user, service_password) values(?,?,?,?,?,?,?)",
		iaasInstance.InternalId, iaasInstance.Status, iaasInstance.LastStatusUpdate, iaasInstance.LastMessage, urlEncrypted, iaasInstance.ServiceUser, passwordEncrypted)
	if err != nil {
		fmt.Printf("failed to insert IaaSInstance %v, error: %s\n", iaasInstance, err)
	} else {
		Id, _ = result.LastInsertId()
		iaasInstance.Id = Id
		fmt.Printf("inserted %v\n", iaasInstance)
	}
	return Id, err
}

func UpdateIaaSInstance(iaasInstance IaaSInstance) error {
	var err error
	db := GetDB()
	defer db.Close()
	passwordEncrypted, err := util.Encrypt(iaasInstance.ServicePassword)
	if err != nil {
		return err
	}
	urlEncrypted, err := util.Encrypt(iaasInstance.ServiceUrl)
	if err != nil {
		return err
	}
	_, err = db.Exec("update iaas_instance set internal_id=?, status=?, last_status_update=?, last_message=?, service_url=?, service_user=?, service_password=? where id=?",
		iaasInstance.InternalId, iaasInstance.Status, iaasInstance.LastStatusUpdate, iaasInstance.LastMessage, urlEncrypted, iaasInstance.ServiceUser, passwordEncrypted, iaasInstance.Id)
	if err != nil {
		fmt.Printf("failed to update IaaSInstance %v, error: %s\n", iaasInstance, err)
	}
	return err
}

func UpdateStatusIaaSInstance(iaasInstance IaaSInstance, status string, lastMessage string) {
	iaasInstance.Status = status
	iaasInstance.LastStatusUpdate = time.Now()
	iaasInstance.LastMessage = lastMessage
	_ = UpdateIaaSInstance(iaasInstance)
}

// GetIaaSInstances get one or all IaasInstances. Specify Id=0 to get all instances
func GetIaaSInstances(id int64) []IaaSInstance {
	var err error
	result := make([]IaaSInstance, 0)
	db := GetDB()
	defer db.Close()
	var rows *sql.Rows
	if id == 0 {
		rows, err = db.Query("select Id, internal_id, Status, last_status_update, last_message,service_url,service_user, service_password from iaas_instance")
	} else {
		rows, err = db.Query("select Id, internal_id, Status, last_status_update, last_message,service_url,service_user, service_password from iaas_instance where id=?", id)
	}
	if err != nil {
		fmt.Printf("failed to query the iaas_instances, err: %s\n", err)
	} else {
		result = getIaaSInstances(rows)
	}
	return result
}

func GetIaaSInstanceByBindingId(id string) IaaSInstance {
	var err error
	result := make([]IaaSInstance, 0)
	db := GetDB()
	defer db.Close()
	var rows *sql.Rows
	rows, err = db.Query("select i.Id, internal_id, i.Status, i.last_status_update, i.last_message,i.service_url,i.service_user, i.service_password from iaas_instance i, service_instance s, service_binding b where b.service_instance_id=s.instance_id and s.iaas_instance_id=i.id and b.service_binding_id=?", id)
	if err != nil {
		fmt.Printf("failed to query the iaas_instances for binding_id %s, err: %s\n", id, err)
	} else {
		result = getIaaSInstances(rows)
	}
	if len(result) == 1 {
		return result[0]
	}
	return IaaSInstance{}
}

func getIaaSInstances(rows *sql.Rows) []IaaSInstance {
	var err error
	result := make([]IaaSInstance, 0)
	if rows != nil {
		defer rows.Close()
		var Id int64
		var lastStatusUpdate time.Time
		var internalId, status, lastMessage, serviceUrl, serviceUser, servicePassword string
		for rows.Next() {
			err = rows.Scan(&Id, &internalId, &status, &lastStatusUpdate, &lastMessage, &serviceUrl, &serviceUser, &servicePassword)
			if err != nil {
				fmt.Printf("failed to scan the iaas_instance row, error:%s\n", err)
			} else {
				passwordDecrypted, err := util.Decrypt(servicePassword)
				if err != nil {
					log.Printf("failed to decrypt the service password for iaas_instance with id %d, error: %s", Id, err)
				}
				urlDecrypted, err := util.Decrypt(serviceUrl)
				if err != nil {
					log.Printf("failed to decrypt the service url for iaas_instance with id %d, error: %s", Id, err)
				}
				result = append(result, IaaSInstance{
					Id:               Id,
					InternalId:       internalId,
					Status:           status,
					LastStatusUpdate: lastStatusUpdate,
					LastMessage:      lastMessage,
					ServiceUrl:       urlDecrypted,
					ServiceUser:      serviceUser,
					ServicePassword:  passwordDecrypted,
				})
			}
		}
	}
	return result
}

func IsLastServiceInstanceForIaaS(instanceId int64) bool {
	var err error
	db := GetDB()
	defer db.Close()
	var rows *sql.Rows
	rows, err = db.Query("select count(*) from service_instance si, iaas_instance ii where si.iaas_instance_id=ii.id and si.iaas_instance_id=?", instanceId)
	if err != nil {
		fmt.Printf("failed to query for last service_instance for IaaS Id %d: %s\n", instanceId, err)
		return false
	} else {
		if rows.Next() {
			var numInstances int
			err = rows.Scan(&numInstances)
			if err != nil {
				log.Printf("failed to get the number of service_instance for IaaS id %d: %s\n", instanceId, err)
				return false
			}
			if numInstances == 1 {
				return true
			}
		}
	}
	return false
}
