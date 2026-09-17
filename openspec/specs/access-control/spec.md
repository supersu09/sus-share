# Access Control Specification

## Purpose

定义共享密码的默认值、可接受的密码值范围，以及修改密码后的认证行为，同时保留已部署配置的兼容性。

## Requirements

### Requirement: Default shared password
When authentication is enabled and no password hash is configured, the system MUST accept `password` as the shared password. Once a password is changed, the resulting hash MUST be persisted and used on later restarts.

#### Scenario: Start with no password hash
- **WHEN** authentication is enabled and `auth.password_hash` is absent or empty
- **THEN** logging in with `password` succeeds
- **AND** logging in with any other password fails

#### Scenario: Change away from the default password
- **WHEN** the shared password is changed from the default
- **THEN** the new hash is persisted
- **AND** the default password no longer authenticates after a restart

### Requirement: No password policy restrictions
The application MUST NOT impose minimum length, maximum length, character-class, or complexity requirements on the shared password. Transport-level request limits MAY still apply, but they MUST NOT be exposed as password-policy errors.

#### Scenario: Change to a short password
- **WHEN** an authenticated user changes the password to a one-character value
- **THEN** the change succeeds
- **AND** the new password authenticates while the old password fails

#### Scenario: Change to an empty password
- **WHEN** an authenticated user changes the password to an empty value
- **THEN** the change succeeds
- **AND** submitting the empty password subsequently authenticates

### Requirement: Existing password hashes remain valid
An already configured Argon2id password hash MUST continue to authenticate with its original plaintext password without migration.

#### Scenario: Start with an existing hash
- **WHEN** authentication is enabled and a valid custom password hash is configured
- **THEN** the original password authenticates
- **AND** the default password does not override it
