create database mfsbdb;
use mfsbdb;
create user 'mfsb-user'@'%' identified by "mfsb-password";

-- drop table if exists service_instance;
-- drop table if exists iaas_instance;

-- create tables, see @create-tables.sql

grant select,update,insert,delete on mfsbdb.iaas_instance to 'mfsb-user'@'%';
grant select,update,insert,delete on mfsbdb.service_instance to 'mfsb-user'@'%';
grant select,update,insert,delete on mfsbdb.service_binding to 'mfsb-user'@'%';