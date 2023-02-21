insert into iaas_instance(id, internal_id, status, last_status_update, last_message, service_url, service_user, service_password)
values (47, 'internalID1', 'succeeded', current_timestamp, 'service has been succeeded', 'jdbc://ergens', 'serviceuser', 'servicepw');
insert into iaas_instance(id, internal_id, status, last_status_update, last_message, service_url, service_user, service_password)
values (42, 'internalID2', 'in progress', current_timestamp, 'service is being succeeded', 'jdbc://ergens', 'unknown', 'unknown');

insert into service_instance(id, service_id, instance_id, plan_id, parameters, env, organization_name, space_name, instance_name, iaas_instance_id, status)
values (1, 'zerviceid1', 'inztanceid1', 'planid1', 'test-parameters', 'p03', 'it4it-org', 'panzer-space', 'mySweetRDSDB', 47, 'succeeded');

insert into service_instance(id, service_id, instance_id, plan_id, env, organization_name, space_name, instance_name, iaas_instance_id, status)
values (2, 'zerviceid1', 'inztanceid2', 'planid1', 'p04', 'it4it-org', 'panzer-space', 'mySweetRDSDB', 47, 'succeeded');

insert into service_binding(id, service_binding_id, service_instance_id)
values (1, 'binding-id1', 'inztanceid1');
insert into service_binding(id, service_binding_id, service_instance_id)
values (2, 'binding-id2', 'inztanceid1');
insert into service_binding(id, service_binding_id, service_instance_id)
values (3, 'binding-id3', 'inztanceid2');