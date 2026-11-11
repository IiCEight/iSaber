\set ON_ERROR_STOP on
\getenv runtime_password ISABER_RUNTIME_PASSWORD
\getenv database POSTGRES_DB
BEGIN;
SELECT 'CREATE ROLE isaber_runtime LOGIN'
WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname='isaber_runtime') \gexec
ALTER ROLE isaber_runtime PASSWORD :'runtime_password';
REVOKE ALL ON DATABASE :"database" FROM PUBLIC;
GRANT CONNECT ON DATABASE :"database" TO isaber_runtime;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO isaber_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO isaber_runtime;
REVOKE ALL ON owners, goose_db_version FROM isaber_runtime;
GRANT SELECT ON owners, goose_db_version TO isaber_runtime;
GRANT UPDATE(singleton) ON owners TO isaber_runtime;
COMMIT;
