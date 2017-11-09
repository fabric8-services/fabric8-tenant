CREATE TYPE profiles AS ENUM ('free', 'paid');
ALTER TABLE tenants ADD profile profiles default 'free';
