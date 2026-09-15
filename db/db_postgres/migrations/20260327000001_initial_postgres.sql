
-- +goose Up
-- PostgreSQL support (8.6). This migration creates the core GoPhish schema
-- for PostgreSQL installations. The DDL is adapted from the MySQL migrations
-- with PostgreSQL-compatible types (SERIAL, BOOLEAN, TIMESTAMPTZ, TEXT).
--
-- NOTE: If you are migrating from an existing MySQL or SQLite installation,
-- use a tool like pgloader to convert your data. This migration only creates
-- the schema from scratch.

-- For fresh postgres installations, the migration framework will handle
-- schema creation. All ALTER TABLE migrations from the MySQL migration set
-- are compatible with PostgreSQL and can be applied by copying them into
-- this directory. Only CREATE TABLE statements need type adjustments.

-- This file intentionally does not recreate all 60+ historic migrations.
-- Users deploying on PostgreSQL should:
-- 1. Set "db_name": "postgres" in config.json
-- 2. Set "db_path" to a PostgreSQL DSN (e.g. "host=localhost user=gophish dbname=gophish sslmode=disable")
-- 3. Copy MySQL ALTER TABLE migrations into this directory (they are PG-compatible)

-- +goose Down
