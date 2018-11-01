ALTER TABLE tenants ADD COLUMN ns_username TEXT;
CREATE INDEX idx_ns_username ON tenants USING btree (ns_username);
UPDATE tenants t SET t.ns_username = (SELECT name FROM namespaces n WHERE t.id = n.tenant_id AND t.type = 'user');
