## Purpose

定义 SusShare 的配置边界、持久化行为和运行时更新语义，让服务器管理员控制部署约束，同时让 Web 只维护极少数设置。

## ADDED Requirements

### Requirement: Use an explicit configuration schema
The service SHALL read a YAML configuration containing server listening settings, storage root, retention policy, authentication settings, content limits, and web title. Defaults SHALL provide a finite retention period, enabled authentication, a 1 MiB text limit, and explicit file and total storage limits. The configuration SHALL support an Argon2id password hash and SHALL NOT require a plaintext password.

#### Scenario: Start with configured paths
- **WHEN** the service starts with `storage.path` set to `/data` and config volume `/config`
- **THEN** it reads configuration from the config volume and stores item data under the configured storage root

#### Scenario: Start without a password hash
- **WHEN** authentication is enabled and no valid password hash is configured
- **THEN** startup fails with a configuration error rather than accepting a default password

### Requirement: Separate static server configuration from runtime settings
The Web API SHALL permit runtime changes only for retention policy and password. Listening address and port, storage path, HTTPS behavior, maximum file size, maximum storage size, maximum text size, API behavior, and logging level SHALL be server-managed configuration and SHALL NOT be editable through the Web UI.

#### Scenario: Inspect settings
- **WHEN** an authenticated user retrieves runtime settings
- **THEN** the response exposes retention policy and password-change semantics but no storage path or listener controls

#### Scenario: Attempt to change storage path
- **WHEN** an authenticated user submits a storage path through the settings API
- **THEN** the request is rejected as a forbidden or unknown setting

### Requirement: Update retention policy atomically
A successful retention update SHALL validate a finite duration between one hour and seven days, persist the new value, and make it apply to subsequently created items without changing existing item expiry times. A failed update SHALL leave both the persisted and in-memory policy unchanged.

#### Scenario: Change retention
- **WHEN** an authenticated user changes retention from 24 hours to 3 days
- **THEN** subsequent items receive 72-hour expiry, existing items keep their old expiry, and the setting survives restart

#### Scenario: Submit permanent retention
- **WHEN** an authenticated user submits a permanent or zero-duration retention value
- **THEN** the update is rejected and the current policy remains active

### Requirement: Update password atomically
A successful password update SHALL validate the current password, hash the new password with Argon2id, persist the hash, replace the active hash in memory, and invalidate all sessions. A failed update SHALL leave the previous hash and sessions usable.

#### Scenario: Password update succeeds
- **WHEN** the correct current password and a valid new password are submitted
- **THEN** the persisted hash changes, all sessions invalidate, and the new password is required for login

#### Scenario: Current password is wrong
- **WHEN** password change is submitted with an incorrect current password
- **THEN** no hash changes and existing sessions remain valid

### Requirement: Persist configuration safely
Runtime configuration changes SHALL be serialized and written through a same-directory temporary file followed by an atomic rename. If persistence fails, the service SHALL retain the previous configuration file and active in-memory settings. A successful rewrite MAY discard administrator comments and formatting.

#### Scenario: Configuration write fails
- **WHEN** the storage or config filesystem rejects the temporary configuration write
- **THEN** the API returns an error and both the active and persisted retention values remain unchanged

### Requirement: Deploy as a single container
The service SHALL ship a Docker image that runs one server process and mounts `/config` and `/data` as persistent volumes. The container SHALL NOT require PostgreSQL, Redis, or a separate desktop client.

#### Scenario: Run the container
- **WHEN** the container starts with config and data volumes attached
- **THEN** the Web UI and REST API are served by the single process and survive container recreation through the mounted volumes
