CREATE TABLE tenants_update (
  last_version_fabric8_tenant_user_file text,
  last_version_fabric8_tenant_che_mt_file text,
  last_version_fabric8_tenant_jenkins_file text,
  last_version_fabric8_tenant_jenkins_quotas_file text,
  last_version_fabric8_tenant_che_file text,
  last_version_fabric8_tenant_che_quotas_file text,
  last_version_fabric8_tenant_deploy_file text,
  status text,
  failed_count int,
  last_time_updated timestamp with time zone
);

INSERT INTO tenants_update VALUES (DEFAULT,DEFAULT,DEFAULT,DEFAULT,DEFAULT,DEFAULT,DEFAULT,'finished',DEFAULT,DEFAULT);
ALTER TABLE namespaces ADD COLUMN updated_by TEXT;