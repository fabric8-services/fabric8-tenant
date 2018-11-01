ALTER TABLE tenants ADD COLUMN ns_username TEXT;
CREATE INDEX idx_ns_username ON tenants USING btree (ns_username);