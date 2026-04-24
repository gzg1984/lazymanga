export const DEFAULT_METADATA_DISPLAY_FIELDS = [
  'title',
  'series_name',
  'scanlator_group',
  'author_name',
  'author_alias',
  'original_work',
  'event_code',
  'comic_market',
  'year',
  'karita_id'
]

export function normalizeMetadataDisplayMode(mode, manualEditorMode = '') {
  const normalized = String(mode || '').trim().toLowerCase()
  if (normalized === 'hidden' || normalized === 'none' || normalized === 'off') {
    return 'hidden'
  }
  if (normalized === 'auto') {
    return 'auto'
  }
  if (normalized === 'selected' || normalized === 'fields' || normalized === 'custom') {
    return 'selected'
  }
  const editorMode = String(manualEditorMode || '').trim().toLowerCase()
  return editorMode === 'metadata-editor' || editorMode === 'metadata' ? 'selected' : 'hidden'
}

export function parseMetadataDisplayFields(raw) {
  const values = Array.isArray(raw)
    ? raw
    : String(raw || '').split(/[\n\r,，;；\t]+/u)

  const seen = new Set()
  const result = []
  for (const value of values) {
    const key = String(value || '').trim()
    if (!key || key.startsWith('_') || seen.has(key)) {
      continue
    }
    seen.add(key)
    result.push(key)
  }
  return result
}

export function stringifyMetadataDisplayFields(raw) {
  return parseMetadataDisplayFields(raw).join(',')
}

export function resolveMetadataDisplayConfig(settings) {
  const manualEditorMode = String(settings?.manual_editor_mode || settings?.manualEditorMode || '').trim()
  const mode = normalizeMetadataDisplayMode(settings?.metadata_display_mode || settings?.metadataDisplayMode, manualEditorMode)
  const configuredFields = parseMetadataDisplayFields(settings?.metadata_display_fields || settings?.metadataDisplayFields)
  const fields = mode === 'selected'
    ? (configuredFields.length ? configuredFields : [...DEFAULT_METADATA_DISPLAY_FIELDS])
    : configuredFields

  return {
    manualEditorMode,
    mode,
    fields,
    fieldSet: new Set(fields)
  }
}

export function shouldExposeMetadataFieldByConfig(key, normalizedValue, settings) {
  const normalizedKey = String(key || '').trim()
  if (!normalizedKey || normalizedKey.startsWith('_')) {
    return false
  }
  if (normalizedKey === 'path_parts' || normalizedKey === 'source_path' || normalizedKey === 'original_name') {
    return false
  }
  if (!normalizedValue) {
    return false
  }

  const config = settings?.fieldSet ? settings : resolveMetadataDisplayConfig(settings)
  if (config.mode === 'hidden') {
    return false
  }
  if (config.mode === 'selected' && config.fieldSet.size > 0 && !config.fieldSet.has(normalizedKey)) {
    return false
  }
  return true
}

export function metadataDisplayModeLabel(mode) {
  switch (normalizeMetadataDisplayMode(mode)) {
    case 'hidden':
      return '不显示 metadata'
    case 'auto':
      return '自动显示识别到的字段'
    case 'selected':
      return '仅显示指定字段'
    default:
      return '按仓库配置显示'
  }
}
