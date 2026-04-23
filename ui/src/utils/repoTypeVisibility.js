export const MANUAL_MANGA_REPO_TYPE_KEY = 'manga-manual'

const HIDDEN_REPO_TYPE_KEYS = new Set(['manga', 'none'])

export const DEFAULT_VISIBLE_REPO_TYPE_KEY = MANUAL_MANGA_REPO_TYPE_KEY

export function isRepoTypeHidden(key) {
  return HIDDEN_REPO_TYPE_KEYS.has(String(key || '').trim().toLowerCase())
}

export function filterVisibleRepoTypes(items) {
  if (!Array.isArray(items)) return []
  return items.filter((item) => !isRepoTypeHidden(item?.key))
}
