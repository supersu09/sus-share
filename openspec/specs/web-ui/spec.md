# Web UI Specification

## Purpose

定义共享板界面的语言、发布后的可见状态、附件区域行为、固定输入布局和视觉主题要求。

## Requirements

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
The content publishing area MUST remain fixed at the bottom of the viewport and MUST use a default vertical size of 200px when the board is displayed. The publishing area MUST expose a drag control at its top edge that changes its height when dragged upward or downward. Height changes MUST immediately resize the independently scrollable item list above the composer without allowing controls, text, or the drag control to overlap. The adjusted height MUST remain bounded so the composer never covers the board header and always leaves the item-list viewport usable. The user's height adjustment MUST NOT persist after a page reload.

The composer MUST support pointer dragging and keyboard height adjustment. The text input MUST NOT provide the browser's independent resize affordance. In text and file modes, the upload input area MUST occupy the same content width as the text input and the active mode MUST remain usable at every permitted composer height.

#### Scenario: Use a short viewport
- **WHEN** the browser viewport is narrow or short
- **THEN** the composer remains fixed at the bottom
- **AND** list content scrolls above the composer without being permanently obscured

#### Scenario: Open the board
- **WHEN** the board becomes visible
- **THEN** the publishing area's height is 200px
- **AND** the active text or file form fits without overlap

#### Scenario: Drag the composer taller
- **WHEN** the user drags the composer's top control upward
- **THEN** the publishing area height increases within its permitted bounds
- **AND** the item list's available height decreases immediately

#### Scenario: Drag the composer shorter
- **WHEN** the user drags the composer's top control downward
- **THEN** the publishing area height decreases within its permitted bounds
- **AND** no composer control, text, or list content overlaps

#### Scenario: Adjust the composer with the keyboard
- **WHEN** the drag control has focus and the user presses the arrow keys
- **THEN** the publishing area height changes in the corresponding direction within its permitted bounds

#### Scenario: Switch composer modes
- **WHEN** the user switches between text and file modes
- **THEN** the file upload area and the text input have the same content width
- **AND** the current composer height is retained

### Requirement: Centered modal dialogs
All operator-facing modal dialogs for settings, viewing, and preview MUST be horizontally and vertically centered in the available viewport when opened. A dialog MUST NOT extend beyond the viewport bounds, and content taller than the available dialog body MUST scroll within the dialog rather than moving the shared board underneath it.

#### Scenario: Open a modal
- **WHEN** the user opens the settings dialog, a text view dialog, or a file preview dialog
- **THEN** the dialog is centered horizontally and vertically in the viewport
- **AND** its edges remain inside the viewport

#### Scenario: Display tall modal content
- **WHEN** preview or settings content is taller than the available viewport
- **THEN** the dialog remains centered and within viewport bounds
- **AND** the content scrolls inside the dialog without scrolling the board page underneath

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
