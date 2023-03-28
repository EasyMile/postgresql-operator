# Changelog

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
