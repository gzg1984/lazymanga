import { ref } from 'vue'

const MAX_TASKS = 30
const SPEED_SMOOTHING_ALPHA = 0.035
const SPEED_DECAY_WHEN_IDLE = 0.97
const SPEED_QUANTUM_BPS = 128 * 1024
const SPEED_UPDATE_MIN_INTERVAL_MS = 900

let taskSeq = 0

export const downloadMode = ref('auto')
export const downloadTasks = ref([])

function nextTaskId() {
  taskSeq += 1
  return `download-${Date.now()}-${taskSeq}`
}

function isEmbeddedEnvironment() {
  if (typeof window === 'undefined' || typeof navigator === 'undefined') {
    return false
  }

  const ua = String(navigator.userAgent || '').toLowerCase()
  const embeddedHints = [
    'lzc',
    'lazycat',
    'electron',
    'webview',
    'micromessenger',
    'dingtalk',
    'qqbrowser',
    'qq/'
  ]

  if (embeddedHints.some((hint) => ua.includes(hint))) {
    return true
  }

  if (window.__LZC__ || window.LazyCat || window.lzcBridge || window.webkit?.messageHandlers?.lzc) {
    return true
  }

  return false
}

export function isStandardBrowserEnvironment() {
  if (typeof window === 'undefined' || typeof document === 'undefined' || typeof navigator === 'undefined') {
    return false
  }

  if (isEmbeddedEnvironment()) {
    return false
  }

  const ua = String(navigator.userAgent || '').toLowerCase()
  const hasMainstreamToken = /chrome|safari|firefox|edg\//.test(ua)
  return hasMainstreamToken
}

function resolveDownloadMode() {
  if (downloadMode.value === 'browser') {
    return 'browser'
  }
  if (downloadMode.value === 'ui') {
    return 'ui'
  }
  return isStandardBrowserEnvironment() ? 'browser' : 'ui'
}

function createTask(meta) {
  return {
    id: nextTaskId(),
    sourceKey: meta.sourceKey || '',
    sourceLabel: meta.sourceLabel || '下载',
    requestUrl: meta.url,
    fallbackFileName: meta.fallbackFileName || 'download.iso',
    fileName: meta.fallbackFileName || 'download.iso',
    mode: meta.mode,
    status: 'pending',
    loadedBytes: 0,
    totalBytes: 0,
    percent: 0,
    speedBps: 0,
    message: '',
    errorMessage: '',
    createdAt: Date.now(),
    updatedAt: Date.now(),
    canCancel: false,
    canRemove: false,
    abortController: null
  }
}

function updateTask(task, patch) {
  Object.assign(task, patch)
  task.updatedAt = Date.now()
  // Force reactive update in environments where nested object mutations
  // inside ref(array) are not reliably observed.
  downloadTasks.value = [...downloadTasks.value]
}

function pushTask(task) {
  downloadTasks.value = [task, ...downloadTasks.value].slice(0, MAX_TASKS)
}

function cleanupTasks() {
  const running = []
  const finished = []
  for (const task of downloadTasks.value) {
    if (task.status === 'running' || task.status === 'pending') {
      running.push(task)
    } else {
      finished.push(task)
    }
  }
  downloadTasks.value = [...running, ...finished.slice(0, MAX_TASKS - running.length)]
}

function buildProgressMessage(loadedBytes, totalBytes, speedBps) {
  const speedText = speedBps > 0 ? `, ${formatSpeed(speedBps)}` : ''
  if (totalBytes > 0) {
    const percent = Math.round((loadedBytes / totalBytes) * 100)
    return `下载中 ${percent}% (${formatBytes(loadedBytes)} / ${formatBytes(totalBytes)}${speedText})`
  }
  return `下载中 ${formatBytes(loadedBytes)}${speedText}`
}

function quantizeSpeed(speedBps) {
  const speed = Number(speedBps || 0)
  if (!Number.isFinite(speed) || speed <= 0) {
    return 0
  }

  if (speed < SPEED_QUANTUM_BPS) {
    return Math.round(speed)
  }

  return Math.round(speed / SPEED_QUANTUM_BPS) * SPEED_QUANTUM_BPS
}

function triggerBrowserDownload(url, fallbackFileName) {
  const a = document.createElement('a')
  a.href = url
  a.download = fallbackFileName || 'download.iso'
  a.rel = 'noopener'
  document.body.appendChild(a)
  a.click()
  a.remove()
}

function triggerBlobDownload(blob, fileName) {
  const blobUrl = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = blobUrl
  a.download = fileName || 'download.iso'
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(blobUrl)
}

async function parseErrorMessage(res, fallback) {
  try {
    const data = await res.clone().json()
    if (data && data.error) {
      return `${fallback}: ${data.error}`
    }
  } catch (_) {
    // ignore json parse failures
  }

  try {
    const text = await res.text()
    if (text) {
      return `${fallback}: ${text}`
    }
  } catch (_) {
    // ignore text parse failures
  }

  return `${fallback} (HTTP ${res.status})`
}

function parseDownloadFileName(contentDisposition, fallbackName = 'download.iso') {
  if (!contentDisposition) {
    return fallbackName
  }

  const utf8Match = /filename\*=UTF-8''(.+)$/.exec(contentDisposition)
  if (utf8Match && utf8Match[1]) {
    try {
      return decodeURIComponent(utf8Match[1])
    } catch (_) {
      return utf8Match[1]
    }
  }

  const normalMatch = /filename="?([^";]+)"?/.exec(contentDisposition)
  if (normalMatch && normalMatch[1]) {
    return normalMatch[1]
  }

  return fallbackName
}

function parseContentLength(lengthHeader) {
  const n = Number(lengthHeader)
  if (!Number.isFinite(n) || n <= 0) {
    return 0
  }
  return n
}

export function findLatestTaskBySource(sourceKey) {
  if (!sourceKey) {
    return null
  }
  return downloadTasks.value.find((task) => task.sourceKey === sourceKey) || null
}

export function isTaskBusy(task) {
  if (!task) {
    return false
  }
  return task.status === 'pending' || task.status === 'running' || task.status === 'delegated'
}

export function formatBytes(bytes) {
  const size = Number(bytes || 0)
  if (!Number.isFinite(size) || size < 0) {
    return '0 B'
  }

  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = size
  let unitIndex = 0
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex += 1
  }

  if (unitIndex === 0) {
    return `${Math.round(value)} ${units[unitIndex]}`
  }
  return `${value.toFixed(2)} ${units[unitIndex]}`
}

export function formatSpeed(speedBps) {
  const speed = Number(speedBps || 0)
  if (!Number.isFinite(speed) || speed <= 0) {
    return '0 B/s'
  }

  if (speed < 1024) {
    return `${Math.round(speed)} B/s`
  }
  if (speed < 1024 * 1024) {
    return `${Math.round(speed / 1024)} KB/s`
  }
  if (speed < 1024 * 1024 * 1024) {
    return `${(speed / (1024 * 1024)).toFixed(1)} MB/s`
  }
  return `${(speed / (1024 * 1024 * 1024)).toFixed(2)} GB/s`
}

export function getTaskHint(task) {
  if (!task) {
    return ''
  }

  if (task.status === 'running') {
    return buildProgressMessage(task.loadedBytes, task.totalBytes, Number(task.speedBps || 0))
  }

  if (task.status === 'delegated') {
    return '已交给浏览器下载管理'
  }

  if (task.status === 'completed') {
    return '下载完成'
  }

  if (task.status === 'failed') {
    return task.errorMessage || '下载失败'
  }

  if (task.status === 'canceled') {
    return '已取消'
  }

  return ''
}

export function cancelTask(taskId) {
  const task = downloadTasks.value.find((item) => item.id === taskId)
  if (!task) {
    return
  }

  if (task.abortController) {
    task.abortController.abort()
  }
}

export function clearFinishedTasks() {
  downloadTasks.value = downloadTasks.value.filter((task) => task.status === 'running' || task.status === 'pending')
}

export function removeTask(taskId) {
  downloadTasks.value = downloadTasks.value.filter((task) => task.id !== taskId)
}

export async function startManagedDownload(options) {
  const mode = resolveDownloadMode()
  const task = createTask({
    sourceKey: options.sourceKey,
    sourceLabel: options.sourceLabel,
    url: options.url,
    fallbackFileName: options.fallbackFileName,
    mode
  })
  pushTask(task)

  if (mode === 'browser') {
    triggerBrowserDownload(options.url, options.fallbackFileName)
    updateTask(task, {
      status: 'delegated',
      canCancel: false,
      canRemove: true,
      message: '已交给浏览器下载管理'
    })
    cleanupTasks()
    return { ok: true, task }
  }

  const abortController = new AbortController()
  updateTask(task, { status: 'running', canCancel: true, abortController })

  try {
    const result = await new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest()
      xhr.open(options.method || 'GET', options.url, true)
      xhr.responseType = 'blob'

      if (options.headers && typeof options.headers === 'object') {
        Object.entries(options.headers).forEach(([k, v]) => {
          if (v !== undefined && v !== null) {
            xhr.setRequestHeader(String(k), String(v))
          }
        })
      }

      let lastLoaded = 0
      let lastAt = Date.now()
      let smoothSpeedBps = 0
      let lastSpeedEmitAt = 0
      let lastEmittedSpeedBps = 0

      xhr.onprogress = (event) => {
        const loadedBytes = Number(event.loaded || 0)
        const totalBytes = event.lengthComputable ? Number(event.total || 0) : 0
        const now = Date.now()
        const deltaBytes = Math.max(0, loadedBytes - lastLoaded)
        const deltaMs = Math.max(1, now - lastAt)
        const instantSpeedBps = deltaBytes > 0 ? (deltaBytes * 1000) / deltaMs : 0

        if (instantSpeedBps > 0) {
          if (smoothSpeedBps <= 0) {
            smoothSpeedBps = instantSpeedBps
          } else {
            smoothSpeedBps = smoothSpeedBps * (1 - SPEED_SMOOTHING_ALPHA) + instantSpeedBps * SPEED_SMOOTHING_ALPHA
          }
        } else if (smoothSpeedBps > 0) {
          smoothSpeedBps *= SPEED_DECAY_WHEN_IDLE
          if (smoothSpeedBps < 1) {
            smoothSpeedBps = 0
          }
        }

        const quantizedSpeedBps = quantizeSpeed(smoothSpeedBps)
        const shouldUpdateSpeed = (now - lastSpeedEmitAt) >= SPEED_UPDATE_MIN_INTERVAL_MS
        if (shouldUpdateSpeed) {
          lastEmittedSpeedBps = quantizedSpeedBps
          lastSpeedEmitAt = now
        }

        lastLoaded = loadedBytes
        lastAt = now

        updateTask(task, {
          loadedBytes,
          totalBytes,
          speedBps: lastEmittedSpeedBps,
          percent: totalBytes > 0 ? (loadedBytes / totalBytes) * 100 : 0,
          message: buildProgressMessage(loadedBytes, totalBytes, lastEmittedSpeedBps)
        })
      }

      xhr.onload = () => {
        if (xhr.status < 200 || xhr.status >= 300) {
          const errText = String(xhr.responseText || '').trim()
          reject(new Error(errText ? `下载失败: ${errText}` : `下载失败 (HTTP ${xhr.status})`))
          return
        }

        const contentDisposition = xhr.getResponseHeader('Content-Disposition')
        const contentLength = parseContentLength(xhr.getResponseHeader('Content-Length'))
        const fileName = parseDownloadFileName(contentDisposition, options.fallbackFileName || 'download.iso')
        const blob = xhr.response
        const loadedBytes = Number(blob?.size || task.loadedBytes || 0)
        const totalBytes = contentLength || loadedBytes

        resolve({ fileName, blob, loadedBytes, totalBytes })
      }

      xhr.onerror = () => {
        reject(new Error('下载失败：网络异常'))
      }

      xhr.onabort = () => {
        const abortError = new Error('下载已取消')
        abortError.name = 'AbortError'
        reject(abortError)
      }

      abortController.signal.addEventListener('abort', () => xhr.abort(), { once: true })
      xhr.send()
    })

    triggerBlobDownload(result.blob, result.fileName)
    updateTask(task, {
      fileName: result.fileName,
      status: 'completed',
      loadedBytes: result.loadedBytes,
      totalBytes: result.totalBytes,
      speedBps: 0,
      percent: 100,
      canCancel: false,
      canRemove: true,
      abortController: null,
      message: '下载完成'
    })

    cleanupTasks()
    return { ok: true, task }
  } catch (e) {
    if (e?.name === 'AbortError') {
      updateTask(task, {
        status: 'canceled',
        canCancel: false,
        canRemove: true,
        abortController: null,
        errorMessage: '下载已取消'
      })
      cleanupTasks()
      return { ok: false, task }
    }

    updateTask(task, {
      status: 'failed',
      canCancel: false,
      canRemove: true,
      abortController: null,
      errorMessage: e?.message || '下载失败'
    })
    cleanupTasks()
    return { ok: false, task }
  }
}
