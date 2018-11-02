ALTER TABLE tenants ADD COLUMN ns_base_name TEXT;
CREATE INDEX idx_ns_base_name ON tenants USING btree (ns_base_name);
UPDATE tenants SET ns_base_name = (SELECT name FROM namespaces n WHERE tenants.id = n.tenant_id AND n.type = 'user');
