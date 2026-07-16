<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import {
  Activity,
  BadgeDollarSign,
  CalendarPlus,
  CheckCheck,
  CircleUserRound,
  ClipboardCopy,
  Clock3,
  ExternalLink,
  FileUp,
  Gift,
  LayoutDashboard,
  LoaderCircle,
  LogOut,
  MessageSquareReply,
  RefreshCw,
  RotateCcw,
  Save,
  Search,
  Settings2,
  ShieldCheck,
  UserRoundPlus,
  UserRoundCheck,
} from 'lucide-vue-next'
import { localOfferReplyRequest, queryString, request } from './api'
import Pagination from './components/Pagination.vue'

const navItems = [
  { id: 'overview', label: '概览', icon: LayoutDashboard },
  { id: 'vip', label: '用户与 Pro', icon: UserRoundCheck },
  { id: 'payments', label: '付费记录', icon: BadgeDollarSign },
  { id: 'refunds', label: '退款用户', icon: RotateCcw },
  { id: 'activity', label: '日活用户', icon: Activity },
  { id: 'todayRegistrations', label: '今日注册', icon: UserRoundPlus },
  { id: 'registrations', label: '每日注册', icon: CalendarPlus },
  { id: 'offerReply', label: '兑换码回复', icon: MessageSquareReply },
  { id: 'update', label: '版本更新', icon: Settings2 },
]

const current = ref('overview')
const connected = ref(false)
const loading = reactive({})
const toast = reactive({ visible: false, type: 'success', message: '' })
const today = localDateString(new Date())
const operator = ref(localStorage.getItem('spider-admin-operator') || 'local_admin')

const overview = reactive({
  daily_active: 0,
  registrations: 0,
  payments: 0,
  refund_requests: 0,
  refunded: 0,
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
const paymentFilters = reactive({ search: '', source: 'all', from: '', to: '' })
const refunds = reactive({ items: [], total: 0, page: 1, page_size: 30 })
const refundFilters = reactive({ search: '', source: 'all', status: 'requested', from: '', to: '' })
const activities = reactive({ items: [], total: 0, page: 1, page_size: 30 })
const activityFilters = reactive({ search: '', date: today })
const todayRegistrations = reactive({ items: [], total: 0, page: 1, page_size: 30 })
const todayRegistrationFilters = reactive({ search: '' })
const registrations = reactive({ items: [], total: 0, page: 1, page_size: 30 })
const registrationFilters = reactive({ search: '', from: today, to: today })

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
const activeUserFilters = computed(() => {
  if (current.value === 'activity') return activityFilters
  if (current.value === 'todayRegistrations') return todayRegistrationFilters
  return registrationFilters
})
const visibleUserList = computed(() => {
  if (current.value === 'activity') return activities
  if (current.value === 'todayRegistrations') return todayRegistrations
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
  try {
    await request('/health')
    connected.value = true
  } catch (error) {
    connected.value = false
    notify(error.message, 'error')
  }
}

async function selectSection(id) {
  current.value = id
  if (id === 'overview') await loadOverview()
  if (id === 'payments') await loadPayments()
  if (id === 'refunds') await loadRefunds()
  if (id === 'activity') await loadActivities()
  if (id === 'todayRegistrations') await loadTodayRegistrations()
  if (id === 'registrations') await loadRegistrations()
  if (id === 'offerReply') await loadOfferReplyWorkspace()
  if (id === 'update') await loadAppUpdate()
}

async function loadOverview() {
  await withLoading('overview', async () => {
    Object.assign(overview, await request(`/overview${queryString({ from: today, to: today })}`))
  }).catch(() => {})
}

async function searchUser() {
  const identifier = userIdentifier.value.trim()
  if (!identifier) return
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

async function loadRegistrations(page = registrations.page) {
  await withLoading('registrations', async () => {
    Object.assign(registrations, await request(`/registrations${queryString({ ...registrationFilters, page, page_size: registrations.page_size })}`))
  }).catch(() => {})
}

function loadVisibleUserList(page = 1) {
  if (current.value === 'activity') return loadActivities(page)
  if (current.value === 'todayRegistrations') return loadTodayRegistrations(page)
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
    notify(generated.already_used ? `ID ${generated.id} 已使用，已重新生成回复` : `ID ${generated.id} 回复已生成并标记为已使用`)
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
    const action = used ? '标记为已使用' : '恢复为未使用'
    if (!window.confirm(`确认将序号 ${normalizedRange} ${action}？`)) return
  }
  const result = await withLoading('offerUsage', async () => localOfferReplyRequest('/usage', {
    method: 'POST',
    body: { range: normalizedRange, used },
  })).catch(() => null)
  if (!result) return
  Object.assign(offerReplyStatus, result.status)
  offerUsageRange.value = ''
  notify(`${used ? '已使用' : '未使用'}状态已更新 ${result.changed_count} 条`)
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

function sourceLabel(source, offerType) {
  if (source === 'offer_code' || offerType === 3) return '兑换码'
  return offerType === 0 ? '购买 / 历史未知' : 'Apple 购买'
}

function vipKindLabel(kind) {
  if (kind === 'lifetime') return '永久'
  if (kind === 'monthly') return '限时'
  return '无'
}

function pageCount(data) {
  return Math.max(1, Math.ceil(data.total / data.page_size))
}

onMounted(async () => {
  await checkConnection()
  if (connected.value) await loadOverview()
})
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
        <button
          v-for="item in navItems"
          :key="item.id"
          class="nav-item"
          :class="{ active: current === item.id }"
          type="button"
          @click="selectSection(item.id)"
        >
          <component :is="item.icon" :size="18" />
          <span>{{ item.label }}</span>
        </button>
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
        <div class="connection-state" :class="{ online: connected }">
          <span class="status-dot"></span>
          {{ connected ? '远程服务已连接' : '远程服务未连接' }}
          <button class="icon-button" type="button" title="重新连接" @click="checkConnection">
            <RefreshCw :size="16" :class="{ spin: loading.health }" />
          </button>
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
          <article class="metric-card accent-amber">
            <Clock3 :size="20" />
            <span>退款申请</span>
            <strong>{{ overview.refund_requests }}</strong>
          </article>
          <article class="metric-card accent-red">
            <RotateCcw :size="20" />
            <span>退款完成</span>
            <strong>{{ overview.refunded }}</strong>
          </article>
        </div>

        <div class="quick-actions">
          <button type="button" @click="selectSection('vip')"><CircleUserRound :size="19" />用户与 Pro</button>
          <button type="button" @click="selectSection('payments')"><BadgeDollarSign :size="19" />付费记录</button>
          <button type="button" @click="selectSection('refunds')"><RotateCcw :size="19" />退款用户</button>
          <button type="button" @click="selectSection('todayRegistrations')"><UserRoundPlus :size="19" />今日注册</button>
          <button type="button" @click="selectSection('offerReply')"><MessageSquareReply :size="19" />兑换码回复</button>
          <button type="button" @click="selectSection('update')"><Settings2 :size="19" />版本更新</button>
        </div>
      </section>

      <section v-else-if="current === 'vip'" class="page-section">
        <form class="search-bar" @submit.prevent="searchUser">
          <Search :size="18" />
          <input v-model="userIdentifier" placeholder="账号、UID 或 SP 用户 ID" autocomplete="off" />
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
          <input v-model="paymentFilters.from" type="date" />
          <input v-model="paymentFilters.to" type="date" />
          <div class="compact-search"><Search :size="16" /><input v-model="paymentFilters.search" placeholder="UID / 账号 / 交易号" @keyup.enter="loadPayments(1)" /></div>
          <button class="primary-button" type="button" @click="loadPayments(1)">查询</button>
        </div>
        <div class="result-meta">共 {{ payments.total }} 条</div>
        <div class="table-wrap">
          <table>
            <thead><tr><th>用户</th><th>来源</th><th>商品</th><th>购买时间</th><th>到期时间</th><th>状态</th><th>交易号</th></tr></thead>
            <tbody>
              <tr v-for="item in payments.items" :key="item.id">
                <td><strong>{{ item.nickname || item.account || `UID ${item.uid}` }}</strong><small>UID {{ item.uid }}</small></td>
                <td><span class="source-tag" :class="item.source">{{ sourceLabel(item.source, item.offer_type) }}</span></td>
                <td>{{ item.product_id }}<small v-if="item.offer_identifier">{{ item.offer_identifier }}</small></td>
                <td>{{ formatDateTime(item.purchase_at) }}</td>
                <td>{{ formatDateTime(item.expires_at) }}</td>
                <td><span class="status-pill" :class="item.revocation_at ? 'negative' : 'positive'">{{ item.revocation_at ? '已退款' : '有效记录' }}</span></td>
                <td class="mono">{{ item.transaction_id }}</td>
              </tr>
              <tr v-if="!payments.items?.length"><td colspan="7" class="empty-cell">暂无记录</td></tr>
            </tbody>
          </table>
        </div>
        <Pagination :data="payments" :page-count="pageCount(payments)" @change="loadPayments" />
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

      <section v-else-if="current === 'activity' || current === 'todayRegistrations' || current === 'registrations'" class="page-section">
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
        <div class="result-meta">共 {{ visibleUserList.total }} 人<span v-if="current === 'todayRegistrations'"> · {{ today }}</span></div>
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
          <div><span>已使用</span><strong>{{ offerReplyStatus.used_count }}</strong></div>
          <div><span>未使用</span><strong>{{ offerReplyStatus.unused_count }}</strong></div>
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
                  <b v-if="offerReplyResult.already_used">· 此兑换码此前已使用</b>
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
              <CheckCheck :size="16" />标记已使用
            </button>
            <button class="secondary-button" type="button" :disabled="loading.offerUsage" @click="setOfferUsage(offerUsageRange, false, true)">
              <RotateCcw :size="16" />恢复未使用
            </button>
          </div>
        </div>

        <div class="filter-toolbar offer-code-filters">
          <div class="segmented">
            <button type="button" :class="{ selected: offerCodeFilters.status === 'all' }" @click="offerCodeFilters.status = 'all'; loadOfferCodes(1)">全部</button>
            <button type="button" :class="{ selected: offerCodeFilters.status === 'unused' }" @click="offerCodeFilters.status = 'unused'; loadOfferCodes(1)">未使用</button>
            <button type="button" :class="{ selected: offerCodeFilters.status === 'used' }" @click="offerCodeFilters.status = 'used'; loadOfferCodes(1)">已使用</button>
          </div>
          <div class="compact-search"><Search :size="16" /><input v-model="offerCodeFilters.search" placeholder="序号 / 兑换码 / 链接" @keyup.enter="loadOfferCodes(1)" /></div>
          <button class="secondary-button" type="button" @click="loadOfferCodes(1)"><Search :size="16" />查询</button>
        </div>

        <div class="table-wrap offer-code-table">
          <table>
            <thead><tr><th>序号</th><th>兑换码</th><th>兑换链接</th><th>使用状态</th><th>状态来源</th><th>更新时间</th><th>操作</th></tr></thead>
            <tbody>
              <tr v-for="item in offerCodes.items" :key="item.id">
                <td class="mono">{{ item.id }}</td>
                <td class="offer-code-cell"><strong>{{ item.code }}</strong><button class="icon-button" type="button" title="复制兑换码" @click="copyOfferCode(item.code)"><ClipboardCopy :size="14" /></button></td>
                <td class="offer-url-cell"><a :href="item.url" target="_blank" rel="noopener noreferrer">{{ item.url }}</a></td>
                <td><span class="status-pill" :class="item.used ? 'negative' : 'positive'">{{ item.used ? '已使用' : '未使用' }}</span></td>
                <td>{{ offerUsageSourceLabel(item.used_source) }}</td>
                <td>{{ formatDateTime(item.used_at || item.updated_at) }}</td>
                <td>
                  <button v-if="item.used" class="icon-button bordered" type="button" title="恢复未使用" @click="setOfferUsage(String(item.id), false)"><RotateCcw :size="14" /></button>
                  <button v-else class="icon-button bordered" type="button" title="标记已使用" @click="setOfferUsage(String(item.id), true)"><CheckCheck :size="14" /></button>
                </td>
              </tr>
              <tr v-if="!offerCodes.items?.length"><td colspan="7" class="empty-cell">暂无兑换码</td></tr>
            </tbody>
          </table>
        </div>
        <Pagination :data="offerCodes" :page-count="pageCount(offerCodes)" @change="loadOfferCodes" />
      </section>

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

    <transition name="toast">
      <div v-if="toast.visible" class="toast" :class="toast.type">{{ toast.message }}</div>
    </transition>
  </div>
</template>
