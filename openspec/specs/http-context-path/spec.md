# HTTP Context Path Specification

## Purpose

定义 SusShare 在站点根路径和反向代理子路径下对外暴露时的 URL、资源引用和会话 Cookie 行为。

## Requirements

### Requirement: External context path configuration
The system SHALL support a configured external URL prefix for the Web UI and API. The prefix MUST be normalized to a single leading slash and no trailing slash, and the empty value MUST preserve root-path deployment.

#### Scenario: Configure a subpath with a trailing slash
- **WHEN** the external prefix is configured as `/share/`
- **THEN** the running configuration normalizes it to `/share`
- **AND** root-path deployment remains unchanged when the prefix is empty

#### Scenario: Visit the subpath without a trailing slash
- **WHEN** a user visits the configured external prefix without a trailing slash
- **THEN** the system redirects to the same prefix with a trailing slash

### Requirement: Subpath-safe assets and API requests
When the UI is served under the configured external prefix, all stylesheet, script, API, content, and preview URLs generated or requested by that page MUST resolve beneath the same external prefix.

#### Scenario: Nginx preserves the prefix
- **WHEN** the UI is opened at `https://www.suscode.cn/share/` and the proxy forwards `/share/...` to SusShare unchanged
- **THEN** the browser loads stylesheets and scripts from beneath `/share/`
- **AND** UI API requests are sent to `/share/api/v1/...`

#### Scenario: Nginx strips the prefix
- **WHEN** the proxy forwards `/share/...` to SusShare after removing `/share`
- **THEN** SusShare still serves the UI and API
- **AND** URLs emitted by the page remain beneath `/share/`

### Requirement: Context-scoped session cookies
Login and logout cookies MUST use the configured external prefix as their path. Changing the prefix MUST NOT alter cookie security attributes.

#### Scenario: Log in under a subpath
- **WHEN** a user authenticates while the external prefix is `/share`
- **THEN** the session cookie is scoped to `/share`
- **AND** logout clears the cookie with the same path
