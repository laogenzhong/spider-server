<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  ArrowDown, ArrowUp, Copy, Download, FilePlus2, GripVertical, ImageOff,
  ListPlus, RefreshCw, RotateCcw, Search, Trash2, UploadCloud,
} from 'lucide-vue-next'

const emit = defineEmits(['notify'])

const locales = [
  { id: 'zh-Hans', label: '简体中文' },
  { id: 'zh-Hant', label: '繁體中文' },
  { id: 'en', label: 'English' },
  { id: 'ja', label: '日本語' },
  { id: 'ko', label: '한국어' },
]
const categories = [
  ['exercise_category_chest', '胸'], ['exercise_category_back', '背'], ['exercise_category_shoulder', '肩'],
  ['exercise_category_upper_arm', '上臂'], ['exercise_category_forearm', '前臂'], ['exercise_category_core', '核心'],
  ['exercise_category_thigh', '大腿'], ['exercise_category_calf', '小腿'], ['exercise_category_neck', '颈部'],
  ['exercise_category_cardio', '有氧'],
]
const types = ref([])
const typeDefinitions = ref([])
const sourceTypeDefinitions = ref([])
const subcategories = {
  exercise_category_chest: [['exercise_chest_upper', '上胸'], ['exercise_chest_middle', '中胸'], ['exercise_chest_lower', '下胸']],
  exercise_category_shoulder: [['exercise_shoulder_front', '前束'], ['exercise_shoulder_middle', '中束'], ['exercise_shoulder_rear', '后束']],
  exercise_category_upper_arm: [['watch_tag_biceps', '肱二头肌'], ['watch_tag_triceps', '肱三头肌']],
  exercise_category_thigh: [['watch_tag_legs', '腿部'], ['watch_tag_glutes', '臀部']],
}
const categoryLabel = Object.fromEntries(categories)
const subcategoryLabel = Object.fromEntries(Object.values(subcategories).flat())
const typeLabel = computed(() => Object.fromEntries(types.value))
const sourceTypeByKey = computed(() => Object.fromEntries(sourceTypeDefinitions.value.map((item) => [item.key, item])))

const actions = ref([])
const selectedEditorID = ref('')
const editingLocale = ref('zh-Hans')
const query = ref('')
const categoryFilter = ref('all')
const subcategoryFilter = ref('all')
const typeFilter = ref('all')
const statusFilter = ref('all')
const loading = ref(false)
const loadError = ref('')
const dirty = ref(false)
const sourceGeneratedAt = ref('')
const previewFailed = ref(false)
const draggedEditorID = ref('')
const editorPanel = ref(null)
const workspacePanel = ref(null)
const sidebarWidth = ref(360)
const resizingWorkspace = ref(false)
const syncingClient = ref(false)
let stopWorkspaceResize = null

function editorID() {
  return globalThis.crypto?.randomUUID?.() || `editor-${Date.now()}-${Math.random()}`
}

function localeStrings(value, fallback = '') {
  return Object.fromEntries(locales.map(({ id }) => [id, String(value?.[id] ?? (id === 'zh-Hans' ? value?.zh : undefined) ?? fallback)]))
}

function localeLists(value) {
  return Object.fromEntries(locales.map(({ id }) => {
    const raw = value?.[id] ?? (id === 'zh-Hans' ? value?.zh : undefined)
    return [id, Array.isArray(raw) ? raw.map((item) => String(item).trim()).filter(Boolean) : []]
  }))
}

function normalizedAction(raw, index = 0) {
  const fallbackName = String(raw.fallbackName || raw.names?.en || raw.gifName || '').trim()
  return {
    _editorID: raw._editorID || editorID(),
    enabled: raw.enabled !== false,
    sortOrder: Number.isInteger(Number(raw.sortOrder)) ? Number(raw.sortOrder) : index,
    gifName: String(raw.gifName || '').trim(),
    nameKey: String(raw.nameKey || '').trim(),
    sourceExerciseID: String(raw.sourceExerciseID || '').trim(),
    sourceName: String(raw.sourceName || '').trim(),
    categoryKey: String(raw.categoryKey || '').trim(),
    subcategoryKey: String(raw.subcategoryKey || '').trim(),
    typeKey: String(raw.typeKey || '').trim(),
    sourceGif: String(raw.sourceGif || raw.gifRelativePath || '').trim().replaceAll('\\', '/'),
    gifRelativePath: String(raw.gifRelativePath || '').trim().replaceAll('\\', '/'),
    names: localeStrings(raw.names, fallbackName),
    libraryNames: localeStrings(raw.libraryNames, fallbackName),
    descriptions: localeStrings(raw.descriptions),
    instructions: localeLists(raw.instructions),
    targetMuscles: localeLists(raw.targetMuscles),
    secondaryMuscles: localeLists(raw.secondaryMuscles),
  }
}

function blankAction() {
  const timestamp = Date.now()
  return normalizedAction({
    enabled: true,
    sortOrder: actions.value.length,
    gifName: `new-action-${timestamp}`,
    nameKey: `exercise_import_new_action_${timestamp}`,
    categoryKey: 'exercise_category_chest',
    subcategoryKey: 'exercise_chest_middle',
    typeKey: types.value.some(([key]) => key === 'exercise_type_dumbbell') ? 'exercise_type_dumbbell' : (types.value[0]?.[0] || ''),
    sourceGif: `incoming/new-action-${timestamp}.gif`,
    gifRelativePath: `chest/dumbbell/new-action-${timestamp}.gif`,
    names: Object.fromEntries(locales.map(({ id }) => [id, id === 'en' ? 'New Exercise' : '新动作'])),
    libraryNames: Object.fromEntries(locales.map(({ id }) => [id, id === 'en' ? 'New Exercise' : '新动作'])),
    descriptions: {}, instructions: {}, targetMuscles: {}, secondaryMuscles: {},
  })
}

const orderedActions = computed(() => [...actions.value].sort((left, right) => left.sortOrder - right.sortOrder || left.gifName.localeCompare(right.gifName, 'en')))
const availableSubcategoryFilters = computed(() => {
  const availableKeys = new Set(orderedActions.value
    .filter((action) => categoryFilter.value === 'all' || action.categoryKey === categoryFilter.value)
    .map((action) => action.subcategoryKey)
    .filter(Boolean))
  const configuredOptions = Object.values(subcategories).flat().filter(([key]) => availableKeys.delete(key))
  const unknownOptions = [...availableKeys].sort().map((key) => [key, subcategoryLabel[key] || key])
  return [...configuredOptions, ...unknownOptions]
})
const filteredActions = computed(() => {
  const keyword = query.value.trim().toLocaleLowerCase()
  return orderedActions.value.filter((action) => {
    if (categoryFilter.value !== 'all' && action.categoryKey !== categoryFilter.value) return false
    if (subcategoryFilter.value !== 'all' && action.subcategoryKey !== subcategoryFilter.value) return false
    if (typeFilter.value !== 'all' && actionDisplayTypeKey(action) !== typeFilter.value) return false
    if (statusFilter.value === 'enabled' && !action.enabled) return false
    if (statusFilter.value === 'disabled' && action.enabled) return false
    if (!keyword) return true
    return [action.gifName, action.nameKey, action.sourceName, action.names['zh-Hans'], action.names.en, action.gifRelativePath]
      .some((value) => String(value || '').toLocaleLowerCase().includes(keyword))
  })
})
const selectedAction = computed(() => actions.value.find((action) => action._editorID === selectedEditorID.value) || null)
const selectedSubcategories = computed(() => subcategories[selectedAction.value?.categoryKey] || [])
const selectedDisplayTypeKey = computed(() => actionDisplayTypeKey(selectedAction.value))
const selectedPosition = computed(() => orderedActions.value.findIndex((action) => action._editorID === selectedEditorID.value))

watch(categoryFilter, () => { subcategoryFilter.value = 'all' })

function exportedAction(action, index) {
  return {
    enabled: action.enabled,
    sortOrder: index,
    gifName: action.gifName.trim(),
    nameKey: action.nameKey.trim(),
    fallbackName: action.names.en.trim(),
    sourceExerciseID: action.sourceExerciseID.trim(),
    sourceName: action.sourceName.trim(),
    categoryKey: action.categoryKey,
    subcategoryKey: action.subcategoryKey,
    typeKey: action.typeKey,
    displayTypeKey: actionDisplayTypeKey(action),
    typeNames: localeStrings(typeDefinitions.value.find((item) => item.key === actionDisplayTypeKey(action))?.names),
    sourceGif: action.sourceGif.trim().replaceAll('\\', '/'),
    gifRelativePath: action.gifRelativePath.trim().replaceAll('\\', '/'),
    names: localeStrings(action.names),
    libraryNames: localeStrings(action.libraryNames),
    descriptions: localeStrings(action.descriptions),
    instructions: localeLists(action.instructions),
    targetMuscles: localeLists(action.targetMuscles),
    secondaryMuscles: localeLists(action.secondaryMuscles),
  }
}

const generatedPackage = computed(() => ({
  schemaVersion: 1,
  mode: 'full',
  generatedAt: new Date().toISOString(),
  taxonomy: { types: typeDefinitions.value, sourceTypes: sourceTypeDefinitions.value },
  actions: orderedActions.value.map(exportedAction),
}))

function actionName(action) {
  return action.libraryNames?.[editingLocale.value] || action.names?.[editingLocale.value] || action.names?.en || action.gifName
}

function actionDisplayTypeKey(action) {
  const typeKey = String(action?.typeKey || '')
  return sourceTypeByKey.value[typeKey]?.displayKey || typeKey
}

function updateSelectedDisplayType(value) {
  if (!selectedAction.value) return
  selectedAction.value.typeKey = value
  markDirty()
}

function actionGIFURL(action) {
  const path = String(action?.gifRelativePath || '').trim()
  if (!path) return ''
  return `/exercise-gifs/${path.split('/').map((segment) => encodeURIComponent(segment)).join('/')}`
}

function safeGIFPath(value) {
  const path = String(value || '').trim().replaceAll('\\', '/')
  const parts = path.split('/')
  return Boolean(path) && !path.startsWith('/') && path.toLocaleLowerCase().endsWith('.gif')
    && !parts.some((part) => !part || part === '.' || part === '..')
}

function markDirty() {
  dirty.value = true
}

function resetWorkspaceWidth() {
  sidebarWidth.value = clampedSidebarWidth(360)
  localStorage.setItem('spider-exercise-library-sidebar-width', String(sidebarWidth.value))
  emit('notify', '已恢复动作库默认布局')
}

function clampedSidebarWidth(value) {
  const workspaceWidth = workspacePanel.value?.getBoundingClientRect().width || 930
  return Math.round(Math.min(Math.max(280, workspaceWidth - 570), Math.max(280, value)))
}

function handleWorkspaceWindowResize() {
  if (window.matchMedia('(max-width: 900px)').matches) return
  sidebarWidth.value = clampedSidebarWidth(sidebarWidth.value)
}

function beginWorkspaceResize(event) {
  if (!workspacePanel.value || window.matchMedia('(max-width: 900px)').matches) return
  event.preventDefault()
  const startX = event.clientX
  const startWidth = sidebarWidth.value
  resizingWorkspace.value = true
  document.documentElement.style.cursor = 'col-resize'
  document.documentElement.style.userSelect = 'none'

  const handleMove = (moveEvent) => {
    sidebarWidth.value = clampedSidebarWidth(startWidth + moveEvent.clientX - startX)
  }
  const handleEnd = () => {
    window.removeEventListener('pointermove', handleMove)
    window.removeEventListener('pointerup', handleEnd)
    window.removeEventListener('pointercancel', handleEnd)
    document.documentElement.style.cursor = ''
    document.documentElement.style.userSelect = ''
    resizingWorkspace.value = false
    localStorage.setItem('spider-exercise-library-sidebar-width', String(sidebarWidth.value))
    stopWorkspaceResize = null
  }
  stopWorkspaceResize = handleEnd
  window.addEventListener('pointermove', handleMove)
  window.addEventListener('pointerup', handleEnd)
  window.addEventListener('pointercancel', handleEnd)
}

function normalizeSortOrders() {
  orderedActions.value.forEach((action, index) => { action.sortOrder = index })
}

function applySortOrder() {
  if (!selectedAction.value) return
  const target = Math.max(0, Math.min(actions.value.length - 1, Number(selectedAction.value.sortOrder) || 0))
  const current = selectedPosition.value
  const ordered = orderedActions.value
  const [action] = ordered.splice(current, 1)
  ordered.splice(target, 0, action)
  ordered.forEach((item, index) => { item.sortOrder = index })
  actions.value = ordered
  markDirty()
}

function applyDisplaySortOrder(value) {
  if (!selectedAction.value) return
  selectedAction.value.sortOrder = Math.max(0, Number(value || 1) - 1)
  applySortOrder()
}

function moveAction(editorIDValue, offset) {
  const ordered = orderedActions.value
  const current = ordered.findIndex((action) => action._editorID === editorIDValue)
  const target = current + offset
  if (current < 0 || target < 0 || target >= ordered.length) return
  const [action] = ordered.splice(current, 1)
  ordered.splice(target, 0, action)
  ordered.forEach((item, index) => { item.sortOrder = index })
  actions.value = ordered
  markDirty()
}

function dropAction(targetEditorID) {
  const sourceEditorID = draggedEditorID.value
  draggedEditorID.value = ''
  if (!sourceEditorID || sourceEditorID === targetEditorID) return
  const ordered = orderedActions.value
  const sourceIndex = ordered.findIndex((action) => action._editorID === sourceEditorID)
  const targetIndex = ordered.findIndex((action) => action._editorID === targetEditorID)
  if (sourceIndex < 0 || targetIndex < 0) return
  const [action] = ordered.splice(sourceIndex, 1)
  ordered.splice(targetIndex, 0, action)
  ordered.forEach((item, index) => { item.sortOrder = index })
  actions.value = ordered
  markDirty()
}

function updateCategory() {
  const options = subcategories[selectedAction.value?.categoryKey] || []
  if (selectedAction.value && !options.some(([key]) => key === selectedAction.value.subcategoryKey)) {
    selectedAction.value.subcategoryKey = options[0]?.[0] || ''
  }
  markDirty()
}

function setMuscleList(field, locale, value) {
  if (!selectedAction.value) return
  selectedAction.value[field][locale] = String(value).split(/[,，、\n]/).map((item) => item.trim()).filter(Boolean)
  markDirty()
}

function addInstruction(locale) {
  selectedAction.value?.instructions[locale].push('')
  markDirty()
}

function removeInstruction(locale, index) {
  selectedAction.value?.instructions[locale].splice(index, 1)
  markDirty()
}

function moveInstruction(locale, index, offset) {
  const items = selectedAction.value?.instructions[locale]
  const target = index + offset
  if (!items || target < 0 || target >= items.length) return
  const [item] = items.splice(index, 1)
  items.splice(target, 0, item)
  markDirty()
}

function selectAction(action) {
  selectedEditorID.value = action._editorID
  previewFailed.value = false
}

async function addAction() {
  normalizeSortOrders()
  const action = blankAction()
  actions.value.push(action)
  selectedEditorID.value = action._editorID
  editingLocale.value = 'zh-Hans'
  markDirty()
  await nextTick()
  editorPanel.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

async function duplicateSelectedAction() {
  if (!selectedAction.value) return
  const copy = normalizedAction(JSON.parse(JSON.stringify(selectedAction.value)), actions.value.length)
  copy._editorID = editorID()
  copy.gifName = `${copy.gifName}-copy`
  copy.nameKey = `${copy.nameKey}_copy`
  copy.sourceGif = copy.sourceGif.replace(/\.gif$/i, '-copy.gif')
  copy.gifRelativePath = copy.gifRelativePath.replace(/\.gif$/i, '-copy.gif')
  copy.sortOrder = selectedPosition.value + 1
  actions.value.forEach((action) => { if (action.sortOrder >= copy.sortOrder) action.sortOrder += 1 })
  actions.value.push(copy)
  selectedEditorID.value = copy._editorID
  markDirty()
  await nextTick()
  editorPanel.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function removeSelectedAction() {
  const action = selectedAction.value
  if (!action || !window.confirm(`确定从本次全量配置中移除“${actionName(action)}”？`)) return
  const index = actions.value.findIndex((item) => item._editorID === action._editorID)
  actions.value.splice(index, 1)
  normalizeSortOrders()
  selectedEditorID.value = orderedActions.value[Math.min(index, actions.value.length - 1)]?._editorID || ''
  markDirty()
}

function validatePackage() {
  const errors = []
  const ids = new Set()
  const paths = new Set()
  generatedPackage.value.actions.forEach((action, index) => {
    const title = `第 ${index + 1} 条${action.gifName ? `（${action.gifName}）` : ''}`
    for (const key of ['gifName', 'nameKey', 'categoryKey', 'typeKey', 'sourceGif', 'gifRelativePath']) {
      if (!String(action[key] || '').trim()) errors.push(`${title}的 ${key} 不能为空`)
    }
    for (const { id, label } of locales) {
      if (!action.names[id]?.trim()) errors.push(`${title}缺少${label}名称`)
      if (!action.libraryNames[id]?.trim()) errors.push(`${title}缺少${label}卡片短名称`)
      if (!action.instructions[id]?.some((item) => item.trim())) errors.push(`${title}缺少${label}动作步骤`)
    }
    if (!categories.some(([key]) => key === action.categoryKey)) errors.push(`${title}的训练部位无效`)
    if (!sourceTypeByKey.value[action.typeKey]) errors.push(`${title}的内部动作类型无效`)
    if (!types.value.some(([key]) => key === actionDisplayTypeKey(action))) errors.push(`${title}的客户端器械类型无效`)
    if (!safeGIFPath(action.sourceGif)) errors.push(`${title}的素材路径必须是安全的相对 GIF 路径`)
    if (!safeGIFPath(action.gifRelativePath)) errors.push(`${title}的项目路径必须是安全的相对 GIF 路径`)
    if (ids.has(action.gifName)) errors.push(`动作 ID 重复：${action.gifName}`)
    if (paths.has(action.gifRelativePath)) errors.push(`项目 GIF 路径重复：${action.gifRelativePath}`)
    ids.add(action.gifName)
    paths.add(action.gifRelativePath)
  })
  return errors
}

async function loadCurrentCatalog() {
  loading.value = true
  loadError.value = ''
  try {
    const response = await fetch('/exercise-catalog.json', { cache: 'no-store' })
    if (!response.ok) throw new Error(`读取客户端配置失败（${response.status}）`)
    const catalog = await response.json()
    if (catalog.schemaVersion !== 1 || catalog.mode !== 'full' || !Array.isArray(catalog.actions) || !catalog.actions.length) {
      throw new Error('客户端配置快照格式不正确')
    }
    actions.value = catalog.actions.map(normalizedAction)
    if (!Array.isArray(catalog.taxonomy?.types) || !catalog.taxonomy.types.length) {
      throw new Error('客户端配置未提供器械类型定义')
    }
    typeDefinitions.value = catalog.taxonomy.types.map((item) => ({
      key: String(item.key || ''),
      names: localeStrings(item.names, item.key),
    })).filter((item) => item.key)
    const rawSourceTypes = Array.isArray(catalog.taxonomy.sourceTypes) && catalog.taxonomy.sourceTypes.length
      ? catalog.taxonomy.sourceTypes
      : catalog.taxonomy.types
    sourceTypeDefinitions.value = rawSourceTypes.map((item) => ({
      key: String(item.key || ''),
      names: localeStrings(item.names, item.key),
      displayKey: String(item.displayKey || item.key || ''),
      displayNames: localeStrings(item.displayNames || item.names, item.displayKey || item.key),
    })).filter((item) => item.key)
    types.value = typeDefinitions.value.map((item) => [item.key, item.names['zh-Hans'] || item.names.en || item.key])
    normalizeSortOrders()
    selectedEditorID.value = orderedActions.value[0]?._editorID || ''
    sourceGeneratedAt.value = catalog.generatedAt || ''
    dirty.value = false
    previewFailed.value = false
    emit('notify', `已读取客户端当前全量配置，共 ${actions.value.length} 个动作`)
  } catch (error) {
    loadError.value = error.message || '读取客户端配置失败'
  } finally {
    loading.value = false
  }
}

function downloadCurrentPackage() {
  normalizeSortOrders()
  const errors = validatePackage()
  if (errors.length) {
    emit('notify', `无法导出：${errors.slice(0, 3).join('；')}`, 'error')
    return
  }
  const blob = new Blob([`${JSON.stringify(generatedPackage.value, null, 2)}\n`], { type: 'application/json;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = 'exercise-library-catalog.json'
  link.click()
  URL.revokeObjectURL(url)
  dirty.value = false
  emit('notify', `已导出 ${actions.value.length} 个动作的完整配置`)
}

async function syncToClient() {
  normalizeSortOrders()
  const errors = validatePackage()
  if (errors.length) {
    emit('notify', `更新前请修复：${errors.slice(0, 3).join('；')}`, 'error')
    return
  }
  if (!window.confirm(`确定用当前完整配置更新客户端的 ${actions.value.length} 个动作？`)) return
  syncingClient.value = true
  try {
    const response = await fetch('/local-client-sync/exercise-library', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(generatedPackage.value),
    })
    const text = await response.text()
    let result
    try { result = JSON.parse(text) } catch { throw new Error('本地客户端同步服务不可用，请通过 npm run dev 或 npm run preview 打开后台') }
    if (!response.ok || result.code !== 0) throw new Error(result.message || '客户端动作库更新失败')
    dirty.value = false
    sourceGeneratedAt.value = generatedPackage.value.generatedAt
    emit('notify', `客户端动作库已更新，共 ${actions.value.length} 个动作；重新构建客户端后生效`)
  } catch (error) {
    emit('notify', error.message || '客户端动作库更新失败', 'error')
  } finally {
    syncingClient.value = false
  }
}

onMounted(() => {
  const savedWidth = Number(localStorage.getItem('spider-exercise-library-sidebar-width'))
  if (Number.isFinite(savedWidth) && savedWidth >= 280) sidebarWidth.value = savedWidth
  nextTick(handleWorkspaceWindowResize)
  window.addEventListener('resize', handleWorkspaceWindowResize)
  loadCurrentCatalog()
})
onBeforeUnmount(() => {
  stopWorkspaceResize?.()
  window.removeEventListener('resize', handleWorkspaceWindowResize)
})
</script>

<template>
  <section class="exercise-catalog-tool">
    <div class="tool-header">
      <div>
        <h2>动作库配置</h2>
        <p>管理动作素材、五语名称、动作介绍与步骤、肌群和客户端展示顺序。每次导出都是完整动作目录。</p>
      </div>
      <div class="tool-actions">
        <button class="secondary-button" type="button" :disabled="loading" @click="loadCurrentCatalog"><RefreshCw :size="16" :class="{ spinning: loading }" />重新读取客户端</button>
        <button class="secondary-button" type="button" @click="resetWorkspaceWidth"><RotateCcw :size="16" />恢复布局</button>
        <button class="secondary-button" type="button" :disabled="loading || !types.length" @click="addAction"><FilePlus2 :size="16" />添加动作</button>
        <button class="primary-button" type="button" :disabled="!actions.length" @click="downloadCurrentPackage"><Download :size="16" />导出完整 JSON</button>
        <button class="primary-button" type="button" :disabled="!actions.length || syncingClient" @click="syncToClient"><UploadCloud :size="16" :class="{ spinning: syncingClient }" />{{ syncingClient ? '正在更新…' : '更新客户端' }}</button>
      </div>
    </div>

    <div class="workflow-note">
      <span><strong>{{ actions.length }}</strong> 个动作</span>
      <span><strong>{{ actions.filter((item) => item.enabled).length }}</strong> 个已启用</span>
      <span v-if="dirty" class="dirty-state">配置已修改，完整 JSON 已实时更新</span>
      <span v-else-if="sourceGeneratedAt">客户端快照：{{ new Date(sourceGeneratedAt).toLocaleString() }}</span>
    </div>
    <p v-if="loadError" class="load-error">{{ loadError }}</p>

    <div ref="workspacePanel" class="catalog-workspace" :class="{ resizing: resizingWorkspace }" :style="{ '--catalog-sidebar-width': `${sidebarWidth}px` }">
      <aside class="catalog-sidebar">
        <div class="filters">
          <label class="search-field"><Search :size="16" /><input v-model="query" placeholder="搜索名称、ID、来源或路径" /></label>
          <div class="filter-row">
            <select v-model="categoryFilter"><option value="all">全部部位</option><option v-for="[key, label] in categories" :key="key" :value="key">{{ label }}</option></select>
            <select v-model="subcategoryFilter" :disabled="!availableSubcategoryFilters.length"><option value="all">{{ availableSubcategoryFilters.length ? '全部子部位' : '无子部位' }}</option><option v-for="[key, label] in availableSubcategoryFilters" :key="key" :value="key">{{ label }}</option></select>
            <select v-model="typeFilter"><option value="all">全部器械</option><option v-for="[key, label] in types" :key="key" :value="key">{{ label }}</option></select>
            <select v-model="statusFilter"><option value="all">全部状态</option><option value="enabled">已启用</option><option value="disabled">已停用</option></select>
          </div>
          <div class="sidebar-summary"><span>显示 {{ filteredActions.length }} 个</span><span>拖动排序 · 左→右</span></div>
        </div>

        <div class="action-list">
          <button
            v-for="action in filteredActions"
            :key="action._editorID"
            class="action-row"
            :class="{ active: action._editorID === selectedEditorID, disabled: !action.enabled, dragging: action._editorID === draggedEditorID }"
            type="button"
            draggable="true"
            @dragstart="draggedEditorID = action._editorID"
            @dragend="draggedEditorID = ''"
            @dragover.prevent
            @drop="dropAction(action._editorID)"
            @click="selectAction(action)"
          >
            <GripVertical class="drag-handle" :size="15" />
            <span class="order-badge">{{ action.sortOrder + 1 }}</span>
            <span class="list-gif"><span>GIF</span><img :src="actionGIFURL(action)" loading="lazy" alt="" /></span>
            <span class="action-row-copy"><strong>{{ actionName(action) }}</strong><small>{{ categoryLabel[action.categoryKey] }} · {{ typeLabel[actionDisplayTypeKey(action)] }}</small><em>{{ action.gifName }}</em></span>
          </button>
        </div>
      </aside>

      <button class="workspace-resizer" type="button" title="拖动调整左右宽度，双击恢复默认" aria-label="调整动作列表和编辑区宽度" @pointerdown="beginWorkspaceResize" @dblclick="resetWorkspaceWidth"><span /></button>

      <main v-if="selectedAction" ref="editorPanel" class="action-editor">
        <div class="editor-heading">
          <div><span class="eyebrow">动作 {{ selectedPosition + 1 }} / {{ actions.length }}</span><h3>{{ actionName(selectedAction) }}</h3><p>{{ selectedAction.gifName }}</p></div>
          <div class="editor-actions">
            <label class="status-switch"><input v-model="selectedAction.enabled" type="checkbox" @change="markDirty" /><span>{{ selectedAction.enabled ? '已启用' : '已停用' }}</span></label>
            <button class="icon-text-button" type="button" @click="duplicateSelectedAction"><Copy :size="15" />复制</button>
            <button class="icon-text-button danger" type="button" @click="removeSelectedAction"><Trash2 :size="15" />删除</button>
          </div>
        </div>

        <section class="editor-section asset-section">
          <div class="gif-preview">
            <img v-if="actionGIFURL(selectedAction) && !previewFailed" :key="actionGIFURL(selectedAction)" :src="actionGIFURL(selectedAction)" alt="动作 GIF 预览" @load="previewFailed = false" @error="previewFailed = true" />
            <div v-else class="gif-empty"><ImageOff :size="32" /><span>找不到当前 GIF</span><small>检查项目 GIF 路径以及 SPIDER_CLIENT_ROOT</small></div>
          </div>
          <div class="asset-fields">
            <div class="section-title"><div><h4>素材与标识</h4><p>动作 ID 保存后应保持稳定，训练记录通过它关联动作。</p></div></div>
            <div class="form-grid two-columns">
              <label><span>动作 ID *</span><input v-model.trim="selectedAction.gifName" @input="markDirty" /></label>
              <label><span>客户端名称键 *</span><input v-model.trim="selectedAction.nameKey" @input="markDirty" /></label>
              <label><span>来源动作 ID</span><input v-model.trim="selectedAction.sourceExerciseID" @input="markDirty" /></label>
              <label><span>来源原始名称</span><input v-model.trim="selectedAction.sourceName" @input="markDirty" /></label>
              <label class="wide"><span>素材根目录中的 GIF 路径 *</span><input v-model.trim="selectedAction.sourceGif" @input="markDirty" /></label>
              <label class="wide"><span>客户端项目中的 GIF 路径 *</span><input v-model.trim="selectedAction.gifRelativePath" @input="previewFailed = false; markDirty()" /></label>
            </div>
          </div>
        </section>

        <section class="editor-section">
          <div class="section-title"><div><h4>分类与展示顺序</h4><p>器械类型与客户端动作库筛选一致，共 {{ types.length }} 类；排序数字越小越靠前。</p></div><div class="order-buttons"><button type="button" :disabled="selectedPosition <= 0" @click="moveAction(selectedAction._editorID, -1)"><ArrowUp :size="15" />前移</button><button type="button" :disabled="selectedPosition >= actions.length - 1" @click="moveAction(selectedAction._editorID, 1)"><ArrowDown :size="15" />后移</button></div></div>
          <div class="form-grid four-columns">
            <label><span>展示顺序</span><input :value="selectedAction.sortOrder + 1" type="number" min="1" :max="actions.length" @change="applyDisplaySortOrder($event.target.value)" /></label>
            <label><span>训练部位 *</span><select v-model="selectedAction.categoryKey" @change="updateCategory"><option v-for="[key, label] in categories" :key="key" :value="key">{{ label }}</option></select></label>
            <label><span>细分部位</span><select v-model="selectedAction.subcategoryKey" :disabled="!selectedSubcategories.length" @change="markDirty"><option value="">不细分</option><option v-for="[key, label] in selectedSubcategories" :key="key" :value="key">{{ label }}</option></select></label>
            <label><span>器械类型 *</span><select :value="selectedDisplayTypeKey" @change="updateSelectedDisplayType($event.target.value)"><option v-for="[key, label] in types" :key="key" :value="key">{{ label }}</option></select></label>
          </div>
        </section>

        <section class="editor-section locale-section">
          <div class="section-title"><div><h4>多语言名称与动作介绍</h4><p>五种语言分别维护；动作步骤会直接展示在客户端动作详情中。</p></div></div>
          <div class="locale-tabs"><button v-for="locale in locales" :key="locale.id" type="button" :class="{ active: editingLocale === locale.id }" @click="editingLocale = locale.id">{{ locale.label }}<span v-if="!selectedAction.names[locale.id] || !selectedAction.libraryNames[locale.id] || !selectedAction.instructions[locale.id].length" class="locale-warning">!</span></button></div>

          <div class="locale-editor">
            <div class="muscle-grid">
              <label><span>详情页完整名称 *</span><input v-model.trim="selectedAction.names[editingLocale]" @input="markDirty" /></label>
              <label><span>动作库卡片短名称 *</span><input v-model.trim="selectedAction.libraryNames[editingLocale]" @input="markDirty" /></label>
            </div>
            <label><span>动作介绍</span><textarea v-model.trim="selectedAction.descriptions[editingLocale]" rows="3" placeholder="简要介绍动作目的、特点或适用场景" @input="markDirty" /></label>
            <div class="muscle-grid">
              <label><span>目标肌群</span><textarea :value="selectedAction.targetMuscles[editingLocale].join('、')" rows="2" placeholder="多个肌群用逗号分隔" @change="setMuscleList('targetMuscles', editingLocale, $event.target.value)" /></label>
              <label><span>辅助肌群</span><textarea :value="selectedAction.secondaryMuscles[editingLocale].join('、')" rows="2" placeholder="多个肌群用逗号分隔" @change="setMuscleList('secondaryMuscles', editingLocale, $event.target.value)" /></label>
            </div>

            <div class="instruction-heading"><div><strong>动作步骤 *</strong><small>按客户端展示顺序排列</small></div><button class="secondary-button small" type="button" @click="addInstruction(editingLocale)"><ListPlus :size="15" />添加步骤</button></div>
            <div v-if="selectedAction.instructions[editingLocale].length" class="instruction-list">
              <div v-for="(instruction, index) in selectedAction.instructions[editingLocale]" :key="index" class="instruction-row">
                <span class="step-number">{{ index + 1 }}</span>
                <textarea v-model="selectedAction.instructions[editingLocale][index]" rows="2" :placeholder="`第 ${index + 1} 步`" @input="markDirty" />
                <div class="step-actions"><button type="button" :disabled="index === 0" @click="moveInstruction(editingLocale, index, -1)"><ArrowUp :size="14" /></button><button type="button" :disabled="index === selectedAction.instructions[editingLocale].length - 1" @click="moveInstruction(editingLocale, index, 1)"><ArrowDown :size="14" /></button><button class="danger" type="button" @click="removeInstruction(editingLocale, index)"><Trash2 :size="14" /></button></div>
              </div>
            </div>
            <button v-else class="empty-steps" type="button" @click="addInstruction(editingLocale)"><ListPlus :size="22" /><span>当前语言还没有动作步骤，点击添加</span></button>
          </div>
        </section>
      </main>

      <div v-else class="empty-editor"><FilePlus2 :size="34" /><strong>选择或添加一个动作</strong><span>完整编辑界面会显示在这里</span></div>
    </div>
  </section>
</template>

<style scoped>
.exercise-catalog-tool{display:grid;gap:18px;max-width:1720px}.tool-header{display:flex;justify-content:space-between;gap:24px;align-items:flex-start}.tool-header h2{margin:0 0 6px;font-size:22px}.tool-header p{margin:0;max-width:800px;color:#64748b;line-height:1.65}.tool-actions,.editor-actions,.order-buttons{display:flex;align-items:center;gap:9px;flex-wrap:wrap}.spinning{animation:spin 1s linear infinite}.workflow-note{display:flex;gap:20px;align-items:center;flex-wrap:wrap;padding:13px 16px;border-radius:12px;background:#eff6ff;color:#1e3a5f}.workflow-note strong{font-size:18px}.dirty-state{color:#b45309;font-weight:700}.load-error{margin:0;padding:12px 14px;border-radius:10px;background:#fef2f2;color:#b91c1c}.catalog-workspace{display:grid;grid-template-columns:360px minmax(680px,1fr);gap:18px;align-items:start}.catalog-sidebar,.action-editor,.empty-editor{border:1px solid #dfe5ea;border-radius:16px;background:#fff}.catalog-sidebar{position:sticky;top:12px;overflow:hidden}.filters{display:grid;gap:10px;padding:13px;border-bottom:1px solid #e5e9ed;background:#f8fafc}.search-field{display:flex;align-items:center;gap:8px;padding:0 10px;border:1px solid #cbd5e1;border-radius:8px;background:#fff}.search-field input{width:100%;height:38px;border:0;outline:0}.filter-row{display:grid;grid-template-columns:repeat(2,1fr);gap:7px}.filter-row select{min-width:0}.sidebar-summary{display:flex;justify-content:space-between;color:#7b8791;font-size:11px}.action-list{height:720px;overflow:auto;padding:7px}.action-row{display:grid;grid-template-columns:15px 29px 64px minmax(0,1fr);gap:8px;align-items:center;width:100%;margin-bottom:5px;padding:7px;border:1px solid transparent;border-radius:11px;background:transparent;color:inherit;text-align:left;cursor:pointer}.action-row:hover{background:#f5f8f9}.action-row.active{border-color:#8fb8ad;background:#eaf5f1}.action-row.disabled{opacity:.48}.drag-handle{color:#9aa4ac;cursor:grab}.order-badge{display:grid;place-items:center;width:27px;height:27px;border-radius:50%;background:#eef1f3;color:#64717b;font-size:10px;font-weight:800}.list-gif{position:relative;display:grid;place-items:center;width:62px;height:54px;overflow:hidden;border:1px solid #e1e6e8;border-radius:8px;background:#f2f4f5;color:#a4adb4;font-size:9px;font-weight:800}.list-gif img{position:absolute;width:100%;height:100%;object-fit:contain;background:#fff}.action-row-copy{display:flex;min-width:0;flex-direction:column;gap:3px}.action-row-copy strong,.action-row-copy small,.action-row-copy em{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.action-row-copy strong{font-size:13px}.action-row-copy small{color:#64748b;font-size:10px}.action-row-copy em{color:#9aa3aa;font-size:9px;font-style:normal}.action-editor{display:grid;gap:0;overflow:hidden}.editor-heading{display:flex;justify-content:space-between;gap:20px;align-items:center;padding:20px 22px;border-bottom:1px solid #e4e8eb;background:#fbfcfc}.editor-heading h3{margin:3px 0;font-size:22px}.editor-heading p{margin:0;color:#89939b;font:11px ui-monospace,SFMono-Regular,Menlo,monospace}.eyebrow{color:#217d69;font-size:11px;font-weight:800;letter-spacing:.05em}.status-switch{display:flex!important;flex-direction:row!important;align-items:center;gap:7px!important;padding:8px 10px;border-radius:9px;background:#eef6f3}.status-switch input{width:auto!important;height:auto!important}.icon-text-button,.order-buttons button{display:flex;align-items:center;gap:5px;padding:8px 10px;border:1px solid #d7dee3;border-radius:8px;background:#fff;color:#34414a}.danger{color:#b33d38!important}.editor-section{padding:22px;border-bottom:1px solid #e4e8eb}.asset-section{display:grid;grid-template-columns:280px minmax(0,1fr);gap:22px}.gif-preview{display:grid;place-items:center;height:280px;overflow:hidden;border:1px solid #dfe5e8;border-radius:16px;background:#f3f5f6}.gif-preview img{width:100%;height:100%;object-fit:contain;background:#fff}.gif-empty{display:flex;flex-direction:column;align-items:center;gap:8px;color:#89939b;text-align:center}.gif-empty small{max-width:210px}.section-title{display:flex;justify-content:space-between;gap:16px;align-items:flex-start;margin-bottom:16px}.section-title h4{margin:0 0 5px;font-size:17px}.section-title p{margin:0;color:#7c8790;font-size:12px}.form-grid{display:grid;gap:13px}.two-columns{grid-template-columns:repeat(2,minmax(0,1fr))}.four-columns{grid-template-columns:.7fr 1fr 1fr 1fr}.wide{grid-column:1/-1}.action-editor label,.locale-editor label{display:flex;flex-direction:column;gap:6px;color:#53606a;font-size:12px;font-weight:700}.action-editor input,.action-editor select,.action-editor textarea,.filters select{box-sizing:border-box;width:100%;min-width:0;border:1px solid #cfd7dd;border-radius:8px;background:#fff;color:#202a31;outline:0}.action-editor input,.action-editor select,.filters select{height:38px;padding:0 10px}.action-editor textarea{padding:9px 10px;line-height:1.5;resize:vertical}.action-editor input:focus,.action-editor select:focus,.action-editor textarea:focus{border-color:#78a99d;box-shadow:0 0 0 3px rgb(33 125 105 / 9%)}.order-buttons button:disabled,.step-actions button:disabled{opacity:.35}.type-correspondence{display:flex;gap:12px;flex-wrap:wrap;margin-top:14px;padding:11px 13px;border-radius:10px;background:#f0f7f5;color:#3f514b;font-size:12px}.type-correspondence span{display:flex;align-items:center;gap:7px}.type-correspondence b{color:#217d69}.type-correspondence code{padding:2px 5px;border-radius:5px;background:#fff;color:#66736e;font-size:10px}.locale-tabs{display:flex;gap:7px;overflow:auto;margin-bottom:16px}.locale-tabs button{position:relative;white-space:nowrap;padding:9px 14px;border:1px solid #d7dee3;border-radius:9px;background:#fff;color:#46535d}.locale-tabs button.active{border-color:#1f806a;background:#1f806a;color:#fff}.locale-warning{position:absolute;right:-4px;top:-5px;display:grid;place-items:center;width:16px;height:16px;border-radius:50%;background:#d97706;color:#fff;font-size:10px;font-weight:900}.locale-editor{display:grid;gap:16px}.muscle-grid{display:grid;grid-template-columns:1fr 1fr;gap:14px}.instruction-heading{display:flex;justify-content:space-between;align-items:center;gap:12px;margin-top:3px}.instruction-heading>div{display:flex;flex-direction:column;gap:3px}.instruction-heading small{color:#89939b}.small{padding:7px 9px!important}.instruction-list{display:grid;gap:9px}.instruction-row{display:grid;grid-template-columns:32px minmax(0,1fr) 34px;gap:9px;align-items:start}.step-number{display:grid;place-items:center;width:30px;height:30px;margin-top:2px;border-radius:50%;background:#e8f4f0;color:#217d69;font-weight:900}.step-actions{display:grid;gap:4px}.step-actions button{display:grid;place-items:center;width:32px;height:29px;border:1px solid #d9e0e4;border-radius:7px;background:#fff}.empty-steps{display:flex;flex-direction:column;align-items:center;gap:8px;padding:30px;border:1px dashed #cbd5dc;border-radius:12px;background:#fafcfc;color:#75818a}.empty-editor{display:flex;min-height:460px;flex-direction:column;align-items:center;justify-content:center;gap:9px;color:#82909a}.empty-editor strong{color:#35424b}@keyframes spin{to{transform:rotate(360deg)}}@media(max-width:1180px){.catalog-workspace{grid-template-columns:300px minmax(600px,1fr)}.asset-section{grid-template-columns:220px minmax(0,1fr)}.gif-preview{height:220px}.four-columns{grid-template-columns:repeat(2,1fr)}}@media(max-width:900px){.tool-header,.editor-heading{flex-direction:column;align-items:flex-start}.catalog-workspace{grid-template-columns:1fr}.catalog-sidebar{position:static}.action-list{height:420px}.asset-section,.two-columns,.four-columns,.muscle-grid{grid-template-columns:1fr}.wide{grid-column:auto}}
</style>

<style scoped>
.catalog-workspace {
  grid-template-columns: var(--catalog-sidebar-width, 360px) 10px minmax(560px, 1fr);
  gap: 8px;
}

.catalog-sidebar {
  container-type: inline-size;
}

.action-list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(230px, 1fr));
  align-content: start;
  gap: 6px;
}

.action-row {
  min-width: 0;
  margin-bottom: 0;
}

.action-row.dragging {
  opacity: .28;
  outline: 1px dashed #4f9584;
}

.workspace-resizer {
  position: relative;
  align-self: stretch;
  min-height: 720px;
  padding: 0;
  border: 0;
  background: transparent;
  cursor: col-resize;
  touch-action: none;
}

.workspace-resizer::before {
  position: absolute;
  inset: 0 -3px;
  content: '';
}

.workspace-resizer span {
  position: sticky;
  top: 18px;
  display: block;
  width: 3px;
  height: 96px;
  margin: 18px auto;
  border-radius: 999px;
  background: #d4dbdf;
  transition: background .16s ease, width .16s ease;
}

.workspace-resizer:hover span,
.catalog-workspace.resizing .workspace-resizer span {
  width: 4px;
  background: #4f9584;
}

.catalog-workspace.resizing .catalog-sidebar,
.catalog-workspace.resizing .action-editor {
  pointer-events: none;
}

@media (max-width: 900px) {
  .catalog-workspace { grid-template-columns: 1fr; gap: 18px; }
  .workspace-resizer { display: none; }
}
</style>
