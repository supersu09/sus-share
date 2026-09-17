## Purpose

定义共享密码认证、会话生命周期和统一权限模型，保护 Web UI 与 REST API 而不引入个人账号或条目所有权。

## ADDED Requirements

### Requirement: Authenticate with a shared password
The system SHALL protect all Web UI and REST API operations except login with a session derived from the shared password. The configuration SHALL store an Argon2id password hash rather than a plaintext password. Failed authentication SHALL return a generic authentication error.

#### Scenario: Login succeeds
- **WHEN** a client submits the current shared password
- **THEN** the system issues a session cookie and allows access to protected operations

#### Scenario: Login fails
- **WHEN** a client submits an incorrect password
- **THEN** the system returns an authentication failure without revealing whether the account, hash, or password format was invalid

### Requirement: Use constrained sessions
Session cookies SHALL be HttpOnly and `SameSite=Strict`. The service SHALL NOT enable cross-origin CORS by default. Service restart SHALL invalidate every session. A session SHALL authorize the shared-password role only and SHALL NOT carry user identity or item ownership.

#### Scenario: Restart invalidates sessions
- **WHEN** the service restarts
- **THEN** previously issued session cookies fail authentication and clients must log in again

#### Scenario: Equal permissions
- **WHEN** two clients authenticate with the shared password
- **THEN** both may list, read, publish, and delete any shared item

### Requirement: Logout invalidates the current session
The system SHALL allow a client to invalidate its own session through logout. Logout SHALL NOT affect other sessions.

#### Scenario: Logout one client
- **WHEN** client A logs out while client B remains logged in
- **THEN** client A's session becomes invalid and client B's session remains valid

### Requirement: Invalidate all sessions after password change
After a password change is persisted and the in-memory configuration is replaced, the system SHALL invalidate every existing session, including the browser that submitted the change. Clients SHALL authenticate again with the new password.

#### Scenario: Password change logs out all clients
- **WHEN** an authenticated client changes the shared password successfully
- **THEN** every pre-change session, including the requesting browser session, fails subsequent authentication

### Requirement: Protect mutating operations from cross-site browsers
Mutating browser requests SHALL be accepted only with the intended request format and a valid `SameSite=Strict` session. The service SHALL reject cross-site form-style mutation attempts that do not meet the API request contract.

#### Scenario: Cross-site form mutation
- **WHEN** an untrusted website causes a browser to submit a multipart form to a mutating endpoint
- **THEN** the request is rejected without creating, changing, or deleting shared data
