<script setup>
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import {
  Activity,
  ArrowLeft,
  BadgeDollarSign,
  CalendarPlus,
  ChartNoAxesColumnIncreasing,
  CheckCheck,
  ChevronDown,
  CircleAlert,
  CircleUserRound,
  ClipboardList,
  ClipboardCopy,
  Clock3,
  ExternalLink,
  FileUp,
  Gift,
  LayoutDashboard,
  Dumbbell,
  LoaderCircle,
  LogOut,
  MessageSquareReply,
  MessageSquareText,
  RefreshCw,
  RotateCcw,
  Save,
  Search,
  Settings2,
  ShieldCheck,
  Trophy,
  UserRoundPlus,
  UserRoundCheck,
  UsersRound,
} from 'lucide-vue-next'
import { localOfferReplyRequest, queryString, request, subscribeAdminRoute } from './api'
import ExerciseLibraryImportTool from './components/ExerciseLibraryImportTool.vue'
import Pagination from './components/Pagination.vue'
import WorkoutExploreEditor from './components/WorkoutExploreEditor.vue'

const navGroups = [
  { label: '总览', items: [{ id: 'overview', label: '概览', icon: LayoutDashboard }] },
  { label: '用户运营', items: [
    { id: 'vip', label: '用户与 Pro', icon: UserRoundCheck },
    { id: 'userList', label: '用户列表', icon: UsersRound },
    { id: 'activity', label: '日活用户', icon: Activity },
    { id: 'todayRegistrations', label: '今日注册', icon: UserRoundPlus },
    { id: 'registrations', label: '每日注册', icon: CalendarPlus },
    { id: 'feedback', label: '用户反馈', icon: MessageSquareText },
    { id: 'onboarding', label: 'Onboard 信息', icon: ClipboardList },
    { id: 'friendProfiles', label: '好友资料', icon: UsersRound },
    { id: 'featureAdoption', label: '功能新增', icon: ChartNoAxesColumnIncreasing },
  ] },
  { label: '训练数据', items: [
    { id: 'shareScores', label: '分享积分', icon: Trophy },
    { id: 'planData', label: '用户训练计划', icon: ClipboardList },
    { id: 'workoutData', label: '用户锻炼记录', icon: Dumbbell },
    { id: 'workoutExplore', label: '探索配置', icon: Dumbbell },
    { id: 'exerciseLibraryImport', label: '动作库配置', icon: FileUp },
  ] },
  { label: '商业与系统', items: [
    { id: 'payments', label: '付费记录', icon: BadgeDollarSign },
    { id: 'paywallSessions', label: '付费墙会话', icon: ClipboardList },
    { id: 'refunds', label: '退款用户', icon: RotateCcw },
    { id: 'syncFailures', label: '丢弃任务', icon: CircleAlert },
    { id: 'offerReply', label: '兑换码回复', icon: MessageSquareReply },
    { id: 'update', label: '版本更新', icon: Settings2 },
  ] },
]
const navItems = navGroups.flatMap((group) => group.items)

const current = ref('overview')
const expandedNavGroup = ref('')
const connected = ref(false)
const routeStatus = reactive({ mode: 'automatic', current_route: '', routes: [] })
const loading = reactive({})
const toast = reactive({ visible: false, type: 'success', message: '' })
const today = localDateString(new Date())
const thirtyDaysAgo = localDateString(new Date(Date.now() - 29 * 24 * 60 * 60 * 1000))
const operator = ref(localStorage.getItem('spider-admin-operator') || 'local_admin')

const overview = reactive({
  daily_active: 0,
  registrations: 0,
  payments: 0,
  feedback: 0,
  latest_version: '',
})

const userIdentifier = ref('')
const user = ref(null)
const grantOption = ref('30_days')
const grantReason = ref('admin_console_grant')
const grantOptions = [
  { id: '1_minute', label: '1 分钟', duration_minutes: 1 },
  { id: '7_days', label: '7 天', duration_days: 7 },
  { id: '30_days', label: '1 个月', duration_days: 30 },
  { id: '90_days', label: '3 个月', duration_days: 90 },
  { id: '365_days', label: '1 年', duration_days: 365 },
  { id: 'lifetime', label: '永久', lifetime: true },
]

const payments = reactive({ items: [], total: 0, page: 1, page_size: 30 })
const paymentFilters = reactive({ search: '', source: 'all', entry_point: 'all', from: '', to: '' })
const paywallSessions = reactive({ items: [], total: 0, page: 1, page_size: 30 })
const paywallSessionFilters = reactive({ search: '', status: 'all', entry_point: 'all', from: thirtyDaysAgo, to: today })
const refunds = reactive({ items: [], total: 0, page: 1, page_size: 30 })
const refundFilters = reactive({ search: '', source: 'all', status: 'requested', from: '', to: '' })
const activities = reactive({ items: [], total: 0, page: 1, page_size: 30 })
const activityFilters = reactive({ search: '', date: today })
const todayRegistrations = reactive({ items: [], total: 0, page: 1, page_size: 30 })
const todayRegistrationFilters = reactive({ search: '' })
const userList = reactive({ items: [], total: 0, page: 1, page_size: 30 })
const userListFilters = reactive({ search: '' })
const registrations = reactive({ items: [], total: 0, page: 1, page_size: 30 })
const registrationFilters = reactive({ search: '', from: today, to: today })
const feedback = reactive({ items: [], total: 0, page: 1, page_size: 30 })
const feedbackFilters = reactive({ search: '', from: thirtyDaysAgo, to: today })
const syncFailures = reactive({ items: [], total: 0, page: 1, page_size: 30 })
const syncFailureFilters = reactive({ search: '', status: 'pending', from: '', to: '' })
const expandedSyncFailureID = ref(0)
const onboardingProfiles = reactive({ items: [], total: 0, page: 1, page_size: 30 })
const onboardingFilters = reactive({ search: '', from: thirtyDaysAgo, to: today })
const expandedOnboardingID = ref(0)
const friendProfiles = reactive({ items: [], total: 0, page: 1, page_size: 30 })
const friendProfileFilters = reactive({ search: '', from: thirtyDaysAgo, to: today })
const expandedFriendProfileID = ref(0)
const featureAdoption = reactive({ items: [], total: 0, page: 1, page_size: 30 })
const featureAdoptionFilters = reactive({ from: thirtyDaysAgo, to: today })
const sharedContentScores = reactive({ items: [], total: 0, page: 1, page_size: 30 })
const sharedContentScoreFilters = reactive({ kind: 'plan', search: '' })
const planDataUsers = reactive({ items: [], total: 0, page: 1, page_size: 30 })
const planDataFilters = reactive({ search: '' })
const planDataSelectedUser = ref(null)
const planDataDetail = ref([])
const workoutDataUsers = reactive({ items: [], total: 0, page: 1, page_size: 30 })
const workoutDataFilters = reactive({ search: '' })
const workoutDataSelectedUser = ref(null)
const workoutDataSessions = reactive({ items: [], total: 0, page: 1, page_size: 20 })

const offerReplyStatus = reactive({ total_count: 0, used_count: 0, unused_count: 0, max_used_id: 0, next_id: 0 })
const offerCodes = reactive({ items: [], total: 0, page: 1, page_size: 50 })
const offerCodeFilters = reactive({ search: '', status: 'all' })
const offerReplyInput = ref('')
const offerUsageRange = ref('')
const offerCSVInput = ref(null)
const offerImportDragging = ref(false)
const offerReplyResult = reactive({
  id: 0,
  already_used: false,
  reply: '',
})

const appUpdate = reactive({
  latest_version: '',
  min_supported_version: '',
  force_update_enabled: false,
  update_available_enabled: true,
  app_store_url: '',
  message_zh_hans: '',
  message_zh_hant: '',
  message_en: '',
  message_ja: '',
  message_ko: '',
})

const currentTitle = computed(() => navItems.find((item) => item.id === current.value)?.label || '管理后台')
const isRequesting = computed(() => Object.values(loading).some(Boolean))
const currentRouteLabel = computed(() => routeStatus.current_route === 'line2' ? '线路二' : '线路一')
const currentRouteURL = computed(() => routeStatus.routes.find((route) => route.id === routeStatus.current_route)?.url || '')
const hasConfiguredRoute = (routeID) => routeStatus.routes.some((route) => route.id === routeID)
const activeUserFilters = computed(() => {
  if (current.value === 'activity') return activityFilters
  if (current.value === 'todayRegistrations') return todayRegistrationFilters
  if (current.value === 'userList') return userListFilters
  return registrationFilters
})
const visibleUserList = computed(() => {
  if (current.value === 'activity') return activities
  if (current.value === 'todayRegistrations') return todayRegistrations
  if (current.value === 'userList') return userList
  return registrations
})

function localDateString(date) {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function notify(message, type = 'success') {
  toast.message = message
  toast.type = type
  toast.visible = true
  window.clearTimeout(notify.timer)
  notify.timer = window.setTimeout(() => {
    toast.visible = false
  }, 3200)
}

async function withLoading(key, action) {
  loading[key] = true
  try {
    return await action()
  } catch (error) {
    notify(error.message || '操作失败', 'error')
    throw error
  } finally {
    loading[key] = false
  }
}

async function checkConnection() {
	loading.health = true
  try {
    await request('/health')
    Object.assign(routeStatus, await request('/route-status'))
    connected.value = true
  } catch (error) {
    connected.value = false
    notify(error.message, 'error')
	} finally {
		loading.health = false
  }
}

async function changeRouteMode(mode) {
  if (routeStatus.mode === mode) return
  loading.route = true
  try {
    Object.assign(routeStatus, await request('/route-mode', { method: 'PUT', body: { mode } }))
    await checkConnection()
    if (connected.value) {
      notify(mode === 'automatic' ? `已恢复自动选线：${currentRouteLabel.value}` : `已固定使用${currentRouteLabel.value}`)
    }
  } catch (error) {
    notify(error.message || '切换线路失败', 'error')
  } finally {
    loading.route = false
  }
}

async function selectSection(id) {
  current.value = id
  const group = navGroups.find((candidate) => candidate.items.some((item) => item.id === id))
  if (group) expandedNavGroup.value = group.label
  if (id === 'overview') await loadOverview()
  if (id === 'payments') await loadPayments()
  if (id === 'paywallSessions') await loadPaywallSessions()
  if (id === 'refunds') await loadRefunds()
  if (id === 'activity') await loadActivities()
  if (id === 'todayRegistrations') await loadTodayRegistrations()
  if (id === 'userList') await loadUserList()
  if (id === 'registrations') await loadRegistrations()
  if (id === 'feedback') await loadFeedback()
  if (id === 'syncFailures') await loadSyncFailures()
  if (id === 'onboarding') await loadOnboardingProfiles()
  if (id === 'friendProfiles') await loadFriendProfiles()
  if (id === 'featureAdoption') await loadFeatureAdoption()
  if (id === 'shareScores') await loadSharedContentScores()
  if (id === 'planData') await loadPlanDataUsers()
  if (id === 'workoutData') await loadWorkoutDataUsers()
  if (id === 'offerReply') await loadOfferReplyWorkspace()
  if (id === 'update') await loadAppUpdate()
}

function toggleNavGroup(label) {
	if (label === '总览') return
  expandedNavGroup.value = expandedNavGroup.value === label ? '' : label
}

function isNavGroupExpanded(label) {
  return label === '总览' || expandedNavGroup.value === label
}

async function loadOverview() {
  await withLoading('overview', async () => {
    Object.assign(overview, await request(`/overview${queryString({ from: today, to: today })}`))
  }).catch(() => {})
}

async function searchUser() {
  const identifier = userIdentifier.value.trim()
  if (!identifier) {
    await selectSection('userList')
    return
  }
  await withLoading('user', async () => {
    user.value = await request(`/users/${encodeURIComponent(identifier)}`)
  }).catch(() => {
    user.value = null
  })
}

async function grantVIP() {
  if (!user.value) return
  const option = grantOptions.find((item) => item.id === grantOption.value)
  localStorage.setItem('spider-admin-operator', operator.value.trim())
  await withLoading('grant', async () => {
    await request('/vip/grant', {
      method: 'POST',
      body: {
        account: user.value.account,
        lifetime: Boolean(option.lifetime),
        duration_days: option.duration_days || 0,
        duration_minutes: option.duration_minutes || 0,
        operator: operator.value.trim(),
        reason: grantReason.value.trim(),
      },
    })
    notify('Pro 已开通')
    await searchUser()
  }).catch(() => {})
}

async function revokeVIP() {
  if (!user.value || !window.confirm(`确认撤销 ${user.value.account} 的后台开通 Pro？Apple 购买权益不会受影响。`)) return
  localStorage.setItem('spider-admin-operator', operator.value.trim())
  await withLoading('revoke', async () => {
    await request('/vip/revoke', {
      method: 'POST',
      body: {
        account: user.value.account,
        operator: operator.value.trim(),
        reason: 'admin_console_revoke',
      },
    })
    notify('后台开通 Pro 已撤销')
    await searchUser()
  }).catch(() => {})
}

async function loadPayments(page = payments.page) {
  await withLoading('payments', async () => {
    Object.assign(payments, await request(`/payments${queryString({ ...paymentFilters, page, page_size: payments.page_size })}`))
  }).catch(() => {})
}

async function loadPaywallSessions(page = paywallSessions.page) {
  await withLoading('paywallSessions', async () => {
    Object.assign(paywallSessions, await request(`/paywall-sessions${queryString({ ...paywallSessionFilters, page, page_size: paywallSessions.page_size })}`))
  }).catch(() => {})
}

async function loadRefunds(page = refunds.page) {
  await withLoading('refunds', async () => {
    Object.assign(refunds, await request(`/refunds${queryString({ ...refundFilters, page, page_size: refunds.page_size })}`))
  }).catch(() => {})
}

async function loadActivities(page = activities.page) {
  await withLoading('activities', async () => {
    Object.assign(activities, await request(`/daily-active${queryString({ search: activityFilters.search, from: activityFilters.date, to: activityFilters.date, page, page_size: activities.page_size })}`))
  }).catch(() => {})
}

async function loadTodayRegistrations(page = todayRegistrations.page) {
  await withLoading('todayRegistrations', async () => {
    Object.assign(todayRegistrations, await request(`/registrations${queryString({ search: todayRegistrationFilters.search, from: today, to: today, page, page_size: todayRegistrations.page_size })}`))
  }).catch(() => {})
}

async function loadUserList(page = userList.page) {
  await withLoading('userList', async () => {
    Object.assign(userList, await request(`/registrations${queryString({ search: userListFilters.search, page, page_size: userList.page_size })}`))
  }).catch(() => {})
}

async function loadRegistrations(page = registrations.page) {
  await withLoading('registrations', async () => {
    Object.assign(registrations, await request(`/registrations${queryString({ ...registrationFilters, page, page_size: registrations.page_size })}`))
  }).catch(() => {})
}

async function loadFeedback(page = feedback.page) {
  await withLoading('feedback', async () => {
    Object.assign(feedback, await request(`/feedback${queryString({ ...feedbackFilters, page, page_size: feedback.page_size })}`))
  }).catch(() => {})
}

async function loadSyncFailures(page = syncFailures.page) {
  await withLoading('syncFailures', async () => {
    Object.assign(syncFailures, await request(`/client-sync-failures${queryString({ ...syncFailureFilters, page, page_size: syncFailures.page_size })}`))
    expandedSyncFailureID.value = 0
  }).catch(() => {})
}

async function resolveSyncFailure(item) {
  if (!window.confirm(`确认将任务 ${item.id} 标记为已处理？此操作只更新处理状态，不会自动重放请求。`)) return
  const resolvedBy = operator.value.trim()
  if (!resolvedBy) {
    notify('请填写操作人', 'error')
    return
  }
  localStorage.setItem('spider-admin-operator', resolvedBy)
  await withLoading(`resolveSyncFailure-${item.id}`, async () => {
    await request(`/client-sync-failures/${item.id}/resolve`, {
      method: 'POST',
      body: { operator: resolvedBy, note: 'admin_console_mark_resolved' },
    })
    notify('任务已标记为已处理')
    const page = syncFailureFilters.status === 'pending' && syncFailures.items.length === 1 && syncFailures.page > 1
      ? syncFailures.page - 1
      : syncFailures.page
    await loadSyncFailures(page)
  }).catch(() => {})
}

async function loadOnboardingProfiles(page = onboardingProfiles.page) {
  await withLoading('onboarding', async () => {
    Object.assign(onboardingProfiles, await request(`/onboarding-profiles${queryString({ ...onboardingFilters, page, page_size: onboardingProfiles.page_size })}`))
    expandedOnboardingID.value = 0
  }).catch(() => {})
}

async function loadFriendProfiles(page = friendProfiles.page) {
  await withLoading('friendProfiles', async () => {
    Object.assign(friendProfiles, await request(`/friend-profiles${queryString({ ...friendProfileFilters, page, page_size: friendProfiles.page_size })}`))
    expandedFriendProfileID.value = 0
  }).catch(() => {})
}

async function loadFeatureAdoption(page = featureAdoption.page) {
  await withLoading('featureAdoption', async () => {
    Object.assign(featureAdoption, await request(`/feature-adoption${queryString({ ...featureAdoptionFilters, page, page_size: featureAdoption.page_size })}`))
  }).catch(() => {})
}

async function loadSharedContentScores(page = sharedContentScores.page) {
  await withLoading('sharedContentScores', async () => {
    Object.assign(sharedContentScores, await request(`/shared-content-scores${queryString({ ...sharedContentScoreFilters, page, page_size: sharedContentScores.page_size })}`))
  }).catch(() => {})
}

async function loadPlanDataUsers(page = planDataUsers.page) {
  await withLoading('planDataUsers', async () => {
    Object.assign(planDataUsers, await request(`/plan-data-users${queryString({ ...planDataFilters, page, page_size: planDataUsers.page_size })}`))
  }).catch(() => {})
}

async function openPlanDataDetail(item) {
  const detail = await withLoading('planDataDetail', async () => request(`/plan-data-users/${item.uid}`)).catch(() => null)
  if (!detail) return
  planDataSelectedUser.value = item
  planDataDetail.value = detail.folders || []
}

function closePlanDataDetail() {
  planDataSelectedUser.value = null
  planDataDetail.value = []
}

async function loadWorkoutDataUsers(page = workoutDataUsers.page) {
  await withLoading('workoutDataUsers', async () => {
    Object.assign(workoutDataUsers, await request(`/workout-data-users${queryString({ ...workoutDataFilters, page, page_size: workoutDataUsers.page_size })}`))
  }).catch(() => {})
}

async function openWorkoutDataDetail(item) {
  workoutDataSelectedUser.value = item
  await loadWorkoutDataSessions(1)
}

async function loadWorkoutDataSessions(page = workoutDataSessions.page) {
  if (!workoutDataSelectedUser.value) return
  await withLoading('workoutDataSessions', async () => {
    Object.assign(workoutDataSessions, await request(`/workout-data-users/${workoutDataSelectedUser.value.uid}/sessions${queryString({ page, page_size: workoutDataSessions.page_size })}`))
  }).catch(() => {})
}

function closeWorkoutDataDetail() {
  workoutDataSelectedUser.value = null
  Object.assign(workoutDataSessions, { items: [], total: 0, page: 1 })
}

function loadVisibleUserList(page = 1) {
  if (current.value === 'activity') return loadActivities(page)
  if (current.value === 'todayRegistrations') return loadTodayRegistrations(page)
  if (current.value === 'userList') return loadUserList(page)
  return loadRegistrations(page)
}

async function loadOfferReplyStatus() {
  await withLoading('offerReplyStatus', async () => {
    Object.assign(offerReplyStatus, await localOfferReplyRequest('/status'))
  }).catch(() => {})
}

async function loadOfferCodes(page = offerCodes.page) {
  await withLoading('offerCodes', async () => {
    Object.assign(offerCodes, await localOfferReplyRequest(`/codes${queryString({
      ...offerCodeFilters,
      page,
      page_size: offerCodes.page_size,
    })}`))
  }).catch(() => {})
}

async function loadOfferReplyWorkspace(page = 1) {
  await Promise.all([loadOfferReplyStatus(), loadOfferCodes(page)])
}

function useNextOfferReply() {
  if (!offerReplyStatus.next_id) return
  offerReplyInput.value = String(offerReplyStatus.next_id)
}

async function generateOfferReply() {
  const input = offerReplyInput.value.trim()
  if (!input) {
    notify('请输入兑换序号 ID 或兑换码', 'error')
    return
  }
  const result = await withLoading('offerReplyGenerate', async () => {
    const generated = await localOfferReplyRequest('/replies', { method: 'POST', body: { input } })
    Object.assign(offerReplyResult, generated)
    Object.assign(offerReplyStatus, generated.status)
    notify(generated.already_used ? `ID ${generated.id} 已兑换，已重新生成回复` : `ID ${generated.id} 回复已生成并标记为已兑换`)
    return generated
  }).catch(() => null)
  if (result) await loadOfferCodes(offerCodes.page)
}

async function copyOfferReply() {
  if (!offerReplyResult.reply) return
  try {
    await navigator.clipboard.writeText(offerReplyResult.reply)
  } catch {
    const textarea = document.createElement('textarea')
    textarea.value = offerReplyResult.reply
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    textarea.remove()
  }
  notify('回复全文已复制')
}

async function copyOfferCode(code) {
  try {
    await navigator.clipboard.writeText(code)
    notify('兑换码已复制')
  } catch {
    notify('复制失败，请手动选择兑换码', 'error')
  }
}

async function importOfferCSV(file) {
  if (!file) return
  if (!file.name.toLowerCase().endsWith('.csv')) {
    notify('请选择 CSV 文件', 'error')
    return
  }
  const result = await withLoading('offerImport', async () => localOfferReplyRequest('/import', {
    method: 'POST',
    rawBody: await file.arrayBuffer(),
    headers: {
      'Content-Type': 'text/csv; charset=utf-8',
      'X-Offer-Filename': encodeURIComponent(file.name),
    },
  })).catch(() => null)
  if (!result) return
  Object.assign(offerReplyStatus, result.status)
  notify(`导入 ${result.imported_count} 条：新增 ${result.added_count}，更新 ${result.updated_count}，已有 ${result.existing_count}`)
  await loadOfferCodes(1)
}

async function handleOfferCSVChange(event) {
  const file = event.target.files?.[0]
  event.target.value = ''
  await importOfferCSV(file)
}

async function handleOfferCSVDrop(event) {
  offerImportDragging.value = false
  await importOfferCSV(event.dataTransfer?.files?.[0])
}

async function setOfferUsage(range, used, confirmBulk = false) {
  const normalizedRange = String(range || '').trim()
  if (!normalizedRange) {
    notify('请输入兑换序号范围', 'error')
    return
  }
  if (confirmBulk) {
    const action = used ? '标记为已兑换' : '恢复为未兑换'
    if (!window.confirm(`确认将序号 ${normalizedRange} ${action}？`)) return
  }
  const result = await withLoading('offerUsage', async () => localOfferReplyRequest('/usage', {
    method: 'POST',
    body: { range: normalizedRange, used },
  })).catch(() => null)
  if (!result) return
  Object.assign(offerReplyStatus, result.status)
  offerUsageRange.value = ''
  notify(`${used ? '已兑换' : '未兑换'}状态已更新 ${result.changed_count} 条`)
  await loadOfferCodes(offerCodes.page)
}

function offerUsageSourceLabel(source) {
  if (source === 'legacy_state') return '旧状态迁移'
  if (source === 'reply_generated') return '回复生成'
  if (source === 'manual_batch') return '手动设置'
  return '—'
}

async function loadAppUpdate() {
  await withLoading('update', async () => {
    Object.assign(appUpdate, await request('/app-update'))
  }).catch(() => {})
}

async function saveAppUpdate() {
  await withLoading('saveUpdate', async () => {
    Object.assign(appUpdate, await request('/app-update', { method: 'PUT', body: appUpdate }))
    notify('版本更新配置已保存')
  }).catch(() => {})
}

function formatDateTime(value) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
  }).format(date)
}

function formatEpochDateTime(value) {
  if (!value) return '—'
  const timestamp = Number(value)
  return formatDateTime(new Date(timestamp < 100000000000 ? timestamp * 1000 : timestamp))
}

function exerciseName(item) {
  return item.custom_name || item.name_snapshot || item.name_key || item.exercise_id || '未命名动作'
}

function weightUnitLabel(unit) {
  if (unit === 1) return 'kg'
  if (unit === 2) return 'lb'
  return '斤'
}

function weightText(weightX10, unit) {
  return `${Number(weightX10 || 0) / 10}${weightUnitLabel(unit)}`
}

function parseJSON(value) {
  if (!value) return null
  if (typeof value === 'object') return value
  try {
    return JSON.parse(value)
  } catch {
    return null
  }
}

function formatJSON(value) {
  const parsed = parseJSON(value)
  return parsed ? JSON.stringify(parsed, null, 2) : (value || '暂无数据')
}

function onboardingSummary(value) {
  const profile = parseJSON(value)
  if (!profile) return '—'
  return [profile.goal, profile.trainingFocus, profile.experience, profile.theme].filter(Boolean).join(' · ') || '—'
}

function sourceLabel(source, offerType) {
  if (source === 'offer_code' || offerType === 3) return '兑换码'
  return offerType === 0 ? '购买 / 历史未知' : 'Apple 购买'
}

const paywallEntryLabels = {
  onboarding: 'Onboard 页面',
  profile_membership: '我的 · Pro 会员卡',
  profile_theme: '我的 · Pro 主题',
  profile_training_tag_palette: '我的 · 训练标签配色',
  history_pro_button: '历史 · 右上角 Pro',
  history_date_navigation: '历史 · 日期范围',
  history_year_overview: '历史 · 年度总览',
  history_chart: '历史 · Pro 图表',
  today_date_navigation: '今日 · 日期范围',
  health_date_navigation: '健康 · 日期范围',
  health_metric_range: '健康 · 指标时间范围',
  health_analysis: '健康分析 · 时间范围',
  friend_radar_range: '好友 · 雷达范围',
  friend_training_history: '好友 · 历史训练',
  plan_library_limit: '计划数量不足',
  share_plan: '分享计划',
  share_training: '分享训练',
  save_friend_training_as_plan: '好友动作训练存为计划',
  create_custom_exercise: '创建自定义动作',
  custom_exercise_introduction: '自定义动作介绍',
  custom_training_tag: '自定义训练标签',
  workout_note: '训练备注',
  body_media: '身体照片动态媒体',
}

function paywallEntryLabel(entry) {
  if (!entry) return '历史版本 / 未记录'
  return paywallEntryLabels[entry] || entry
}

function paywallSessionStatusLabel(status) {
  if (status === 'purchased') return '已购买'
  if (status === 'cancelled') return '已取消'
  return '默认（含杀进程）'
}

function paywallSessionStatusClass(status) {
  if (status === 'purchased') return 'positive'
  if (status === 'cancelled') return 'negative'
  return 'warning'
}

function vipKindLabel(kind) {
  if (kind === 'lifetime') return '永久'
  if (kind === 'monthly') return '限时'
  return '无'
}

function pageCount(data) {
  return Math.max(1, Math.ceil(data.total / data.page_size))
}

let unsubscribeAdminRoute = () => {}

onMounted(async () => {
  unsubscribeAdminRoute = subscribeAdminRoute((routeID) => {
    routeStatus.current_route = routeID
  })
  await checkConnection()
  if (connected.value) await loadOverview()
})

onUnmounted(() => unsubscribeAdminRoute())
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar">
      <div class="brand-block">
        <div class="brand-mark">S</div>
        <div>
          <strong>Spider Admin</strong>
          <span>Operations Console</span>
        </div>
      </div>

      <nav class="nav-list" aria-label="后台导航">
        <section v-for="group in navGroups" :key="group.label" class="nav-group" :class="{ expanded: isNavGroupExpanded(group.label) }">
          <button class="nav-group-toggle" type="button" :aria-expanded="isNavGroupExpanded(group.label)" @click="toggleNavGroup(group.label)">
            <span>{{ group.label }}</span><ChevronDown :size="15" />
          </button>
          <div class="nav-group-items">
            <button
              v-for="item in group.items"
              :key="item.id"
              class="nav-item"
              :class="{ active: current === item.id }"
              type="button"
              @click="selectSection(item.id)"
            >
              <component :is="item.icon" :size="18" />
              <span>{{ item.label }}</span>
            </button>
          </div>
        </section>
      </nav>

      <div class="sidebar-footer">
        <ShieldCheck :size="17" />
        <span>HMAC / HTTPS</span>
      </div>
    </aside>

    <main class="main-area">
      <header class="topbar">
        <div>
          <p class="eyebrow">SPIDER SERVER</p>
          <h1>{{ currentTitle }}</h1>
        </div>
        <div class="topbar-status">
          <div class="route-state" :title="routeStatus.mode === 'automatic' ? '系统会测量两条线路并自动选择可用且更快的线路' : '已固定使用当前线路'">
            <span>{{ routeStatus.mode === 'automatic' ? '自动选线' : (routeStatus.mode === 'single' ? '单线路' : '手动选线') }} · {{ currentRouteLabel }}</span>
            <a v-if="currentRouteURL" class="route-url" :href="currentRouteURL" target="_blank" rel="noopener noreferrer">{{ currentRouteURL }}</a>
          </div>
          <div class="segmented route-switch" aria-label="远程线路选择">
            <button type="button" :class="{ selected: routeStatus.mode === 'automatic' || routeStatus.mode === 'single' }" :disabled="loading.route" @click="changeRouteMode('automatic')">自动</button>
            <button type="button" :class="{ selected: routeStatus.mode === 'line1' }" :disabled="loading.route || !hasConfiguredRoute('line1')" @click="changeRouteMode('line1')">线路一</button>
            <button type="button" :class="{ selected: routeStatus.mode === 'line2' }" :disabled="loading.route || !hasConfiguredRoute('line2')" @click="changeRouteMode('line2')">线路二</button>
          </div>
          <div class="connection-state" :class="{ online: connected }">
            <span class="status-dot"></span>
            {{ connected ? '远程服务已连接' : '远程服务未连接' }}
            <button class="icon-button" type="button" title="重新连接并重新检测线路" @click="checkConnection">
              <RefreshCw :size="16" :class="{ spin: loading.health }" />
            </button>
          </div>
        </div>
      </header>

      <section v-if="current === 'overview'" class="page-section">
        <div class="section-toolbar">
          <div>
            <h2>今日数据</h2>
            <p>{{ today }}</p>
          </div>
          <button class="icon-button bordered" type="button" title="刷新" @click="loadOverview">
            <RefreshCw :size="17" :class="{ spin: loading.overview }" />
          </button>
        </div>

        <div class="metric-grid">
          <article class="metric-card accent-teal">
            <Activity :size="20" />
            <span>日活用户</span>
            <strong>{{ overview.daily_active }}</strong>
          </article>
          <article class="metric-card accent-blue">
            <CalendarPlus :size="20" />
            <span>新增注册</span>
            <strong>{{ overview.registrations }}</strong>
          </article>
          <article class="metric-card accent-ink">
            <BadgeDollarSign :size="20" />
            <span>Apple 交易</span>
            <strong>{{ overview.payments }}</strong>
          </article>
          <button class="metric-card accent-amber overview-metric-action" type="button" @click="selectSection('feedback')">
            <MessageSquareText :size="20" />
            <span>历史用户反馈</span>
            <strong>{{ overview.feedback }}</strong>
          </button>
          <button class="metric-card accent-red overview-metric-action" type="button" @click="selectSection('update')">
            <Settings2 :size="20" />
            <span>版本更新</span>
            <strong>{{ overview.latest_version ? `v${overview.latest_version}` : '未配置' }}</strong>
          </button>
        </div>

        <div class="overview-functions">
          <div class="overview-functions-heading"><h2>全部功能</h2><p>按分类快速进入管理模块</p></div>
          <section v-for="group in navGroups" :key="group.label" class="overview-function-group">
            <h3>{{ group.label }}</h3>
            <div class="quick-actions">
              <button v-for="item in group.items" :key="item.id" type="button" @click="selectSection(item.id)">
                <component :is="item.icon" :size="18" />{{ item.label }}
              </button>
            </div>
          </section>
        </div>
      </section>

      <section v-else-if="current === 'vip'" class="page-section">
        <form class="search-bar" @submit.prevent="searchUser">
          <Search :size="18" />
          <input v-model="userIdentifier" placeholder="账号、UID（如 29）或 SP 用户 ID" autocomplete="off" />
          <button class="primary-button" type="submit" :disabled="loading.user">
            <LoaderCircle v-if="loading.user" :size="17" class="spin" />
            查询用户
          </button>
        </form>

        <div v-if="user" class="user-workspace">
          <div class="user-summary">
            <div class="user-heading">
              <div class="avatar">{{ (user.nickname || user.account || 'U').slice(0, 1).toUpperCase() }}</div>
              <div>
                <h2>{{ user.nickname || user.account }}</h2>
                <p>UID {{ user.uid }} · {{ user.account }}</p>
              </div>
              <span class="status-pill" :class="user.vip.is_vip ? 'positive' : 'neutral'">
                {{ user.vip.is_vip ? `${vipKindLabel(user.vip.kind)} Pro` : '非 Pro' }}
              </span>
            </div>

            <dl class="detail-grid">
              <div><dt>权益来源</dt><dd>{{ user.vip.source || '—' }}</dd></div>
              <div><dt>到期时间</dt><dd>{{ formatDateTime(user.vip.expires_at) }}</dd></div>
              <div><dt>Apple 邮箱</dt><dd>{{ user.apple_email || '—' }}</dd></div>
              <div><dt>最后进入</dt><dd>{{ formatDateTime(user.last_app_enter_at) }}</dd></div>
              <div><dt>当前客户端版本</dt><dd>{{ user.last_app_version || '—' }}</dd></div>
              <div><dt>注册设备</dt><dd>{{ user.register_device_label || '—' }}</dd></div>
              <div><dt>注册系统</dt><dd>{{ user.register_ios_version || '—' }}</dd></div>
              <div><dt>登录设备</dt><dd>{{ user.last_login_device_label || '—' }}</dd></div>
              <div><dt>最后登录</dt><dd>{{ formatDateTime(user.last_login_at) }}</dd></div>
              <div><dt>系统语言</dt><dd>{{ user.last_system_language || '—' }}</dd></div>
              <div><dt>注册时间</dt><dd>{{ formatDateTime(user.created_at) }}</dd></div>
            </dl>
          </div>

          <aside class="action-panel">
            <h3>后台开通 Pro</h3>
            <div class="segmented duration-grid">
              <button
                v-for="option in grantOptions"
                :key="option.id"
                type="button"
                :class="{ selected: grantOption === option.id }"
                @click="grantOption = option.id"
              >{{ option.label }}</button>
            </div>
            <label>操作人<input v-model="operator" autocomplete="off" /></label>
            <label>备注<input v-model="grantReason" autocomplete="off" /></label>
            <button class="primary-button full" type="button" :disabled="loading.grant" @click="grantVIP">
              <Gift :size="17" />开通 Pro
            </button>
            <button class="danger-button full" type="button" :disabled="loading.revoke" @click="revokeVIP">
              <LogOut :size="17" />撤销后台开通
            </button>
          </aside>
        </div>
      </section>

      <section v-else-if="current === 'payments'" class="page-section">
        <div class="filter-toolbar">
          <div class="segmented">
            <button type="button" :class="{ selected: paymentFilters.source === 'all' }" @click="paymentFilters.source = 'all'; loadPayments(1)">全部</button>
            <button type="button" :class="{ selected: paymentFilters.source === 'purchase' }" @click="paymentFilters.source = 'purchase'; loadPayments(1)">Apple 购买</button>
            <button type="button" :class="{ selected: paymentFilters.source === 'offer_code' }" @click="paymentFilters.source = 'offer_code'; loadPayments(1)">兑换码</button>
          </div>
          <select v-model="paymentFilters.entry_point" @change="loadPayments(1)">
            <option value="all">全部付费墙入口</option>
            <option v-for="(label, value) in paywallEntryLabels" :key="value" :value="value">{{ label }}</option>
          </select>
          <input v-model="paymentFilters.from" type="date" />
          <input v-model="paymentFilters.to" type="date" />
          <div class="compact-search"><Search :size="16" /><input v-model="paymentFilters.search" placeholder="UID / 账号 / 交易号 / 入口" @keyup.enter="loadPayments(1)" /></div>
          <button class="primary-button" type="button" @click="loadPayments(1)">查询</button>
        </div>
        <div class="result-meta">共 {{ payments.total }} 条</div>
        <div class="table-wrap">
          <table>
            <thead><tr><th>用户</th><th>来源</th><th>付费墙入口</th><th>商品</th><th>购买时间</th><th>到期时间</th><th>状态</th><th>交易号</th></tr></thead>
            <tbody>
              <tr v-for="item in payments.items" :key="item.id">
                <td><strong>{{ item.nickname || item.account || `UID ${item.uid}` }}</strong><small>UID {{ item.uid }}</small></td>
                <td><span class="source-tag" :class="item.source">{{ sourceLabel(item.source, item.offer_type) }}</span></td>
                <td>
                  <strong>{{ paywallEntryLabel(item.paywall_entry_point) }}</strong>
                  <small v-if="item.purchased_before_login">先购买、后登录补绑 UID</small>
                  <small v-if="item.paywall_presentation_id" class="mono">{{ item.paywall_presentation_id }}</small>
                </td>
                <td>{{ item.product_id }}<small v-if="item.offer_identifier">{{ item.offer_identifier }}</small></td>
                <td>{{ formatDateTime(item.purchase_at) }}</td>
                <td>{{ formatDateTime(item.expires_at) }}</td>
                <td><span class="status-pill" :class="item.revocation_at ? 'negative' : 'positive'">{{ item.revocation_at ? '已退款' : '有效记录' }}</span></td>
                <td class="mono">{{ item.transaction_id }}</td>
              </tr>
              <tr v-if="!payments.items?.length"><td colspan="8" class="empty-cell">暂无记录</td></tr>
            </tbody>
          </table>
        </div>
        <Pagination :data="payments" :page-count="pageCount(payments)" @change="loadPayments" />
      </section>

      <section v-else-if="current === 'paywallSessions'" class="page-section">
        <div class="filter-toolbar">
          <div class="segmented">
            <button type="button" :class="{ selected: paywallSessionFilters.status === 'all' }" @click="paywallSessionFilters.status = 'all'; loadPaywallSessions(1)">全部</button>
            <button type="button" :class="{ selected: paywallSessionFilters.status === 'default' }" @click="paywallSessionFilters.status = 'default'; loadPaywallSessions(1)">默认 / 杀进程</button>
            <button type="button" :class="{ selected: paywallSessionFilters.status === 'cancelled' }" @click="paywallSessionFilters.status = 'cancelled'; loadPaywallSessions(1)">已取消</button>
            <button type="button" :class="{ selected: paywallSessionFilters.status === 'purchased' }" @click="paywallSessionFilters.status = 'purchased'; loadPaywallSessions(1)">已购买</button>
          </div>
          <select v-model="paywallSessionFilters.entry_point" @change="loadPaywallSessions(1)">
            <option value="all">全部弹出原因</option>
            <option v-for="(label, value) in paywallEntryLabels" :key="value" :value="value">{{ label }}</option>
          </select>
          <input v-model="paywallSessionFilters.from" type="date" />
          <input v-model="paywallSessionFilters.to" type="date" />
          <div class="compact-search"><Search :size="16" /><input v-model="paywallSessionFilters.search" placeholder="UID / 账号 / 设备 ID / 会话 / 商品" @keyup.enter="loadPaywallSessions(1)" /></div>
          <button class="primary-button" type="button" @click="loadPaywallSessions(1)">查询</button>
        </div>
        <div class="result-meta">共 {{ paywallSessions.total }} 次付费墙展示；默认状态表示尚未收到取消或购买更新，其中包含杀进程场景。</div>
        <div class="table-wrap">
          <table>
            <thead><tr><th>用户</th><th>设备 ID</th><th>弹出原因</th><th>当前状态</th><th>弹出时间</th><th>状态时间</th><th>商品</th><th>版本</th><th>会话标识</th></tr></thead>
            <tbody>
              <tr v-for="item in paywallSessions.items" :key="item.id">
                <td>
                  <strong>{{ item.uid ? (item.nickname || item.account || `UID ${item.uid}`) : '未登录用户' }}</strong>
                  <small>UID {{ item.uid }}</small>
                </td>
                <td class="mono">{{ item.device_unique_id || '历史版本未记录' }}</td>
                <td><strong>{{ paywallEntryLabel(item.entry_point) }}</strong><small class="mono">{{ item.entry_point }}</small></td>
                <td><span class="status-pill" :class="paywallSessionStatusClass(item.status)">{{ paywallSessionStatusLabel(item.status) }}</span></td>
                <td>{{ formatDateTime(item.presented_at) }}</td>
                <td>{{ formatDateTime(item.status_changed_at) }}</td>
                <td>{{ item.product_id || '—' }}</td>
                <td>{{ item.app_version || '—' }}</td>
                <td class="mono">{{ item.presentation_id }}<small>{{ item.anonymous_id }}</small></td>
              </tr>
              <tr v-if="!paywallSessions.items?.length"><td colspan="9" class="empty-cell">暂无付费墙会话</td></tr>
            </tbody>
          </table>
        </div>
        <Pagination :data="paywallSessions" :page-count="pageCount(paywallSessions)" @change="loadPaywallSessions" />
      </section>

      <section v-else-if="current === 'refunds'" class="page-section">
        <div class="filter-toolbar">
          <div class="segmented">
            <button type="button" :class="{ selected: refundFilters.status === 'requested' }" @click="refundFilters.status = 'requested'; loadRefunds(1)">已申请</button>
            <button type="button" :class="{ selected: refundFilters.status === 'completed' }" @click="refundFilters.status = 'completed'; loadRefunds(1)">已退款</button>
          </div>
          <select v-model="refundFilters.source" @change="loadRefunds(1)"><option value="all">全部来源</option><option value="purchase">Apple 购买</option><option value="offer_code">兑换码</option></select>
          <input v-model="refundFilters.from" type="date" />
          <input v-model="refundFilters.to" type="date" />
          <div class="compact-search"><Search :size="16" /><input v-model="refundFilters.search" placeholder="UID / 账号 / 交易号" @keyup.enter="loadRefunds(1)" /></div>
          <button class="primary-button" type="button" @click="loadRefunds(1)">查询</button>
        </div>
        <div class="result-meta">共 {{ refunds.total }} 条</div>
        <div class="table-wrap">
          <table>
            <thead><tr><th>用户</th><th>退款状态</th><th>来源</th><th>商品</th><th>申请时间</th><th>退款时间</th><th>处理状态</th><th>交易号</th></tr></thead>
            <tbody>
              <tr v-for="item in refunds.items" :key="`${item.status}-${item.id}`">
                <td><strong>{{ item.nickname || item.account || `UID ${item.uid}` }}</strong><small>UID {{ item.uid }}</small></td>
                <td><span class="status-pill" :class="item.status === 'completed' ? 'negative' : 'warning'">{{ item.status === 'completed' ? '已退款' : '退款申请' }}</span></td>
                <td><span class="source-tag" :class="item.source">{{ sourceLabel(item.source, item.offer_type) }}</span></td>
                <td>{{ item.product_id }}</td>
                <td>{{ formatDateTime(item.requested_at) }}</td>
                <td>{{ formatDateTime(item.revocation_at) }}</td>
                <td>{{ item.processing_status || '—' }}</td>
                <td class="mono">{{ item.transaction_id }}</td>
              </tr>
              <tr v-if="!refunds.items?.length"><td colspan="8" class="empty-cell">暂无记录</td></tr>
            </tbody>
          </table>
        </div>
        <Pagination :data="refunds" :page-count="pageCount(refunds)" @change="loadRefunds" />
      </section>

      <section v-else-if="current === 'activity' || current === 'todayRegistrations' || current === 'userList' || current === 'registrations'" class="page-section">
        <div class="filter-toolbar">
          <template v-if="current === 'activity'">
            <input v-model="activityFilters.date" type="date" />
          </template>
          <template v-else-if="current === 'registrations'">
            <input v-model="registrationFilters.from" type="date" />
            <input v-model="registrationFilters.to" type="date" />
          </template>
          <div class="compact-search"><Search :size="16" /><input v-model="activeUserFilters.search" placeholder="UID / 账号 / 昵称" /></div>
          <button class="primary-button" type="button" @click="loadVisibleUserList(1)">查询</button>
        </div>
        <div class="result-meta">
          共 {{ visibleUserList.total }} 人
          <span v-if="current === 'todayRegistrations'"> · {{ today }}</span>
          <span v-else-if="current === 'userList'"> · 按注册时间倒序</span>
        </div>
        <div class="table-wrap">
          <table>
            <thead><tr><th>用户</th><th>账号</th><th>设备</th><th>iOS</th><th>{{ current === 'activity' ? '首次活跃' : '注册时间' }}</th><th>{{ current === 'activity' ? '最后活跃' : '最后进入' }}</th><th>语言</th></tr></thead>
            <tbody>
              <tr v-for="item in visibleUserList.items" :key="`${current}-${item.uid}`">
                <td><strong>{{ item.nickname || `UID ${item.uid}` }}</strong><small>UID {{ item.uid }}</small></td>
                <td>{{ item.account }}</td>
                <td>{{ item.register_device_label || '—' }}</td>
                <td>{{ item.register_ios_version || '—' }}</td>
                <td>{{ formatDateTime(current === 'activity' ? item.first_seen_at : item.created_at) }}</td>
                <td>{{ formatDateTime(current === 'activity' ? item.last_seen_at : item.last_app_enter_at) }}</td>
                <td>{{ item.last_system_language || '—' }}</td>
              </tr>
              <tr v-if="!visibleUserList.items?.length"><td colspan="7" class="empty-cell">暂无记录</td></tr>
            </tbody>
          </table>
        </div>
        <Pagination
          :data="visibleUserList"
          :page-count="pageCount(visibleUserList)"
          @change="loadVisibleUserList"
        />
      </section>

      <section v-else-if="current === 'feedback'" class="page-section">
        <div class="filter-toolbar">
          <input v-model="feedbackFilters.from" type="date" />
          <input v-model="feedbackFilters.to" type="date" />
          <div class="compact-search"><Search :size="16" /><input v-model="feedbackFilters.search" placeholder="UID / 账号 / 昵称 / 内容" @keyup.enter="loadFeedback(1)" /></div>
          <button class="primary-button" type="button" @click="loadFeedback(1)">查询</button>
        </div>
        <div class="result-meta">共 {{ feedback.total }} 条</div>
        <div class="table-wrap">
          <table>
            <thead><tr><th>用户</th><th>账号</th><th>反馈内容</th><th>提交时间</th></tr></thead>
            <tbody>
              <tr v-for="item in feedback.items" :key="item.id">
                <td><strong>{{ item.nickname || `UID ${item.uid}` }}</strong><small>UID {{ item.uid }}</small></td>
                <td>{{ item.account || '—' }}</td>
                <td class="feedback-content">{{ item.content }}</td>
                <td>{{ formatDateTime(item.created_at) }}</td>
              </tr>
              <tr v-if="!feedback.items?.length"><td colspan="4" class="empty-cell">暂无反馈</td></tr>
            </tbody>
          </table>
        </div>
        <Pagination :data="feedback" :page-count="pageCount(feedback)" @change="loadFeedback" />
      </section>

      <section v-else-if="current === 'syncFailures'" class="page-section">
        <div class="filter-toolbar">
          <div class="segmented">
            <button type="button" :class="{ selected: syncFailureFilters.status === 'pending' }" @click="syncFailureFilters.status = 'pending'; loadSyncFailures(1)">待处理</button>
            <button type="button" :class="{ selected: syncFailureFilters.status === 'resolved' }" @click="syncFailureFilters.status = 'resolved'; loadSyncFailures(1)">已处理</button>
            <button type="button" :class="{ selected: syncFailureFilters.status === 'all' }" @click="syncFailureFilters.status = 'all'; loadSyncFailures(1)">全部</button>
          </div>
          <input v-model="syncFailureFilters.from" type="date" title="最后失败开始日期" />
          <input v-model="syncFailureFilters.to" type="date" title="最后失败结束日期" />
          <input v-model="operator" class="operator-input" placeholder="操作人" autocomplete="off" />
          <div class="compact-search"><Search :size="16" /><input v-model="syncFailureFilters.search" placeholder="UID / 任务 ID / 接口 / 错误码" @keyup.enter="loadSyncFailures(1)" /></div>
          <button class="primary-button" type="button" @click="loadSyncFailures(1)">查询</button>
        </div>
        <div class="result-meta">共 {{ syncFailures.total }} 条{{ syncFailureFilters.status === 'pending' ? '待处理' : '' }}任务</div>
        <div class="table-wrap">
          <table>
            <thead><tr><th>用户</th><th>队列 / 接口</th><th>业务错误</th><th>最后失败</th><th>尝试</th><th>状态</th><th>操作</th></tr></thead>
            <tbody>
              <template v-for="item in syncFailures.items" :key="item.id">
                <tr>
                  <td><strong>{{ item.nickname || item.account || `UID ${item.uid}` }}</strong><small>UID {{ item.uid }} · 任务 {{ item.id }}</small></td>
                  <td><strong>{{ item.queue_name || '通用队列' }}</strong><small class="mono">{{ item.original_rpc_path }}</small></td>
                  <td><strong>错误码 {{ item.business_code }}</strong><small>{{ item.business_message || '无错误信息' }}</small></td>
                  <td>{{ formatEpochDateTime(item.last_failed_at) }}</td>
                  <td>{{ item.attempt_count }} 次</td>
                  <td><span class="status-pill" :class="item.status === 'resolved' ? 'positive' : 'warning'">{{ item.status === 'resolved' ? '已处理' : '待处理' }}</span></td>
                  <td class="sync-failure-actions">
                    <button class="secondary-button" type="button" @click="expandedSyncFailureID = expandedSyncFailureID === item.id ? 0 : item.id">{{ expandedSyncFailureID === item.id ? '收起' : '查看数据' }}</button>
                    <button v-if="item.status !== 'resolved'" class="primary-button" type="button" :disabled="loading[`resolveSyncFailure-${item.id}`]" @click="resolveSyncFailure(item)"><CheckCheck :size="14" />标记已处理</button>
                  </td>
                </tr>
                <tr v-if="expandedSyncFailureID === item.id" class="expanded-row">
                  <td colspan="7">
                    <div class="record-detail-grid">
                      <div><span>客户端任务 ID</span><strong class="mono">{{ item.client_task_id }}</strong></div>
                      <div><span>客户端创建时间</span><strong>{{ formatEpochDateTime(item.client_created_at) }}</strong></div>
                      <div><span>服务器归档时间</span><strong>{{ formatDateTime(item.created_at) }}</strong></div>
                      <div><span>App 版本</span><strong>{{ item.app_version || '—' }}</strong></div>
                      <div><span>处理信息</span><strong>{{ item.resolved_by ? `${item.resolved_by} · ${formatDateTime(item.resolved_at)}` : '尚未处理' }}</strong></div>
                      <div><span>处理备注</span><strong>{{ item.resolution_note || '—' }}</strong></div>
                    </div>
                    <div class="record-detail-meta">请求接口：{{ item.original_rpc_path }}</div>
                    <pre class="json-viewer">{{ formatJSON(item.request_data_json) }}</pre>
                  </td>
                </tr>
              </template>
              <tr v-if="!syncFailures.items?.length"><td colspan="7" class="empty-cell">暂无丢弃任务</td></tr>
            </tbody>
          </table>
        </div>
        <Pagination :data="syncFailures" :page-count="pageCount(syncFailures)" @change="loadSyncFailures" />
      </section>

      <section v-else-if="current === 'onboarding'" class="page-section">
        <div class="filter-toolbar">
          <input v-model="onboardingFilters.from" type="date" />
          <input v-model="onboardingFilters.to" type="date" />
          <div class="compact-search"><Search :size="16" /><input v-model="onboardingFilters.search" placeholder="UID / 账号 / 昵称" @keyup.enter="loadOnboardingProfiles(1)" /></div>
          <button class="primary-button" type="button" @click="loadOnboardingProfiles(1)">查询</button>
        </div>
        <div class="result-meta">共 {{ onboardingProfiles.total }} 人</div>
        <div class="table-wrap">
          <table>
            <thead><tr><th>用户</th><th>账号</th><th>资料概要</th><th>版本</th><th>完成时间</th><th>操作</th></tr></thead>
            <tbody>
              <template v-for="item in onboardingProfiles.items" :key="item.id">
                <tr>
                  <td><strong>{{ item.nickname || `UID ${item.uid}` }}</strong><small>UID {{ item.uid }}</small></td>
                  <td>{{ item.account || '—' }}</td>
                  <td>{{ onboardingSummary(item.profile_json) }}</td>
                  <td>v{{ item.schema_version }}</td>
                  <td>{{ formatDateTime(item.completed_at) }}</td>
                  <td><button class="secondary-button" type="button" @click="expandedOnboardingID = expandedOnboardingID === item.id ? 0 : item.id">{{ expandedOnboardingID === item.id ? '收起' : '查看内容' }}</button></td>
                </tr>
                <tr v-if="expandedOnboardingID === item.id" class="expanded-row">
                  <td colspan="6">
                    <div class="record-detail-meta">上传 {{ formatDateTime(item.created_at) }} · 更新 {{ formatDateTime(item.updated_at) }}</div>
                    <pre class="json-viewer">{{ formatJSON(item.profile_json) }}</pre>
                  </td>
                </tr>
              </template>
              <tr v-if="!onboardingProfiles.items?.length"><td colspan="6" class="empty-cell">暂无 Onboard 信息</td></tr>
            </tbody>
          </table>
        </div>
        <Pagination :data="onboardingProfiles" :page-count="pageCount(onboardingProfiles)" @change="loadOnboardingProfiles" />
      </section>

      <section v-else-if="current === 'friendProfiles'" class="page-section">
        <div class="filter-toolbar">
          <input v-model="friendProfileFilters.from" type="date" />
          <input v-model="friendProfileFilters.to" type="date" />
          <div class="compact-search"><Search :size="16" /><input v-model="friendProfileFilters.search" placeholder="UID / 账号 / Friend ID / 昵称" @keyup.enter="loadFriendProfiles(1)" /></div>
          <button class="primary-button" type="button" @click="loadFriendProfiles(1)">查询</button>
        </div>
        <div class="result-meta">共 {{ friendProfiles.total }} 人</div>
        <div class="table-wrap">
          <table>
            <thead><tr><th>用户</th><th>Friend ID</th><th>头像</th><th>简介</th><th>训练计划</th><th>训练公开</th><th>创建时间</th><th>操作</th></tr></thead>
            <tbody>
              <template v-for="item in friendProfiles.items" :key="item.id">
                <tr>
                  <td><strong>{{ item.nickname || item.account || `UID ${item.uid}` }}</strong><small>UID {{ item.uid }} · {{ item.account || '无账号' }}</small></td>
                  <td class="mono">{{ item.user_id }}</td>
                  <td>{{ item.avatar_symbol || '—' }}</td>
                  <td>{{ item.bio || '—' }}</td>
                  <td>{{ item.plan_title || '—' }}</td>
                  <td><span class="status-pill" :class="item.training_data_visible ? 'positive' : 'neutral'">{{ item.training_data_visible ? '公开' : '隐藏' }}</span><small>连续 {{ item.spark_days }} 天</small></td>
                  <td>{{ formatDateTime(item.created_at) }}</td>
                  <td><button class="secondary-button" type="button" @click="expandedFriendProfileID = expandedFriendProfileID === item.id ? 0 : item.id">{{ expandedFriendProfileID === item.id ? '收起' : '查看详情' }}</button></td>
                </tr>
                <tr v-if="expandedFriendProfileID === item.id" class="expanded-row">
                  <td colspan="8">
                    <div class="record-detail-grid">
                      <div><span>计划描述</span><strong>{{ item.plan_description || '—' }}</strong></div>
                      <div><span>快照更新时间</span><strong>{{ formatDateTime(item.snapshot_updated_at) }}</strong></div>
                      <div><span>资料更新时间</span><strong>{{ formatDateTime(item.updated_at) }}</strong></div>
                    </div>
                    <pre class="json-viewer">{{ formatJSON(item.recent_training_json) }}</pre>
                  </td>
                </tr>
              </template>
              <tr v-if="!friendProfiles.items?.length"><td colspan="8" class="empty-cell">暂无好友资料</td></tr>
            </tbody>
          </table>
        </div>
        <Pagination :data="friendProfiles" :page-count="pageCount(friendProfiles)" @change="loadFriendProfiles" />
      </section>

      <section v-else-if="current === 'featureAdoption'" class="page-section">
        <div class="filter-toolbar">
          <input v-model="featureAdoptionFilters.from" type="date" />
          <input v-model="featureAdoptionFilters.to" type="date" />
          <button class="primary-button" type="button" @click="loadFeatureAdoption(1)">查询</button>
        </div>
        <div class="result-meta">共 {{ featureAdoption.total }} 个有新增记录的日期</div>
        <div class="table-wrap feature-adoption-table">
          <table>
            <thead><tr><th>日期</th><th>体重用户</th><th>标签用户</th><th>动作组用户</th><th>新增动作组</th><th>新增计划</th><th>更新计划</th><th>照片用户</th></tr></thead>
            <tbody>
              <tr v-for="item in featureAdoption.items" :key="item.date">
                <td><strong>{{ item.date }}</strong></td>
                <td><strong class="daily-count">{{ item.weight_users }}</strong><small>UID 去重</small></td>
                <td><strong class="daily-count">{{ item.training_tag_users }}</strong><small>UID 去重</small></td>
                <td><strong class="daily-count">{{ item.exercise_set_users }}</strong><small>UID 去重</small></td>
                <td><strong class="daily-count">{{ item.exercise_set_count }}</strong><small>UID 去重</small></td>
                <td><strong class="daily-count">{{ item.created_plan_count }}</strong><small>UID 去重</small></td>
                <td><strong class="daily-count">{{ item.updated_plan_count }}</strong><small>UID 去重</small></td>
                <td><strong class="daily-count">{{ item.body_photo_users }}</strong><small>UID 去重</small></td>
              </tr>
              <tr v-if="!featureAdoption.items?.length"><td colspan="8" class="empty-cell">暂无新增记录</td></tr>
            </tbody>
          </table>
        </div>
        <Pagination :data="featureAdoption" :page-count="pageCount(featureAdoption)" @change="loadFeatureAdoption" />
      </section>

      <section v-else-if="current === 'shareScores'" class="page-section">
        <div class="section-toolbar">
          <div><h2>分享使用积分</h2><p>好友真正保存计划或训练后计 1 分；相同用户重复确认使用也会继续计分。</p></div>
          <button class="icon-button bordered" type="button" title="刷新" @click="loadSharedContentScores"><RefreshCw :size="17" :class="{ spin: loading.sharedContentScores }" /></button>
        </div>
        <div class="filter-toolbar">
          <div class="segmented">
            <button type="button" :class="{ selected: sharedContentScoreFilters.kind === 'plan' }" @click="sharedContentScoreFilters.kind = 'plan'; loadSharedContentScores(1)">计划</button>
            <button type="button" :class="{ selected: sharedContentScoreFilters.kind === 'training' }" @click="sharedContentScoreFilters.kind = 'training'; loadSharedContentScores(1)">动作训练</button>
          </div>
          <div class="compact-search"><Search :size="16" /><input v-model="sharedContentScoreFilters.search" placeholder="来源 UID / 来源 ID / 标题" @keyup.enter="loadSharedContentScores(1)" /></div>
          <button class="primary-button" type="button" @click="loadSharedContentScores(1)">查询</button>
        </div>
        <div class="result-meta">共 {{ sharedContentScores.total }} 条 · 服务端按积分、最近使用时间和记录 ID 依次倒序</div>
        <div class="table-wrap">
          <table>
            <thead><tr><th>排名</th><th>内容</th><th>来源用户</th><th>积分</th><th>最近使用</th></tr></thead>
            <tbody>
              <tr v-for="(item, index) in sharedContentScores.items" :key="item.id">
                <td><strong>{{ (sharedContentScores.page - 1) * sharedContentScores.page_size + index + 1 }}</strong></td>
                <td><strong>{{ item.title || (sharedContentScoreFilters.kind === 'plan' ? '未命名计划' : '动作训练') }}</strong><small>{{ item.source_id }}</small></td>
                <td><strong>UID {{ item.source_uid }}</strong></td>
                <td><strong class="daily-count">{{ item.score }}</strong><small>使用积分</small></td>
                <td>{{ formatDateTime(item.last_used_at) }}</td>
              </tr>
              <tr v-if="!sharedContentScores.items?.length"><td colspan="5" class="empty-cell">暂无分享使用积分</td></tr>
            </tbody>
          </table>
        </div>
        <Pagination :data="sharedContentScores" :page-count="pageCount(sharedContentScores)" @change="loadSharedContentScores" />
      </section>

      <section v-else-if="current === 'planData'" class="page-section">
        <template v-if="planDataSelectedUser">
          <div class="section-toolbar">
            <div>
              <button class="back-button" type="button" @click="closePlanDataDetail"><ArrowLeft :size="16" />返回用户列表</button>
              <h2>{{ planDataSelectedUser.nickname || planDataSelectedUser.account || `UID ${planDataSelectedUser.uid}` }} 的训练计划</h2>
              <p>UID {{ planDataSelectedUser.uid }} · 当前保存的计划文件夹与动作配置</p>
            </div>
            <button class="icon-button bordered" type="button" title="刷新" @click="openPlanDataDetail(planDataSelectedUser)"><RefreshCw :size="17" :class="{ spin: loading.planDataDetail }" /></button>
          </div>
          <div v-if="planDataDetail.length" class="plan-folder-list">
            <article v-for="folder in planDataDetail" :key="folder.id" class="data-detail-card">
              <div class="data-detail-heading">
                <div><span>计划文件夹</span><h3>{{ folder.title || '未命名文件夹' }}</h3></div>
                <small>更新 {{ formatEpochDateTime(folder.updated_at) }}</small>
              </div>
              <div v-if="folder.plans?.length" class="plan-list">
                <article v-for="plan in folder.plans" :key="plan.id" class="plan-card">
                  <div class="plan-card-heading"><div><strong>{{ plan.title || '未命名计划' }}</strong><small>更新 {{ formatEpochDateTime(plan.updated_at) }}</small></div><span>{{ plan.exercises?.length || 0 }} 个动作</span></div>
                  <div class="exercise-plan-list">
                    <div v-for="exercise in plan.exercises" :key="exercise.id" class="exercise-plan-row">
                      <div><strong>{{ exerciseName(exercise) }}</strong><small>{{ exercise.exercise_id }}</small></div>
                      <div><span>计划组数</span><strong>{{ exercise.set_count }} 组</strong></div>
                      <div><span>重量 × 次数</span><strong>{{ exercise.sets?.length ? exercise.sets.map((set) => `${set.weight_text || '—'} × ${set.reps_text || '—'}`).join(' · ') : '未填写' }}</strong></div>
                    </div>
                  </div>
                </article>
              </div>
              <p v-else class="empty-detail">此文件夹暂无计划。</p>
            </article>
          </div>
          <div v-else class="empty-state">暂无可展示的计划内容。</div>
        </template>
        <template v-else>
          <div class="section-toolbar">
            <div><h2>用户训练计划</h2><p>仅显示保存了计划文件夹或计划的用户，最近变更在前。</p></div>
            <button class="icon-button bordered" type="button" title="刷新" @click="loadPlanDataUsers"><RefreshCw :size="17" :class="{ spin: loading.planDataUsers }" /></button>
          </div>
          <div class="filter-toolbar"><div class="compact-search"><Search :size="16" /><input v-model="planDataFilters.search" placeholder="UID / 账号 / 昵称" @keyup.enter="loadPlanDataUsers(1)" /></div><button class="primary-button" type="button" @click="loadPlanDataUsers(1)">查询</button></div>
          <div class="result-meta">共 {{ planDataUsers.total }} 位有训练计划数据的用户 · 按最近变更时间倒序</div>
          <div class="table-wrap"><table><thead><tr><th>用户</th><th>最近变更</th><th>操作</th></tr></thead><tbody>
            <tr v-for="item in planDataUsers.items" :key="item.uid"><td><strong>{{ item.nickname || item.account || `UID ${item.uid}` }}</strong><small>UID {{ item.uid }}</small></td><td>{{ formatDateTime(item.latest_data_at) }}</td><td><button class="secondary-button" type="button" @click="openPlanDataDetail(item)">查看计划详情</button></td></tr>
            <tr v-if="!planDataUsers.items?.length"><td colspan="3" class="empty-cell">暂无训练计划数据</td></tr>
          </tbody></table></div>
          <Pagination :data="planDataUsers" :page-count="pageCount(planDataUsers)" @change="loadPlanDataUsers" />
        </template>
      </section>

      <section v-else-if="current === 'workoutData'" class="page-section">
        <template v-if="workoutDataSelectedUser">
          <div class="section-toolbar">
            <div><button class="back-button" type="button" @click="closeWorkoutDataDetail"><ArrowLeft :size="16" />返回用户列表</button><h2>{{ workoutDataSelectedUser.nickname || workoutDataSelectedUser.account || `UID ${workoutDataSelectedUser.uid}` }} 的锻炼记录</h2><p>每次训练按结束时间倒序，展示动作、组数、重量和次数。</p></div>
            <button class="icon-button bordered" type="button" title="刷新" @click="loadWorkoutDataSessions(1)"><RefreshCw :size="17" :class="{ spin: loading.workoutDataSessions }" /></button>
          </div>
          <div v-if="workoutDataSessions.items?.length" class="workout-session-list">
            <article v-for="session in workoutDataSessions.items" :key="session.id" class="data-detail-card">
              <div class="data-detail-heading"><div><span>{{ session.standalone ? '自由训练' : (session.plan_title || '计划训练') }}</span><h3>{{ formatEpochDateTime(session.ended_at) }}</h3></div><small>{{ session.actions?.length || 0 }} 个动作 · {{ session.actions?.reduce((total, action) => total + action.set_count, 0) || 0 }} 组</small></div>
              <div class="workout-action-list"><div v-for="action in session.actions" :key="`${session.id}-${action.exercise_id}`" class="workout-action-row"><div><strong>{{ exerciseName(action) }}</strong><small>{{ action.exercise_id }} · {{ action.set_count }} 组</small></div><div class="workout-set-values"><span v-for="(set, index) in action.sets" :key="index">{{ weightText(set.weight_x10, set.weight_unit) }} × {{ set.reps }} 次</span></div></div></div>
            </article>
          </div>
          <div v-else class="empty-state">暂无可展示的锻炼记录。</div>
          <Pagination :data="workoutDataSessions" :page-count="pageCount(workoutDataSessions)" @change="loadWorkoutDataSessions" />
        </template>
        <template v-else>
          <div class="section-toolbar"><div><h2>用户锻炼记录</h2><p>仅显示有已完成锻炼记录的用户，最近一次锻炼在前。</p></div><button class="icon-button bordered" type="button" title="刷新" @click="loadWorkoutDataUsers"><RefreshCw :size="17" :class="{ spin: loading.workoutDataUsers }" /></button></div>
          <div class="filter-toolbar"><div class="compact-search"><Search :size="16" /><input v-model="workoutDataFilters.search" placeholder="UID / 账号 / 昵称" @keyup.enter="loadWorkoutDataUsers(1)" /></div><button class="primary-button" type="button" @click="loadWorkoutDataUsers(1)">查询</button></div>
          <div class="result-meta">共 {{ workoutDataUsers.total }} 位有锻炼记录的用户 · 按最近锻炼时间倒序</div>
          <div class="table-wrap"><table><thead><tr><th>用户</th><th>最近锻炼</th><th>操作</th></tr></thead><tbody>
            <tr v-for="item in workoutDataUsers.items" :key="item.uid"><td><strong>{{ item.nickname || item.account || `UID ${item.uid}` }}</strong><small>UID {{ item.uid }}</small></td><td>{{ formatDateTime(item.latest_data_at) }}</td><td><button class="secondary-button" type="button" @click="openWorkoutDataDetail(item)">查看锻炼详情</button></td></tr>
            <tr v-if="!workoutDataUsers.items?.length"><td colspan="3" class="empty-cell">暂无锻炼记录数据</td></tr>
          </tbody></table></div>
          <Pagination :data="workoutDataUsers" :page-count="pageCount(workoutDataUsers)" @change="loadWorkoutDataUsers" />
        </template>
      </section>

      <section v-else-if="current === 'offerReply'" class="page-section offer-reply-section">
        <div class="section-toolbar">
          <div>
            <h2>LiftTags 兑换码</h2>
            <p>Admin 本地主库</p>
          </div>
          <span class="local-only-badge"><ShieldCheck :size="14" />仅本机</span>
        </div>

        <div
          class="offer-import-zone"
          :class="{ dragging: offerImportDragging }"
          @dragenter.prevent="offerImportDragging = true"
          @dragover.prevent="offerImportDragging = true"
          @dragleave.prevent="offerImportDragging = false"
          @drop.prevent="handleOfferCSVDrop"
        >
          <FileUp :size="22" />
          <div><strong>拖入兑换码 CSV</strong><span>相同兑换码保留序号和使用状态，新兑换码自动追加</span></div>
          <button class="secondary-button" type="button" :disabled="loading.offerImport" @click="offerCSVInput?.click()">
            <LoaderCircle v-if="loading.offerImport" :size="16" class="spin" />
            <FileUp v-else :size="16" />选择文件
          </button>
          <input ref="offerCSVInput" class="visually-hidden" type="file" accept=".csv,text/csv" @change="handleOfferCSVChange" />
        </div>

        <div class="offer-status-grid" aria-label="兑换码回复状态">
          <div><span>兑换码总数</span><strong>{{ offerReplyStatus.total_count }}</strong></div>
          <div><span>已兑换</span><strong>{{ offerReplyStatus.used_count }}</strong></div>
          <div><span>未兑换</span><strong>{{ offerReplyStatus.unused_count }}</strong></div>
          <div><span>建议下一个</span><strong>{{ offerReplyStatus.next_id || '全部完成' }}</strong></div>
        </div>

        <div class="offer-reply-workspace">
          <form class="offer-input-panel" @submit.prevent="generateOfferReply">
            <div class="panel-heading">
              <span>01</span>
              <h3>选择兑换码</h3>
            </div>
            <label>
              兑换序号 ID / 兑换码
              <input v-model="offerReplyInput" autocomplete="off" spellcheck="false" placeholder="例如：6" />
            </label>
            <div class="offer-input-actions">
              <button class="secondary-button" type="button" :disabled="!offerReplyStatus.next_id" @click="useNextOfferReply">
                填入下一个
              </button>
              <button class="primary-button" type="submit" :disabled="loading.offerReplyGenerate">
                <LoaderCircle v-if="loading.offerReplyGenerate" :size="17" class="spin" />
                <MessageSquareReply v-else :size="17" />
                生成回复
              </button>
            </div>
          </form>

          <div class="offer-output-panel">
            <div class="panel-heading output-heading">
              <span>02</span>
              <div>
                <h3>回复内容</h3>
                <p v-if="offerReplyResult.id">
                  ID {{ offerReplyResult.id }}
                  <b v-if="offerReplyResult.already_used">· 此兑换码此前已兑换</b>
                </p>
              </div>
              <button class="secondary-button copy-reply-button" type="button" :disabled="!offerReplyResult.reply" @click="copyOfferReply">
                <ClipboardCopy :size="16" />复制全文
              </button>
            </div>
            <textarea v-model="offerReplyResult.reply" readonly placeholder="等待生成回复" aria-label="生成的英文回复"></textarea>
          </div>
        </div>

        <div class="offer-library-header">
          <div><h3>所有兑换码</h3><p>共 {{ offerCodes.total }} 条匹配记录</p></div>
          <div class="offer-batch-actions">
            <input v-model="offerUsageRange" placeholder="序号范围，如 1-99" @keyup.enter="setOfferUsage(offerUsageRange, true, true)" />
            <button class="primary-button" type="button" :disabled="loading.offerUsage" @click="setOfferUsage(offerUsageRange, true, true)">
              <CheckCheck :size="16" />标记已兑换
            </button>
            <button class="secondary-button" type="button" :disabled="loading.offerUsage" @click="setOfferUsage(offerUsageRange, false, true)">
              <RotateCcw :size="16" />恢复未兑换
            </button>
          </div>
        </div>

        <div class="filter-toolbar offer-code-filters">
          <div class="segmented">
            <button type="button" :class="{ selected: offerCodeFilters.status === 'all' }" @click="offerCodeFilters.status = 'all'; loadOfferCodes(1)">全部</button>
            <button type="button" :class="{ selected: offerCodeFilters.status === 'unused' }" @click="offerCodeFilters.status = 'unused'; loadOfferCodes(1)">未兑换</button>
            <button type="button" :class="{ selected: offerCodeFilters.status === 'used' }" @click="offerCodeFilters.status = 'used'; loadOfferCodes(1)">已兑换</button>
          </div>
          <div class="compact-search"><Search :size="16" /><input v-model="offerCodeFilters.search" placeholder="序号 / 兑换码 / 链接" @keyup.enter="loadOfferCodes(1)" /></div>
          <button class="secondary-button" type="button" @click="loadOfferCodes(1)"><Search :size="16" />查询</button>
        </div>

        <div class="table-wrap offer-code-table">
          <table>
            <thead><tr><th>序号</th><th>兑换码</th><th>兑换链接</th><th>兑换状态</th><th>状态来源</th><th>更新时间</th><th>操作</th></tr></thead>
            <tbody>
              <tr v-for="item in offerCodes.items" :key="item.id">
                <td class="mono">{{ item.id }}</td>
                <td class="offer-code-cell"><strong>{{ item.code }}</strong><button class="icon-button" type="button" title="复制兑换码" @click="copyOfferCode(item.code)"><ClipboardCopy :size="14" /></button></td>
                <td class="offer-url-cell"><a :href="item.url" target="_blank" rel="noopener noreferrer">{{ item.url }}</a></td>
                <td><span class="status-pill" :class="item.used ? 'negative' : 'positive'">{{ item.used ? '已兑换' : '未兑换' }}</span></td>
                <td>{{ offerUsageSourceLabel(item.used_source) }}</td>
                <td>{{ formatDateTime(item.used_at || item.updated_at) }}</td>
                <td>
                  <button v-if="item.used" class="icon-button bordered" type="button" title="恢复未兑换" @click="setOfferUsage(String(item.id), false)"><RotateCcw :size="14" /></button>
                  <button v-else class="icon-button bordered" type="button" title="标记已兑换" @click="setOfferUsage(String(item.id), true)"><CheckCheck :size="14" /></button>
                </td>
              </tr>
              <tr v-if="!offerCodes.items?.length"><td colspan="7" class="empty-cell">暂无兑换码</td></tr>
            </tbody>
          </table>
        </div>
        <Pagination :data="offerCodes" :page-count="pageCount(offerCodes)" @change="loadOfferCodes" />
      </section>

      <WorkoutExploreEditor v-else-if="current === 'workoutExplore'" />

      <ExerciseLibraryImportTool v-else-if="current === 'exerciseLibraryImport'" @notify="notify" />

      <section v-else-if="current === 'update'" class="page-section update-section">
        <div class="section-toolbar">
          <div><h2>iOS 版本配置</h2><p>数据库实时配置</p></div>
          <button class="primary-button" type="button" :disabled="loading.saveUpdate" @click="saveAppUpdate"><Save :size="17" />保存配置</button>
        </div>
        <div class="form-grid">
          <label>最新版本<input v-model="appUpdate.latest_version" placeholder="1.0.8" /></label>
          <label>最低支持版本<input v-model="appUpdate.min_supported_version" placeholder="1.0.6" /></label>
          <label class="wide">App Store URL<div class="input-with-icon"><input v-model="appUpdate.app_store_url" /><ExternalLink :size="17" /></div></label>
          <label class="toggle-row"><span>允许显示更新提示</span><input v-model="appUpdate.update_available_enabled" type="checkbox" /></label>
          <label class="toggle-row"><span>强制更新</span><input v-model="appUpdate.force_update_enabled" type="checkbox" /></label>
        </div>
        <div class="message-grid">
          <label>简体中文<textarea v-model="appUpdate.message_zh_hans" rows="3"></textarea></label>
          <label>繁体中文<textarea v-model="appUpdate.message_zh_hant" rows="3"></textarea></label>
          <label>English<textarea v-model="appUpdate.message_en" rows="3"></textarea></label>
          <label>日本語<textarea v-model="appUpdate.message_ja" rows="3"></textarea></label>
          <label>한국어<textarea v-model="appUpdate.message_ko" rows="3"></textarea></label>
        </div>
      </section>
    </main>

    <transition name="request-loading">
      <div v-if="isRequesting" class="request-loading-mask" role="status" aria-live="polite" aria-label="正在等待服务器返回">
        <div class="request-loading-card">
          <LoaderCircle :size="28" class="spin" />
          <strong>正在同步数据</strong>
          <span>请等待服务器返回…</span>
        </div>
      </div>
    </transition>

    <transition name="toast">
      <div v-if="toast.visible" class="toast" :class="toast.type">{{ toast.message }}</div>
    </transition>
  </div>
</template>
