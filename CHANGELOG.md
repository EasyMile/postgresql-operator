# Changelog

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
