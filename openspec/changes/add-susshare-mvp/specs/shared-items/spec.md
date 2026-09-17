## Purpose

定义文本和文件共享条目的生命周期、可见性、内容访问、过期清理和安全预览行为，确保 SusShare 在无数据库的单实例环境下仍有可预测的存储语义。

## ADDED Requirements

### Requirement: Publish shared items
The system SHALL allow authenticated clients to publish a UTF-8 text item or one binary file item through the REST API. The server SHALL assign the item identifier and SHALL NOT accept client-supplied identifiers. A multi-file upload SHALL create one independent file item per file and SHALL NOT create an upload batch entity.

#### Scenario: Publish text
- **WHEN** an authenticated client publishes non-empty UTF-8 text within the configured text limit
- **THEN** the system creates one text item with a server-generated identifier, title, size, creation time, and expiry time

#### Scenario: Upload multiple files
- **WHEN** an authenticated client uploads three files in one request
- **THEN** the system creates three independent file items, each retaining exactly one uploaded file and its own metadata

### Requirement: List and read shared items
The system SHALL return active items in descending creation time order. Item data SHALL include the identifier, kind, title, filename for file items, content classification, size, creation time, expiry time, and whether browser preview is safe. Expired and unfinished items SHALL NOT appear in the list.

#### Scenario: List newest first
- **WHEN** item A is created before item B and both are active
- **THEN** the list returns B before A

#### Scenario: Read content
- **WHEN** an authenticated client requests the content of an active item
- **THEN** the system returns the original bytes for that item and does not transform text into a `.txt` file

### Requirement: Enforce item limits
The system SHALL reject text larger than the configured text limit, files larger than the configured maximum file size, and writes that would exceed the configured total storage size. A rejected publish SHALL NOT leave a finalized item visible.

#### Scenario: Text exceeds its limit
- **WHEN** a client publishes text larger than `limits.max_text_size`
- **THEN** the request fails with a limit error and no text item is listed

#### Scenario: Storage would exceed its limit
- **WHEN** a file upload would make total stored content exceed `limits.max_storage_size`
- **THEN** the request fails with a limit error and the finalized item list remains unchanged

### Requirement: Persist only complete items
The system SHALL make an item visible only after its content and metadata are durably staged and atomically promoted to the final storage area. On startup, the system SHALL recover finalized items and discard unfinished staging entries without presenting them as shared items.

#### Scenario: Interrupted publish
- **WHEN** the process stops after content is staged but before the item is promoted
- **THEN** the item is absent from the list after restart and startup cleanup removes its staging entry

#### Scenario: Restart with finalized items
- **WHEN** the service restarts while finalized items remain under the storage root
- **THEN** those items remain listable with their original identifiers, content, creation times, and expiry times

### Requirement: Expire items deterministically
The system SHALL calculate every item's expiry time when it is created using the then-current retention policy. Changing the policy SHALL affect only subsequent items. Expired items SHALL be hidden from list and read operations, and the background cleanup worker SHALL eventually delete their content and metadata. The system SHALL NOT support permanent items.

#### Scenario: Existing item keeps its expiry
- **WHEN** an item is created with a 24-hour policy and the policy is then changed to 7 days
- **THEN** the existing item retains its original 24-hour expiry and only later items use 7 days

#### Scenario: Item expires
- **WHEN** an item's expiry time is in the past
- **THEN** list and read operations treat it as not found and cleanup eventually removes its stored directory

### Requirement: Delete shared items
The system SHALL allow an authenticated client to delete an active item by identifier. After successful deletion, the item's metadata and content SHALL NOT be accessible through the API.

#### Scenario: Delete an active item
- **WHEN** an authenticated client deletes an active item
- **THEN** subsequent list responses omit it and subsequent read requests return not found

### Requirement: Serve previews safely
The system SHALL classify preview safety from server-validated content type and extension, not from the client-supplied MIME value alone. Safe browser-native images, audio, video, and PDF MAY be served for inline preview with `X-Content-Type-Options: nosniff`, a sandboxing content security policy, and no referrer. Text SHALL be rendered by the Web UI as source or plain text. HTML, SVG, XML, unknown types, and extension/MIME mismatches SHALL NOT be rendered as active same-origin documents; they SHALL be shown as source where safe or offered as a download.

#### Scenario: Preview a safe image
- **WHEN** an authenticated user previews a PNG whose sniffed content and extension agree
- **THEN** the response is suitable for inline browser rendering and includes the required nosniff, sandbox, and no-referrer headers

#### Scenario: Preview an HTML file
- **WHEN** an authenticated user opens an uploaded HTML file
- **THEN** the system does not execute the document as same-origin HTML and offers source rendering or download instead
