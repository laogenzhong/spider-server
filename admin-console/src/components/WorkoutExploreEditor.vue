<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { Download, FileUp, Plus, Search, Trash2, X } from 'lucide-vue-next'

const locales = [
  { id: 'zh-Hans', label: '简体中文' },
  { id: 'zh-Hant', label: '繁體中文' },
  { id: 'en', label: 'English' },
  { id: 'ja', label: '日本語' },
  { id: 'ko', label: '한국어' },
]
const levels = [{ id: 'beginner', label: '初学者' }, { id: 'intermediate', label: '中级' }, { id: 'advanced', label: '高级' }]
const goals = [{ id: 'muscle_gain', label: '增肌' }, { id: 'strength', label: '力量' }, { id: 'weight_loss', label: '减重' }]
const equipment = [{ id: 'gym', label: '健身房' }, { id: 'dumbbell', label: '哑铃' }, { id: 'none', label: '无' }]
const categoryOptions = [
  { id: 'home', label: '居家' },
  { id: 'travel', label: '旅行' },
  { id: 'dumbbell', label: '哑铃' },
  { id: 'resistance_band', label: '弹力带' },
  { id: 'cardio_hiit', label: '有氧运动和高强度间歇' },
  { id: 'gym', label: '健身房' },
  { id: 'bodyweight', label: '自重' },
  { id: 'suspension', label: '悬挂带' },
]
const blankText = () => Object.fromEntries(locales.map(({ id }) => [id, '']))
const blankPlan = () => ({ id: `plan-${Date.now()}`, name: blankText(), description: blankText(), defaultSetCount: 3, exercises: [] })
const blankScheme = () => ({ id: `scheme-${Date.now()}`, level: 'beginner', goal: 'muscle_gain', equipment: 'gym', name: blankText(), description: blankText(), plans: [blankPlan()] })
const blankDocument = () => ({
  schemaVersion: 1,
  configID: 'workout-explore',
  schemes: [blankScheme()],
  categories: categoryOptions.map(({ id }) => ({ id, plans: [] })),
})

const documentModel = reactive(blankDocument())
const mode = ref('schemes')
const selectedSchemeIndex = ref(0)
const selectedCategoryID = ref('home')
const selectedPlanIndex = ref(0)
const editingLocale = ref('zh-Hans')
const catalog = ref([])
const catalogQuery = ref('')
const catalogCategory = ref('all')
const catalogType = ref('all')
const status = reactive({ type: '', message: '' })
const configInput = ref(null)
const catalogInput = ref(null)

const selectedScheme = computed(() => documentModel.schemes[selectedSchemeIndex.value] || null)
const selectedCategory = computed(() => documentModel.categories.find(item => item.id === selectedCategoryID.value) || null)
const activeOwner = computed(() => mode.value === 'schemes' ? selectedScheme.value : selectedCategory.value)
const selectedPlan = computed(() => activeOwner.value?.plans?.[selectedPlanIndex.value] || null)
const catalogCategories = computed(() => categoryFilterOptions())
const catalogTypes = computed(() => taxonomyOptions('type', 'typeNames'))
const filteredCatalog = computed(() => {
  const query = catalogQuery.value.trim().toLocaleLowerCase()
  return catalog.value.filter(item => {
    if (catalogCategory.value.startsWith('subcategory:')) {
      if (item.subcategory !== catalogCategory.value.slice('subcategory:'.length)) return false
    } else if (catalogCategory.value !== 'all' && item.category !== catalogCategory.value) return false
    if (catalogType.value !== 'all' && item.type !== catalogType.value) return false
    if (!query) return true
    return [item.id, item.nameKey, ...Object.values(item.names || {}), ...Object.values(item.subcategoryNames || {})].some(value => String(value || '').toLocaleLowerCase().includes(query))
  }).slice(0, 120)
})
const selectedActionIDs = computed(() => new Set(selectedPlan.value?.exercises?.map(item => item.exerciseID) || []))
const previewName = computed(() => selectedPlan.value?.name?.[editingLocale.value] || selectedPlan.value?.name?.en || selectedPlan.value?.id || '')
const previewDescription = computed(() => selectedPlan.value?.description?.[editingLocale.value] || selectedPlan.value?.description?.en || '')

watch(documentModel, value => localStorage.setItem('spider-workout-explore-draft-v1', JSON.stringify(value)), { deep: true })
watch([mode, selectedSchemeIndex, selectedCategoryID], () => { selectedPlanIndex.value = 0 })

onMounted(async () => {
  const saved = localStorage.getItem('spider-workout-explore-draft-v1')
  if (saved) {
    try { replaceDocument(JSON.parse(saved)) } catch { /* Ignore corrupt browser draft. */ }
  }
  try {
    const response = await fetch(`${import.meta.env.BASE_URL}exercise-catalog.json`)
    if (response.ok) catalog.value = normalizeCatalog(await response.json())
  } catch { /* A manually imported catalog remains available as fallback. */ }
})

function flash(message, type = 'success') {
  status.type = type
  status.message = message
  window.setTimeout(() => { if (status.message === message) status.message = '' }, 4500)
}

function replaceDocument(value) {
  const normalized = {
    schemaVersion: 1,
    configID: String(value.configID || 'workout-explore'),
    schemes: Array.isArray(value.schemes) ? value.schemes : [],
    categories: categoryOptions.map(({ id }) => value.categories?.find(item => item.id === id) || { id, plans: [] }),
  }
  Object.keys(documentModel).forEach(key => delete documentModel[key])
  Object.assign(documentModel, normalized)
  selectedSchemeIndex.value = 0
  selectedCategoryID.value = 'home'
  selectedPlanIndex.value = 0
}

function normalizeCatalog(value) {
  const fallbackByID = new Map(catalog.value.map(item => [item.id, item]))
  const normalize = item => normalizeAction(item, fallbackByID.get(String(item?.id || item?.projectId || '').trim()))
  if (Array.isArray(value)) return value.map(normalize).filter(Boolean)
  if (Array.isArray(value?.items)) return value.items.map(normalize).filter(Boolean)
  if (value?.projectActions && typeof value.projectActions === 'object') {
    return Object.entries(value.projectActions).map(([id, item]) => normalizeAction({
      id,
      nameKey: item.projectNameKey,
      category: item.projectCategoryKey,
      type: item.projectTypeKey,
      names: item.projectLibraryDisplayNames || item.projectDetailDisplayNames,
    }, fallbackByID.get(id))).filter(Boolean)
  }
  throw new Error('不支持的动作目录格式')
}

function normalizeAction(item, fallback) {
  const id = String(item?.id || item?.projectId || '').trim()
  if (!id) return null
  const names = item.names || item.displayNames || item.projectLibraryDisplayNames || item.projectDetailDisplayNames || {}
  const sourceType = String(item.sourceType || item.type || item.typeKey || item.projectTypeKey || fallback?.sourceType || '')
  return {
    id,
    nameKey: String(item.nameKey || item.projectNameKey || ''),
    category: String(item.category || item.categoryKey || item.projectCategoryKey || ''),
    categoryNames: normalizeLocalizedNames(item.categoryNames || fallback?.categoryNames),
    subcategory: String(item.subcategory || fallback?.subcategory || ''),
    subcategoryNames: normalizeLocalizedNames(item.subcategoryNames || fallback?.subcategoryNames),
    subcategoryRank: Number.isInteger(Number(item.subcategoryRank)) ? Number(item.subcategoryRank) : Number(fallback?.subcategoryRank ?? -1),
    sourceType,
    type: displayTypeKey(sourceType),
    typeNames: normalizeLocalizedNames(item.typeNames || fallback?.typeNames),
    gifPath: String(item.gifPath || fallback?.gifPath || ''),
    names: normalizeLocalizedNames(names),
  }
}

function displayTypeKey(typeKey) {
  if (typeKey === 'exercise_type_ez_bar') return 'exercise_type_barbell'
  if (typeKey === 'exercise_type_cable') return 'exercise_type_rope'
  if (['exercise_type_sled', 'exercise_type_stationary_bike', 'exercise_type_stair_climber', 'exercise_type_elliptical'].includes(typeKey)) return 'exercise_type_machine'
  if (['exercise_type_assisted', 'exercise_type_bosu_ball', 'exercise_type_medicine_ball', 'exercise_type_stability_ball', 'exercise_type_roller'].includes(typeKey)) return 'exercise_type_other'
  return typeKey
}

function normalizeLocalizedNames(value) {
  const names = value && typeof value === 'object' ? value : {}
  return { ...names, 'zh-Hans': names['zh-Hans'] || names.zh || '' }
}

function localizedLabel(names, fallback) {
  return names?.[editingLocale.value] || names?.en || names?.['zh-Hans'] || fallback
}

function taxonomyOptions(keyField, namesField) {
  const items = new Map()
  catalog.value.forEach(action => {
    const id = action[keyField]
    if (id && !items.has(id)) items.set(id, { id, names: action[namesField] })
  })
  return [...items.values()]
    .map(item => ({ id: item.id, label: localizedLabel(item.names, item.id) }))
    .sort((left, right) => left.label.localeCompare(right.label, editingLocale.value))
}

function categoryFilterOptions() {
  const categories = new Map()
  catalog.value.forEach(action => {
    if (!action.category) return
    if (!categories.has(action.category)) {
      categories.set(action.category, { id: action.category, names: action.categoryNames, subcategories: new Map() })
    }
    if (action.subcategory && !categories.get(action.category).subcategories.has(action.subcategory)) {
      categories.get(action.category).subcategories.set(action.subcategory, {
        id: action.subcategory,
        names: action.subcategoryNames,
        rank: action.subcategoryRank,
      })
    }
  })

  return [...categories.values()]
    .map(category => ({ ...category, label: localizedLabel(category.names, category.id) }))
    .sort((left, right) => left.label.localeCompare(right.label, editingLocale.value))
    .flatMap(category => [
      { id: category.id, label: category.label },
      ...[...category.subcategories.values()]
        .sort((left, right) => left.rank - right.rank)
        .map(subcategory => ({
          id: `subcategory:${subcategory.id}`,
          label: `↳ ${localizedLabel(subcategory.names, subcategory.id)}`,
        })),
    ])
}

async function importJSON(event, kind) {
  const file = event.target.files?.[0]
  event.target.value = ''
  if (!file) return
  try {
    const value = JSON.parse(await file.text())
    if (kind === 'config') {
      replaceDocument(value)
      const errors = validateDocument()
      flash(errors.length ? `已导入，但有 ${errors.length} 个问题` : '配置已导入', errors.length ? 'warning' : 'success')
    } else {
      catalog.value = normalizeCatalog(value)
      flash(`已导入 ${catalog.value.length} 个客户端动作`)
    }
  } catch (error) {
    flash(`导入失败：${error.message}`, 'error')
  }
}

function addScheme() {
  documentModel.schemes.push(blankScheme())
  selectedSchemeIndex.value = documentModel.schemes.length - 1
  mode.value = 'schemes'
}

function removeScheme() {
  if (!selectedScheme.value || !window.confirm('确定删除当前方案及其所有计划？')) return
  documentModel.schemes.splice(selectedSchemeIndex.value, 1)
  selectedSchemeIndex.value = Math.max(0, selectedSchemeIndex.value - 1)
}

function addPlan() {
  if (!activeOwner.value) return
  activeOwner.value.plans.push(blankPlan())
  selectedPlanIndex.value = activeOwner.value.plans.length - 1
}

function removePlan() {
  if (!selectedPlan.value || !window.confirm('确定删除当前计划？')) return
  activeOwner.value.plans.splice(selectedPlanIndex.value, 1)
  selectedPlanIndex.value = Math.max(0, selectedPlanIndex.value - 1)
}

function addAction(action) {
  if (!selectedPlan.value || selectedActionIDs.value.has(action.id)) return
  selectedPlan.value.exercises.push({ exerciseID: action.id, setCount: selectedPlan.value.defaultSetCount || 3 })
}

function actionName(actionOrID) {
  const action = typeof actionOrID === 'string' ? catalog.value.find(item => item.id === actionOrID) : actionOrID
  if (!action) return typeof actionOrID === 'string' ? actionOrID : ''
  return action.names?.[editingLocale.value] || action.names?.en || action.names?.['zh-Hans'] || action.nameKey || action.id
}

function actionGIFURL(actionOrID) {
  const action = typeof actionOrID === 'string' ? catalog.value.find(item => item.id === actionOrID) : actionOrID
  if (!action?.gifPath) return ''
  const encodedPath = action.gifPath.split('/').map(segment => encodeURIComponent(segment)).join('/')
  return `${import.meta.env.BASE_URL}exercise-gifs/${encodedPath}`
}

function actionCategoryLabel(action) {
  const category = localizedLabel(action?.categoryNames, action?.category || '')
  const subcategory = localizedLabel(action?.subcategoryNames, action?.subcategory || '')
  return subcategory ? `${category} / ${subcategory}` : category
}

function actionTypeLabel(action) {
  return localizedLabel(action?.typeNames, action?.type || '')
}

function validateLocalized(value, label, errors) {
  locales.forEach(({ id }) => { if (!String(value?.[id] || '').trim()) errors.push(`${label}缺少 ${id}`) })
}

function validatePlans(plans, owner, errors) {
  const ids = new Set()
  plans.forEach((plan, index) => {
    const label = `${owner} / 计划 ${index + 1}`
    if (!String(plan.id || '').trim()) errors.push(`${label} ID 不能为空`)
    if (ids.has(plan.id)) errors.push(`${owner} 内计划 ID 重复：${plan.id}`)
    ids.add(plan.id)
    validateLocalized(plan.name, `${label}名称`, errors)
    validateLocalized(plan.description, `${label}介绍`, errors)
    if (!Number.isInteger(Number(plan.defaultSetCount)) || Number(plan.defaultSetCount) < 1 || Number(plan.defaultSetCount) > 99) errors.push(`${label}默认组数应为 1–99`)
    if (!plan.exercises?.length) errors.push(`${label}至少需要一个动作`)
    plan.exercises?.forEach(action => {
      if (!catalog.value.some(item => item.id === action.exerciseID)) errors.push(`${label}动作 ID 不在当前目录：${action.exerciseID}`)
      if (action.setCount != null && (Number(action.setCount) < 1 || Number(action.setCount) > 99)) errors.push(`${label}动作组数应为 1–99`)
    })
  })
}

function validateDocument() {
  const errors = []
  if (!String(documentModel.configID || '').trim()) errors.push('configID 不能为空')
  const schemeIDs = new Set()
  documentModel.schemes.forEach((scheme, index) => {
    const label = `方案 ${index + 1}`
    if (!String(scheme.id || '').trim()) errors.push(`${label} ID 不能为空`)
    if (schemeIDs.has(scheme.id)) errors.push(`方案 ID 重复：${scheme.id}`)
    schemeIDs.add(scheme.id)
    validateLocalized(scheme.name, `${label}名称`, errors)
    validateLocalized(scheme.description, `${label}介绍`, errors)
    validatePlans(scheme.plans || [], label, errors)
  })
  categoryOptions.forEach(({ id, label }) => validatePlans(documentModel.categories.find(item => item.id === id)?.plans || [], `分类 ${label}`, errors))
  return errors
}

function validateAndReport() {
  const errors = validateDocument()
  if (!errors.length) return flash('验证通过，可以导出')
  flash(`发现 ${errors.length} 个问题：${errors.slice(0, 3).join('；')}`, 'error')
}

function exportConfig() {
  const errors = validateDocument()
  if (errors.length) return flash(`导出前请修复：${errors.slice(0, 3).join('；')}`, 'error')
  const payload = JSON.stringify(documentModel, null, 2)
  const url = URL.createObjectURL(new Blob([payload], { type: 'application/json' }))
  const link = document.createElement('a')
  const safeID = documentModel.configID.replace(/[^a-zA-Z0-9_-]+/g, '-') || 'workout-explore'
  link.href = url
  link.download = `${safeID}.workout-explore.json`
  link.click()
  URL.revokeObjectURL(url)
  flash('配置已导出')
}
</script>

<template>
  <section class="explore-editor">
    <div class="section-toolbar">
      <div><h2>训练计划探索配置</h2><p>本地制作、预览并导出 *.workout-explore.json；不写入服务端数据库</p></div>
      <div class="explore-actions">
        <input ref="configInput" hidden type="file" accept=".json,.workout-explore.json" @change="importJSON($event, 'config')" />
        <button class="secondary-button" type="button" @click="configInput.click()"><FileUp :size="16" />导入配置</button>
        <button class="secondary-button" type="button" @click="validateAndReport">验证</button>
        <button class="primary-button" type="button" @click="exportConfig"><Download :size="16" />导出配置</button>
      </div>
    </div>

    <div v-if="status.message" class="explore-status" :class="status.type">{{ status.message }}</div>

    <div class="explore-meta-card">
      <label>配置 ID<input v-model.trim="documentModel.configID" placeholder="lifttags-default" /></label>
      <div><strong>固定格式</strong><span>schemaVersion 1 · 五种语言 · 8 个分类</span></div>
      <div><strong>客户端索引</strong><span>动作 ID 对应 ExerciseGIFItem.id</span></div>
    </div>

    <div class="explore-workspace">
      <aside class="explore-outline">
        <div class="segmented">
          <button type="button" :class="{ selected: mode === 'schemes' }" @click="mode = 'schemes'">方案</button>
          <button type="button" :class="{ selected: mode === 'categories' }" @click="mode = 'categories'">8 类计划</button>
        </div>

        <template v-if="mode === 'schemes'">
          <button v-for="(scheme, index) in documentModel.schemes" :key="scheme.id + index" class="outline-item" :class="{ active: selectedSchemeIndex === index }" type="button" @click="selectedSchemeIndex = index">
            <strong>{{ scheme.name?.['zh-Hans'] || scheme.id || `方案 ${index + 1}` }}</strong>
            <span>{{ levels.find(item => item.id === scheme.level)?.label }} · {{ goals.find(item => item.id === scheme.goal)?.label }} · {{ equipment.find(item => item.id === scheme.equipment)?.label }}</span>
          </button>
          <button class="secondary-button wide-button" type="button" @click="addScheme"><Plus :size="15" />添加方案</button>
        </template>
        <template v-else>
          <button v-for="category in categoryOptions" :key="category.id" class="outline-item" :class="{ active: selectedCategoryID === category.id }" type="button" @click="selectedCategoryID = category.id">
            <strong>{{ category.label }}</strong><span>{{ documentModel.categories.find(item => item.id === category.id)?.plans.length || 0 }} 个计划</span>
          </button>
        </template>
      </aside>

      <main class="explore-main">
        <template v-if="mode === 'schemes' && selectedScheme">
          <div class="editor-heading"><h3>方案配置</h3><button class="danger-link" type="button" @click="removeScheme"><Trash2 :size="15" />删除方案</button></div>
          <div class="form-grid compact">
            <label>方案 ID<input v-model.trim="selectedScheme.id" /></label>
            <label>水平<select v-model="selectedScheme.level"><option v-for="item in levels" :key="item.id" :value="item.id">{{ item.label }}</option></select></label>
            <label>目标<select v-model="selectedScheme.goal"><option v-for="item in goals" :key="item.id" :value="item.id">{{ item.label }}</option></select></label>
            <label>器械<select v-model="selectedScheme.equipment"><option v-for="item in equipment" :key="item.id" :value="item.id">{{ item.label }}</option></select></label>
          </div>
          <div class="locale-tabs"><button v-for="locale in locales" :key="locale.id" type="button" :class="{ active: editingLocale === locale.id }" @click="editingLocale = locale.id">{{ locale.label }}</button></div>
          <div class="form-grid compact">
            <label>方案名<input v-model="selectedScheme.name[editingLocale]" /></label>
            <label class="wide">方案介绍<textarea v-model="selectedScheme.description[editingLocale]" rows="2"></textarea></label>
          </div>
        </template>
        <template v-else-if="selectedCategory">
          <div class="editor-heading"><div><h3>{{ categoryOptions.find(item => item.id === selectedCategoryID)?.label }}</h3><p>分类 ID：{{ selectedCategoryID }}</p></div></div>
          <div class="locale-tabs"><button v-for="locale in locales" :key="locale.id" type="button" :class="{ active: editingLocale === locale.id }" @click="editingLocale = locale.id">{{ locale.label }}</button></div>
        </template>

        <div v-if="activeOwner" class="plan-tabs">
          <button v-for="(plan, index) in activeOwner.plans" :key="plan.id + index" type="button" :class="{ active: selectedPlanIndex === index }" @click="selectedPlanIndex = index">{{ plan.name?.[editingLocale] || plan.id || `计划 ${index + 1}` }}</button>
          <button type="button" class="add-tab" @click="addPlan"><Plus :size="14" />计划</button>
        </div>

        <template v-if="selectedPlan">
          <div class="editor-heading"><h3>计划内容</h3><button class="danger-link" type="button" @click="removePlan"><Trash2 :size="15" />删除计划</button></div>
          <div class="form-grid compact">
            <label>计划 ID<input v-model.trim="selectedPlan.id" /></label>
            <label>默认组数<input v-model.number="selectedPlan.defaultSetCount" type="number" min="1" max="99" /></label>
            <label>计划名<input v-model="selectedPlan.name[editingLocale]" /></label>
            <label class="wide">计划介绍<textarea v-model="selectedPlan.description[editingLocale]" rows="2"></textarea></label>
          </div>

          <div class="exercise-layout">
            <div class="exercise-catalog-panel">
              <div class="editor-heading"><div><h3>动作库</h3><p>{{ catalog.length }} 个客户端动作</p></div><input ref="catalogInput" hidden type="file" accept=".json" @change="importJSON($event, 'catalog')" /><button class="secondary-button small" type="button" @click="catalogInput.click()"><FileUp :size="14" />导入目录</button></div>
              <div class="catalog-filters"><div class="compact-search"><Search :size="15" /><input v-model="catalogQuery" placeholder="搜索动作 ID 或多语言名称" /></div><select v-model="catalogCategory"><option value="all">全部部位</option><option v-for="item in catalogCategories" :key="item.id" :value="item.id">{{ item.label }}</option></select><select v-model="catalogType"><option value="all">全部器械</option><option v-for="item in catalogTypes" :key="item.id" :value="item.id">{{ item.label }}</option></select></div>
              <div class="catalog-results">
                <button v-for="action in filteredCatalog" :key="action.id" class="catalog-action-card" type="button" :disabled="selectedActionIDs.has(action.id)" @click="addAction(action)">
                  <span class="exercise-gif-frame">
                    <span class="gif-fallback">GIF</span>
                    <img v-if="actionGIFURL(action)" :src="actionGIFURL(action)" :alt="actionName(action)" loading="lazy" @error="$event.currentTarget.style.display = 'none'" />
                  </span>
                  <span class="action-copy"><strong>{{ actionName(action) }}</strong><small>{{ action.id }}</small><em>{{ actionCategoryLabel(action) }} · {{ actionTypeLabel(action) }}</em></span>
                  <span class="add-action-icon"><Plus :size="16" /></span>
                </button>
              </div>
            </div>

            <div class="selected-exercises-panel">
              <div class="editor-heading"><div><h3>计划动作</h3><p>{{ selectedPlan.exercises.length }} 个动作</p></div></div>
              <div v-for="(action, index) in selectedPlan.exercises" :key="action.exerciseID" class="selected-action-row">
                <span class="selected-gif-frame"><span class="gif-fallback">GIF</span><img v-if="actionGIFURL(action.exerciseID)" :src="actionGIFURL(action.exerciseID)" :alt="actionName(action.exerciseID)" loading="lazy" @error="$event.currentTarget.style.display = 'none'" /><b>{{ index + 1 }}</b></span>
                <div class="selected-action-copy"><strong>{{ actionName(action.exerciseID) }}</strong><small>{{ action.exerciseID }}</small></div>
                <label class="set-count-field"><span>组数</span><input v-model.number="action.setCount" type="number" min="1" max="99" /></label>
                <button class="remove-action-button" type="button" title="移除动作" @click="selectedPlan.exercises.splice(index, 1)"><X :size="16" /></button>
              </div>
              <div v-if="!selectedPlan.exercises.length" class="empty-cell">从左侧动作库添加动作</div>
            </div>
          </div>
        </template>
        <div v-else class="empty-cell editor-empty">当前条目还没有计划，点击“+ 计划”开始配置</div>
      </main>

      <aside class="explore-preview">
        <span class="preview-kicker">客户端预览 · {{ locales.find(item => item.id === editingLocale)?.label }}</span>
        <div class="preview-phone">
          <div v-if="mode === 'schemes' && selectedScheme" class="preview-scheme"><strong>{{ selectedScheme.name[editingLocale] || '方案名' }}</strong><p>{{ selectedScheme.description[editingLocale] || '方案介绍' }}</p><small>{{ levels.find(item => item.id === selectedScheme.level)?.label }} · {{ goals.find(item => item.id === selectedScheme.goal)?.label }} · {{ equipment.find(item => item.id === selectedScheme.equipment)?.label }}</small></div>
          <div v-if="selectedPlan" class="preview-plan"><h3>{{ previewName || '计划名' }}</h3><p>{{ previewDescription || '计划介绍' }}</p><div v-for="action in selectedPlan.exercises" :key="action.exerciseID"><span class="preview-action"><span class="preview-gif-frame"><img v-if="actionGIFURL(action.exerciseID)" :src="actionGIFURL(action.exerciseID)" :alt="actionName(action.exerciseID)" loading="lazy" /></span><span>{{ actionName(action.exerciseID) }}</span></span><b>{{ action.setCount || selectedPlan.defaultSetCount }} 组</b></div></div>
          <div v-else class="empty-cell">暂无计划</div>
        </div>
      </aside>
    </div>
  </section>
</template>

<style scoped>
.explore-editor{max-width:1600px;padding:24px 28px 40px}.explore-actions,.editor-heading,.catalog-filters{display:flex;align-items:center;gap:10px}.explore-status{margin:0 0 14px;padding:11px 14px;border-radius:10px;background:#e8f7ee;color:#17633a}.explore-status.error{background:#fff0ef;color:#a82c24}.explore-status.warning{background:#fff7df;color:#7a5710}.explore-meta-card{display:grid;grid-template-columns:1.2fr 1fr 1fr;gap:14px;padding:16px;margin-bottom:16px;border:1px solid #dce1e4;border-radius:16px;background:#fff}.explore-meta-card label,.explore-meta-card div{display:flex;flex-direction:column;gap:6px}.explore-meta-card span,.editor-heading p{color:#7e8992;font-size:12px}.explore-workspace{display:grid;grid-template-columns:220px minmax(560px,1fr) 300px;gap:16px;align-items:start}.explore-outline,.explore-main,.explore-preview{border:1px solid #dce1e4;border-radius:16px;background:#fff;padding:14px}.explore-outline{display:flex;flex-direction:column;gap:8px;position:sticky;top:18px}.outline-item{display:flex;flex-direction:column;gap:4px;text-align:left;border:1px solid transparent;background:transparent;border-radius:11px;padding:10px;color:inherit}.outline-item span{font-size:11px;color:#7e8992}.outline-item:hover,.outline-item.active{background:#f4f6f7;border-color:#dce1e4}.wide-button{width:100%;justify-content:center;margin-top:4px}.explore-main{min-width:0}.editor-heading{justify-content:space-between;margin:4px 0 12px}.editor-heading h3,.editor-heading p{margin:0}.danger-link{display:flex;align-items:center;gap:5px;border:0;background:transparent;color:#c2413a}.form-grid.compact{margin-bottom:14px}.locale-tabs,.plan-tabs{display:flex;gap:6px;overflow-x:auto;margin:12px 0}.locale-tabs button,.plan-tabs button{white-space:nowrap;border:1px solid #dce1e4;background:transparent;border-radius:9px;padding:7px 10px;color:inherit}.locale-tabs button.active,.plan-tabs button.active{background:#1d2939;color:#fff;border-color:#1d2939}.plan-tabs .add-tab{display:flex;align-items:center;gap:4px;border-style:dashed}.exercise-layout{display:grid;grid-template-columns:1.15fr .85fr;gap:14px;margin-top:16px}.exercise-catalog-panel,.selected-exercises-panel{min-width:0;border:1px solid #dce1e4;border-radius:13px;padding:12px}.catalog-filters{flex-wrap:wrap}.catalog-filters .compact-search{flex:1 1 220px}.catalog-filters select{max-width:150px}.catalog-results{display:grid;gap:6px;max-height:500px;overflow:auto;margin-top:10px}.catalog-results button{display:flex;justify-content:space-between;align-items:center;text-align:left;border:1px solid #dce1e4;background:transparent;border-radius:9px;padding:9px;color:inherit}.catalog-results button:disabled{opacity:.42}.catalog-results span,.selected-action-row>div{display:flex;flex-direction:column;min-width:0}.catalog-results small,.selected-action-row small{color:#7e8992;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.selected-action-row{display:grid;grid-template-columns:26px minmax(0,1fr) 62px 28px;gap:7px;align-items:center;padding:9px 0;border-bottom:1px solid #dce1e4}.selected-action-row label{display:flex;align-items:center;gap:4px;font-size:12px}.selected-action-row input{width:42px;padding:5px}.selected-action-row button{border:0;background:transparent;color:#7e8992}.action-order{display:grid;place-items:center;width:24px;height:24px;border-radius:50%;background:#f4f6f7;font-size:11px}.editor-empty{padding:54px 10px}.explore-preview{position:sticky;top:18px}.preview-kicker{display:block;color:#7e8992;font-size:11px;margin-bottom:10px}.preview-phone{min-height:430px;padding:14px;border-radius:22px;background:#101419;color:#fff}.preview-scheme{padding:12px;margin-bottom:10px;border-radius:14px;background:#ffffff12}.preview-scheme p,.preview-plan p{font-size:12px;color:#ffffffa6}.preview-scheme small{color:#74d4a3}.preview-plan{padding:14px;border:1px solid #ffffff1c;border-radius:16px;background:#ffffff0c}.preview-plan h3{margin:0}.preview-plan>div{display:flex;justify-content:space-between;gap:8px;padding:9px 0;border-bottom:1px solid #ffffff12;font-size:12px}.preview-plan b{white-space:nowrap}.small{padding:7px 9px}@media(max-width:1200px){.explore-workspace{grid-template-columns:200px 1fr}.explore-preview{grid-column:1/-1;position:static}.preview-phone{min-height:240px}}@media(max-width:800px){.explore-meta-card,.explore-workspace,.exercise-layout{grid-template-columns:1fr}.explore-outline,.explore-preview{position:static}.explore-actions{flex-wrap:wrap}}
.exercise-layout{grid-template-columns:minmax(0,1fr)}.exercise-catalog-panel,.selected-exercises-panel{padding:14px;background:#fbfcfc}.catalog-results{grid-template-columns:repeat(2,minmax(0,1fr));gap:10px;max-height:620px;padding-right:4px}.catalog-results .catalog-action-card{display:grid;grid-template-columns:76px minmax(0,1fr) 30px;gap:10px;min-height:82px;padding:7px;background:#fff;text-align:left}.catalog-results .catalog-action-card:hover:not(:disabled){border-color:#91b9af;box-shadow:0 5px 15px rgb(29 41 57 / 7%);transform:translateY(-1px)}.exercise-gif-frame,.selected-gif-frame,.preview-gif-frame{position:relative;display:grid!important;place-items:center;overflow:hidden;background:#eef1f2;border:1px solid #e0e5e7}.exercise-gif-frame{width:76px;height:66px;border-radius:9px}.exercise-gif-frame img,.selected-gif-frame img,.preview-gif-frame img{position:relative;z-index:1;width:100%;height:100%;object-fit:contain;background:#fff}.gif-fallback{position:absolute;color:#9aa3aa;font-size:10px;font-weight:800;letter-spacing:.08em}.catalog-results .action-copy{display:flex;min-width:0;flex-direction:column;align-self:center;gap:3px}.catalog-results .action-copy strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:13px}.catalog-results .action-copy small{font-size:10px}.catalog-results .action-copy em{overflow:hidden;color:#9aa3aa;font-size:9px;font-style:normal;text-overflow:ellipsis;white-space:nowrap}.add-action-icon{display:grid!important;place-items:center;width:28px;height:28px;align-self:center;border-radius:50%;color:#217d69;background:#e6f3ef}.catalog-action-card:disabled .add-action-icon{color:#6f7981;background:#edf0f2}.selected-exercises-panel{padding-bottom:8px}.selected-action-row{display:grid;grid-template-columns:68px minmax(0,1fr) 100px 34px;gap:12px;min-height:76px;align-items:center;padding:9px 4px;border-bottom:1px solid #e1e6e8}.selected-gif-frame{width:64px;height:56px;border-radius:10px}.selected-gif-frame>b{position:absolute;z-index:2;left:4px;top:4px;display:grid;place-items:center;width:20px;height:20px;border-radius:50%;color:#fff;background:rgb(24 30 35 / 82%);font-size:10px}.selected-action-copy{display:flex!important;min-width:0;flex-direction:column;gap:4px}.selected-action-copy strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:14px}.selected-action-copy small{font-size:10px}.set-count-field{display:grid!important;grid-template-columns:auto 54px;align-items:center;gap:7px!important;margin:0!important;color:#69747d!important}.set-count-field input{width:54px!important;height:34px!important;text-align:center}.remove-action-button{display:grid!important;place-items:center;width:32px;height:32px;border:0;border-radius:50%;color:#8b959d;background:transparent}.remove-action-button:hover{color:#b13e38;background:#fbeaea}.preview-action{display:flex;align-items:center;gap:7px;min-width:0}.preview-gif-frame{flex:0 0 auto;width:36px;height:32px;border:0;border-radius:6px;background:#ffffff12}.preview-action>span:last-child{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.preview-plan>div{align-items:center}.explore-editor input,.explore-editor select,.explore-editor textarea{min-width:0;border:1px solid #d5dbdf;border-radius:6px;color:#252b30;background:#fff;outline:none}.explore-editor input,.explore-editor select{height:36px;padding:0 10px}.explore-editor textarea{padding:9px 10px;resize:vertical}.explore-editor input:focus,.explore-editor select:focus,.explore-editor textarea:focus{border-color:#91b9af;box-shadow:0 0 0 3px rgb(33 125 105 / 8%)}@media(max-width:1450px){.explore-workspace{grid-template-columns:200px minmax(0,1fr)}.explore-preview{grid-column:1/-1;position:static}.preview-phone{min-height:240px}}@media(max-width:900px){.catalog-results{grid-template-columns:1fr}.selected-action-row{grid-template-columns:60px minmax(0,1fr) 88px 32px}.selected-gif-frame{width:56px;height:50px}.set-count-field{grid-template-columns:auto 48px}.set-count-field input{width:48px!important}}
</style>
