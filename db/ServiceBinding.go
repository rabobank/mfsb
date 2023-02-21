package db

import (
	"database/sql"
	"fmt"
)

type ServiceBinding struct {
	Id                int64
	ServiceBindingId  string
	ServiceInstanceId string
}

func (si ServiceBinding) String() string {
	return fmt.Sprintf("ServiceBinding: Id:%d, ServiceBindingId:%s, ServiceInstanceId:%s", si.Id, si.ServiceBindingId, si.ServiceInstanceId)
}

func InsertServiceBinding(serviceBinding ServiceBinding) (int64, error) {
	var err error
	var Id int64
	db := GetDB()
	defer db.Close()
	result, err := db.Exec("insert into service_binding(service_binding_id, service_instance_id) values(?,?)", serviceBinding.ServiceBindingId, serviceBinding.ServiceInstanceId)
	if err != nil {
		fmt.Printf("failed to insert %v, error: %s\n", serviceBinding, err)
	} else {
		Id, _ = result.LastInsertId()
		serviceBinding.Id = Id
		fmt.Printf("inserted %v\n", serviceBinding)
	}
	return Id, err
}

func UpdateServiceBinding(serviceBinding ServiceBinding) error {
	var err error
	db := GetDB()
	defer db.Close()
	_, err = db.Exec("update service_binding set service_binding_id=?, service_instance_id=? where id=?", serviceBinding.ServiceBindingId, serviceBinding.ServiceInstanceId, serviceBinding.Id)
	if err != nil {
		fmt.Printf("failed to update %v, error: %s\n", serviceBinding, err)
	}
	return err
}

func GetServiceBindings(id int64) []ServiceBinding {
	var err error
	result := make([]ServiceBinding, 0)
	db := GetDB()
	defer db.Close()
	var rows *sql.Rows
	if id == 0 {
		rows, err = db.Query("select Id, service_binding_id, service_instance_id from service_binding")
	} else {
		rows, err = db.Query("select Id, service_binding_id, service_instance_id from service_binding where id=?", id)
	}
	if err != nil {
		fmt.Printf("failed to query the service_bindings, err: %s\n", err)
	} else {
		result = getServiceBindings(rows)
	}
	return result
}

func GetServiceBindingByBindingId(id string) ServiceBinding {
	var err error
	var serviceBinding ServiceBinding
	result := make([]ServiceBinding, 0)
	db := GetDB()
	defer db.Close()
	var rows *sql.Rows
	rows, err = db.Query("select Id, service_binding_id, service_instance_id from service_binding where service_binding_id=?", id)
	if err != nil {
		fmt.Printf("failed to query the service_binding for binding_id %s, err: %s\n", id, err)
	} else {
		result = getServiceBindings(rows)
	}
	if len(result) != 1 {
		fmt.Printf("no servicebinding found for service_binding_id %s\n", id)
		return serviceBinding
	}
	if len(result) == 1 {
		return result[0]
	}
	return ServiceBinding{}
}

func getServiceBindings(rows *sql.Rows) []ServiceBinding {
	result := make([]ServiceBinding, 0)
	if rows != nil {
		defer rows.Close()
		var Id int64
		var serviceBindingId, serviceInstanceId string
		for rows.Next() {
			err := rows.Scan(&Id, &serviceBindingId, &serviceInstanceId)
			if err != nil {
				fmt.Printf("failed to scan the service_binding row, error:%s\n", err)
			} else {
				result = append(result, ServiceBinding{
					Id:                Id,
					ServiceBindingId:  serviceBindingId,
					ServiceInstanceId: serviceInstanceId,
				})
			}
		}
	}
	return result
}

func DeleteServiceBinding(Id int64) {
	var err error
	db := GetDB()
	defer db.Close()
	_, err = db.Exec("delete from service_binding where id=?", Id)
	if err != nil {
		fmt.Printf("failed to delete ServiceBinding Id %d, error: %s\n", Id, err)
	}
}
