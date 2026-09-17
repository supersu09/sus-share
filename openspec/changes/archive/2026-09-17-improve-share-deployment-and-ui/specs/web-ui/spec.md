## Purpose

定义共享板界面的语言、发布后的可见状态、附件区域行为、固定输入布局和视觉主题要求。

## ADDED Requirements

### Requirement: Chinese user interface
All operator-facing UI labels, titles, buttons, form labels, placeholders, status messages, confirmation messages, and errors rendered by SusShare MUST be in Chinese. Proper nouns, brand names, units, and protocol identifiers MAY remain non-Chinese.

#### Scenario: Review interface language
- **WHEN** a user opens the login view, shared board, preview dialog, and settings dialog
- **THEN** every operator-facing text is Chinese

### Requirement: Newly published text is visible immediately
After a text item is successfully published, its entry MUST appear in the shared list without a manual refresh. If updating the visible list fails, the UI MUST show a Chinese failure state.

#### Scenario: Publish text successfully
- **WHEN** the user submits non-empty text and the server accepts it
- **THEN** the composer is cleared
- **AND** the new text item is visible in the list immediately

#### Scenario: List update fails after publication
- **WHEN** the server accepts a text item but the subsequent list update fails
- **THEN** the UI shows a Chinese refresh failure

### Requirement: Item size appears once
Each item's formatted size MUST appear only on the right side of the item title row. The creation-time row MUST NOT repeat the size.

#### Scenario: Render an item
- **WHEN** an item is rendered in the shared list
- **THEN** its size is displayed on the title row's right side
- **AND** the time row contains no second size value

### Requirement: Upload success clears attachment state
After a successful upload, the selected file list and per-file attachment result rows MUST be cleared from the attachment area. Upload progress MAY remain visible while the request is in flight.

#### Scenario: Complete a file upload
- **WHEN** all selected files are uploaded successfully
- **THEN** the file input and attachment information area are cleared
- **AND** successfully uploaded items are visible in the shared list

### Requirement: Fixed bottom composer
The content publishing area MUST remain fixed at the bottom of the viewport. The item list MUST be independently scrollable above it, including on narrow screens, without controls or text overlapping.

#### Scenario: Use a short viewport
- **WHEN** the browser viewport is narrow or short
- **THEN** the composer remains fixed at the bottom
- **AND** list content scrolls above the composer without being permanently obscured

### Requirement: Tailwind theme
The shared board MUST use a TailwindCSS-based visual implementation with `#66CCFF` as its primary interactive theme color.

#### Scenario: Inspect the primary controls
- **WHEN** primary controls and selected states are rendered
- **THEN** their visual theme uses `#66CCFF`

### Requirement: Upload action labels
The text publishing submit button and file upload submit button MUST both be labeled `上传`.

#### Scenario: Review composer actions
- **WHEN** text mode and file mode are displayed
- **THEN** both submit buttons show `上传`
