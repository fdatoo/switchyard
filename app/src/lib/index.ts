/**
 * Public surface of the Switchyard component library.
 *
 * Importing this module also imports the token CSS, so consumers don't need
 * to remember to load styles separately. The import order matters: tokens
 * must load before any component renders, otherwise the first paint flashes
 * un-themed.
 *
 * See `./README.md` for architecture and conventions.
 */

import "./tokens/index.css";

/* Theme: state, registry, and setup. */
export { useLanguageStore } from "./theme/language-store";
export { registerVariant, resolveVariant, listRegisteredLanguages } from "./theme/variant-registry";
export { installBuiltinVariants } from "./theme/builtin-variants";
export { BUILTIN_LANGUAGES } from "./theme/types";
export type {
  LanguageId,
  ModeId,
  LanguageDescriptor,
  SlotName,
  VariantRegistration,
} from "./theme/types";

/* Components. */
export { default as SyText } from "./components/text/SyText.vue";
export { default as SyButton } from "./components/button/SyButton.vue";
export { default as SySurface } from "./components/surface/SySurface.vue";
export { default as SyBadge } from "./components/badge/SyBadge.vue";
export { default as SyInput } from "./components/input/SyInput.vue";
export { default as SyCheckbox } from "./components/checkbox/SyCheckbox.vue";
export { default as SySwitch } from "./components/switch/SySwitch.vue";
export { default as SyIcon } from "./components/icon/SyIcon.vue";
export { default as SyKbd } from "./components/kbd/SyKbd.vue";
export { default as SyDot } from "./components/dot/SyDot.vue";
export { default as SyAvatar } from "./components/avatar/SyAvatar.vue";
export { default as SySpinner } from "./components/spinner/SySpinner.vue";
export { default as SyListRow } from "./components/listrow/SyListRow.vue";
export { default as SyEmptyState } from "./components/empty-state/SyEmptyState.vue";
export { default as SyTabs } from "./components/tabs/SyTabs.vue";
export { default as SyNavItem } from "./components/navitem/SyNavItem.vue";
export { default as SyBreadcrumb } from "./components/breadcrumb/SyBreadcrumb.vue";
export { default as SyTooltip } from "./components/tooltip/SyTooltip.vue";
export { default as SySheet } from "./components/sheet/SySheet.vue";
export { default as SyMenu } from "./components/menu/SyMenu.vue";
export { default as SyDataTable } from "./components/datatable/SyDataTable.vue";
export { default as SySearchInput } from "./components/search-input/SySearchInput.vue";
export { default as SyFilterChip } from "./components/filter-chip/SyFilterChip.vue";
export { default as SyStatusBar } from "./components/statusbar/SyStatusBar.vue";
export { default as SyEventRow } from "./components/event-row/SyEventRow.vue";
export { default as SyStoryRow } from "./components/story-row/SyStoryRow.vue";
export { default as SyAutomationCard } from "./components/automation-card/SyAutomationCard.vue";
export { default as SyRoomTile } from "./components/room-tile/SyRoomTile.vue";
export { default as SyAmbientRoomTile } from "./components/ambient-room-tile/SyAmbientRoomTile.vue";
export { default as SyDriverPanel } from "./components/driver-panel/SyDriverPanel.vue";
export { default as SySidebar } from "./components/sidebar/SySidebar.vue";
export { default as SyTopBar } from "./components/topbar/SyTopBar.vue";
export { default as SyShell } from "./components/shell/SyShell.vue";
export { default as SySegmented } from "./components/segmented/SySegmented.vue";
export { default as SyStatTile } from "./components/stat-tile/SyStatTile.vue";
export { default as SyCommandPalette } from "./components/command-palette/SyCommandPalette.vue";
export { default as SyEntityRow } from "./components/entity-row/SyEntityRow.vue";
export { default as SyEntityToggle } from "./components/entity-controls/SyEntityToggle.vue";
export { default as SyBrightnessSlider } from "./components/entity-controls/SyBrightnessSlider.vue";
export { default as SyColorTempSlider } from "./components/entity-controls/SyColorTempSlider.vue";
export { default as SyColorPicker } from "./components/entity-controls/SyColorPicker.vue";
export { default as SySensorValue } from "./components/entity-controls/SySensorValue.vue";
export { default as SyScene } from "./components/scene/SyScene.vue";
export { default as SyCodeEditor } from "./components/code-editor/SyCodeEditor.vue";
export { default as SyFileTree } from "./components/file-tree/SyFileTree.vue";
export { default as SyCodeEditorPanel } from "./components/code-editor-panel/SyCodeEditorPanel.vue";
export { default as SyTestPanel } from "./components/code-editor-panel/SyTestPanel.vue";
