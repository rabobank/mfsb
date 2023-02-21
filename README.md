## MFSB, Multi Foundation Service Broker

A Cloud Foundry Service Broker that supports multiple cloud foundry foundations while "sharing" the created services.  
It implements the [Open Service Broker API](https://github.com/openservicebrokerapi/servicebroker/blob/v2.16/spec.md)


## INTRO

The input for mfsb consists of the following environment variables:
* **MFSB_DEBUG** - If debug logging should be enabled, can be true or false, default is false 
* **MFSB_IAAS** - Indicate on which IaaS we are running, currently only AWS. The catalog is read from <MFSB_CATALOG_DIR>/<MFSB_IAAS>.json
* **MFSB_CATALOG_DIR** - the directory where the catalogs (json files) can be found
* **MFSB_BROKER_USER** - the userid to used when creating the cf service broker (`cf create-service-broker`)
* **MFSB_BROKER_DB_USER** - the userid for the mfsb it's own database
* **MFSB_BROKER_DB_NAME** - the name of the (mysql) database, default is `mfsbdb`
* **MFSB_BROKER_DB_HOST** - the host where the database is running, default is `localhost`
* **MFSB_LISTEN_PORT** - the tcp port the broker should listen on
* **MFSB_CF_ENV** - the "cloud foundry environment", a string representing in which env the the broker is running (d03 p04)
* **MFSB_RDS_SUBNETGRP** - the RDS SubnetGroup to attach to RDS instances
* **MFSB_RDS_SECGRP_ID** - the VPC Security Group (the Id) to attach to RDS instances
* **MFSB_DOCDB_SUBNETGRP** - the DOCDB SubnetGroup to attach to DocumentDB clusters
* **MFSB_DOCDB_SECGRP_ID** - the VPC Security Group (the Id) to attach to DocumentDB clusters
* **MFSB_PERMISSION_BOUNDARY_ARN** - mfsb can add an IAM role to allow teams limited access to the created databases, this property defines the ARN of the IAM Permission Boundary that will be set on it 
* **MFSB_POLICY_ARN** - mfsb can add an IAM role to allow teams limited access to the created databases, this property defines the ARN of the IAM Policy that will be attached to this role 

The following are properties to be set in credhub, do this by creating a credhub service instance, and binding the mfsb app to it:
* ``cf create-service --wait credhub default mfsb-credentials -c '{ "MFSB_BROKER_PASSWORD": "secret1", "MFSB_BROKER_DB_PASSWORD": "secret2" , "MFSB_ENCRYPT_KEY": "secret3" }'``
* ``cf bind-service mfsb mfsb-credentials``
* **MFSB_BROKER_PASSWORD** - the password for the MFSB_BROKER_USER
* **MFSB_BROKER_DB_PASSWORD** - the password for the MFSB_BROKER_DB_USER
* **MFSB_ENCRYPT_KEY** - The encryption key that is used to encrypt/decrypt the generated database admin passwords which are stored in the mfsb database.


### What is the issue with sharing databases between multiple foundations and multiple brokers?

When you create an instance on one foundation (`cf create-service mfsb-aws-service small myWonderfullService -c '{"parm1":"value-for-parm1"}'`) the request goes to the 
cloud controller (cc), the cc generates a guid and starts using that guid as the unique name for the service beginning with sending a provision request to the broker (a PUT to `/v2/service_instances/{service_instance_guid}`)  
When you repeat this on the next foundation, you get a new guid and (it does not matter if you talk to the same broker instance or against another one) that would start provisioning a new instance again.  
So the broker(s) first need to find out if a provision request is for a resource already provisioned on another foundation, for that you need a predictable unique name.  
We are using the request body that is sent along with the provision request, see the file ./resources/samples/create-instance-request-body.json, the things we need from there are 3-fold:  
* org_name
* space_name
* instance_name  
These 3 together form the (let's call it) unique key. These should be stored in the database that is shared between the broker instances. (together with other data as well)

### Broker behaviour to support multiple foundations

#### Create or Delete a service instance
When a service create request comes in, the requested resource (org/space/instance name) can be in 3 different states:
* not found, this is the first time one of the brokers gets a request for the resource
* in progress (create in progress), the resource has already been requested by a broker in (this or) another foundation
* in progress (delete in progress), the resource once was there, but a delete request was sent by a broker in (this or) another foundation
* succeeded (created or deleted), the resource is created or has been deleted by a broker in (this or) another foundation

The following responses should be given depending on the request (PUT/GET/DELETE) and the state of the requested resource:
Request is **GET**
* status=notfound => respond with 202 (Accepted) and start provisioning
* status=in progress => respond with 200 and state "in progress" with optional description giving more details about the progress
* status=succeeded =>  
Request is **PUT**
Request is **DELETE**
* status is "succeeded": it can already respond with a 200 response
* status is "in progress": it should respond with a 202 Accepted response and do nothing
* status is "deleted" (it once was created, but deleted afterwards) or no entry yet exists, it should:
  * update status to "in progress"
  * respond with a 202 Accepted response 
  * start provisioning the service
Once the service is provisioned, the status should be set to "succeeded" (or "failed" if the provisioning failed)

#### Delete service
When a service delete request comes in, first the broker should check if there is already a service entry with the unique key (that might have come from another foundation):
* status is "succeeded":
  * if the current foundation is the last one update the status to "Deleting" and start physically deleting the resource, if not, respond only with Succeeded
  * respond with a 202 Accepted response
  * start deleting the service
* status is "in progress": respond with a 400 Bad Request, indicating that a creation is still in progress
* status is "deleted": respond with a 200 response
* status is "deleting": respond with a 202 Accepted response and do nothing
Once the service is deleted, the status should be updated to "deleted" (or failed if the deletion failed).

The service_instance table (shared between the multiple service brokers) that holds all this (and more) info, is described in resources/sql/create-tables.sql.

## Testing

### creating a local (mysql) test env

```
create user 'mfsb-user'@'localhost' identified by "mfsb-password";
create database mfsbdb;
grant all privileges on mfsbdb.* to 'mfsb-user'@'localhost';
```

### pushing the broker as an app on cloud foundry
```
push the broker app with a valid catalog.json to cloud foundry
# create the service-broker (can be space-scoped)
cf create-service-broker mfsb user pw https://mfsb.apps.\<mydomain\> --space-scoped
```

## creating the broker in cloud foundry:
```
cf create-service-broker mfsb mfsb-broker-user pw https://mfsb.apps.\<mydomain\>
cf enable-service-access rds-service
cf enable-service-access rds-service-test -o system
```

To access the database manually:  
mysql --host=mfsb-db.\<xxxxx\>.\<zone\>.rds.amazonaws.com --user=mfsb_admin -p --password=mfsb_admin_pass

Database instance classes:
https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/Concepts.DBInstanceClass.html

