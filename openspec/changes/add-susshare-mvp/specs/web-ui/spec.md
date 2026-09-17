## Purpose

定义 SusShare 的单页操作界面，让用户登录后查看共享内容、发布文本或文件、手动刷新并维护有限的运行时设置。

## ADDED Requirements

### Requirement: Provide the shared board layout
The Web UI SHALL present active shared items above the publish area. The publish area SHALL default to text mode and offer a file mode. The UI SHALL NOT display a device identity or source device label for an item.

#### Scenario: Open the shared board
- **WHEN** an authenticated user opens the application
- **THEN** active items appear above the composer and the composer defaults to text mode

#### Scenario: Switch to file mode
- **WHEN** the user selects file mode
- **THEN** the composer accepts one or more files through selection or drag-and-drop

### Requirement: Refresh manually
The Web UI SHALL load items once on entry and refresh them only when the user activates refresh. It SHALL NOT open a WebSocket, poll on a timer, or subscribe to server push. During refresh the control SHALL show an in-progress state; on failure the UI SHALL show a concise refresh error and retain the current display.

#### Scenario: Refresh succeeds
- **WHEN** the user activates refresh and the item request succeeds
- **THEN** the list is replaced with the current active items and the refresh control returns to its normal state

#### Scenario: Refresh fails
- **WHEN** the user activates refresh and the item request fails
- **THEN** the UI shows a refresh error and leaves the previously displayed items intact

### Requirement: Publish text and files
Text mode SHALL allow the user to paste or enter UTF-8 text and publish it as a text item without creating a `.txt` file. File mode SHALL allow multiple files and SHALL report per-file upload progress or outcome. Successful publish SHALL make the new item available on the next list refresh.

#### Scenario: Publish pasted SQL
- **WHEN** the user pastes SQL into text mode and publishes it
- **THEN** a text item containing the exact pasted bytes is created and appears after refresh

#### Scenario: Upload mixed files
- **WHEN** the user uploads a ZIP and a YAML file
- **THEN** two file items are created and each can be downloaded independently

### Requirement: Interact with shared items
The UI SHALL provide copy and view controls for text items, and download and delete controls for text and file items as applicable. Text view SHALL render content as source or plain text. File interaction SHALL use the safe preview classification and offer download when inline preview is unavailable or unsafe.

#### Scenario: Copy text
- **WHEN** the user activates copy on a text item
- **THEN** the browser places the item text on the clipboard and the UI reports success or failure

#### Scenario: Open unsupported file
- **WHEN** the user opens a file whose type cannot be safely previewed
- **THEN** the UI offers download rather than attempting a server-side parser

### Requirement: Provide login and settings screens
Unauthenticated users SHALL see a password login screen. Authenticated users SHALL be able to open a settings modal that exposes only retention policy and password change. Retention choices SHALL be limited to the supported finite durations.

#### Scenario: Open settings
- **WHEN** an authenticated user opens settings
- **THEN** only retention policy and password change controls are available

#### Scenario: Change password from the UI
- **WHEN** the user submits a valid new password
- **THEN** the settings operation succeeds and the UI returns to login with all old sessions invalidated
