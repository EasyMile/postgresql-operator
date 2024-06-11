# Changelog

## 3.2.0

### Features

- Add support for type ownership recover in database management

### Bugs

- Remove duplicated metadata in leader election role in Helm chart

## 3.1.0

### Features

- Add support for grant role with admin option if asked
- Add public schema by default in PostgreSQLDatabase objects if nothing is set
- Add support for replica urls (with bouncer supported)

### Bugs

- Ensure database owner is set correctly
- Ensure all tables under listed schema have the right owner

## 3.0.0

### Breaking change

- Do not support anymore postgresql user. Switch to postgresqluserrole is now mandatory

### Features

- Improve Helm chart with new features and code
- Add support for PGBouncer in PosgtreSQL Engine Configurations and PostgreSQL User Roles
- Add custom metric to count reconcile errors in a detailed manner. This is including resource name and namespace in labels where the default provided isn't

### Bugs

- Fix application port in Helm chart
- Fix unnecessary requeue

## 2.1.2

### Bugs

- Change deletion algorithm and add some security to avoid status flush
- Fix pgdb deletion not checking is pgur exists

## 2.1.1

### Bugs

- Fix to add support for args in helm chart
- Fix to avoid doing requeue on success run and prefer a full resync

## 2.1.0

### Deprecation notice

- Deprecate PostgresqlUser custom resource in favor or PostgresqlUserRole
  - This new resource will allow more thing and a greater stability

### Feature

- Add support for PostgresqlUserRole custom resource

### Bugs

- Fix Helm chart CRD and structure

## 2.0.0

### Feature

- Complete rework and upgrade of operator-sdk to latest version

### Tests

- Add tests for all controllers

### Bugs

- Patch bugs detected with tests

## 1.1.2

### Bugs

- Fix group cannot be used to create a database because admin user isn't in that group

## 1.1.1

### Bugs

- Check if errors exist before logging
- Create database with owner directly to avoid having databases with wrong owner
- Fix potential race between default values save and current run of reconciler

## 1.1.0

### Bugs

- Fix on dev resources
- Fix autoheal on schema and extensions in databases
- Fix typo in database user secret

### Features

- Keep all pools in memory to avoid recreating them at each synchronization loop
- Check if roles and database don't already exist before trying to create them

## 1.0.1

### Bugs

- Fix possible too long name generated for roles (PostgreSQL only support 63 characters maximum for identifiers)

## 1.0.0

### Features

- Add support for PostgresqlEngineConfiguration
- Add support for PostgresqlDatabase
- Add support for PostgresqlUser
- Create or update Databases with extensions and schemas
- Create or update Users with rights (Owner, Writer or Reader)
- Connections to multiple PostgreSQL Engines
- Generate secrets for User login and password
- Allow to change User password based on time (e.g: Each 30 days)
