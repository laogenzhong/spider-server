#!/usr/bin/env node

import { mkdir, readFile, readdir, writeFile } from 'node:fs/promises'
import { basename, dirname, join, relative, resolve, sep } from 'node:path'

const adminRoot = resolve(import.meta.dirname, '..')
const localEnvironment = Object.fromEntries((await readFile(join(adminRoot, '.env.local'), 'utf8').catch(() => ''))
  .split(/\r?\n/)
  .map((line) => line.trim())
  .filter((line) => line && !line.startsWith('#') && line.includes('='))
  .map((line) => {
    const [key, ...value] = line.split('=')
    return [key.trim(), value.join('=').trim()]
  }))
const localSetting = (key) => process.env[key] || localEnvironment[key] || ''
const configuredClientRoot = localSetting('SPIDER_CLIENT_ROOT')
if (!configuredClientRoot) {
  throw new Error('缺少 SPIDER_CLIENT_ROOT。请在 admin-console/.env.local 中配置客户端项目根目录。')
}
const clientRoot = resolve(configuredClientRoot)
const output = resolve(process.argv[2] || 'public/exercise-catalog.json')
const gifRoot = resolve(localSetting('EXERCISE_GIF_ROOT') || join(clientRoot, 'spider/Resources/ExerciseGIFs'))
const catalogConfigPath = join(gifRoot, 'exercise-library-catalog.json')
const catalogSourcePath = join(clientRoot, 'spider/Views/Profile/ExerciseGIFCatalog.swift')
const localizationRoot = join(clientRoot, 'spider/Resources/Localizations')
const instructionRoot = join(clientRoot, 'ExerciseInstructions')
const localizationFiles = {
  en: 'i18n_en.json',
  'zh-Hans': 'i18n_zh.json',
  'zh-Hant': 'i18n_zh-Hant.json',
  ja: 'i18n_ja.json',
  ko: 'i18n_ko.json',
}
const instructionFolders = { en: 'en', 'zh-Hans': 'zh', 'zh-Hant': 'zh-Hant', ja: 'ja', ko: 'ko' }

async function collectGIFPaths(directory) {
  const paths = new Map()
  async function walk(current) {
    for (const entry of await readdir(current, { withFileTypes: true })) {
      const fullPath = join(current, entry.name)
      if (entry.isDirectory()) await walk(fullPath)
      else if (entry.isFile() && entry.name.toLocaleLowerCase().endsWith('.gif')) {
        paths.set(basename(entry.name, '.gif'), relative(directory, fullPath).split(sep).join('/'))
      }
    }
  }
  await walk(directory)
  return paths
}

async function collectInstructionProfiles(locale, folder) {
  const profiles = new Map()
  const directory = join(instructionRoot, folder)
  for (const filename of (await readdir(directory)).filter((name) => name.endsWith('.json')).sort()) {
    const document = JSON.parse(await readFile(join(directory, filename), 'utf8'))
    for (const item of document.data || []) {
      const projectID = String(item.mapping?.projectId || item.mapping?.projectGifName || '').trim()
      if (projectID && !profiles.has(projectID)) profiles.set(projectID, item)
    }
  }
  return [locale, profiles]
}

function parseStaticCatalog(source) {
  const descriptorPattern = /\.init\(nameKey:\s*"([^"]+)",\s*categoryKey:\s*"([^"]+)",\s*typeKey:\s*"([^"]+)",\s*gifName:\s*"([^"]+)"\)/g
  const actions = [...source.matchAll(descriptorPattern)].map((match) => ({
    nameKey: match[1], categoryKey: match[2], typeKey: match[3], gifName: match[4],
  }))
  if (actions.length < 1000) throw new Error(`客户端静态动作库解析异常，仅得到 ${actions.length} 条`)
  return actions
}

function parseClientKeyList(source, constantName) {
  const match = source.match(new RegExp(`private static let ${constantName}: \\[String\\] = \\[([\\s\\S]*?)\\n    \\]`))
  if (!match) throw new Error(`无法从客户端解析 ${constantName}`)
  return [...match[1].matchAll(/"([^"]+)"/g)].map((item) => item[1])
}

function parseDisplayTypeMappings(source) {
  const start = source.indexOf('static func displayTypeKey(for typeKey: String)')
  const end = source.indexOf('static func gifURL(', start)
  if (start < 0 || end < 0) throw new Error('无法从客户端解析器械展示分组')
  const body = source.slice(start, end)
  const result = {}
  for (const match of body.matchAll(/case\s+([\s\S]*?):\s*return\s+"([^"]+)"/g)) {
    for (const key of [...match[1].matchAll(/"([^"]+)"/g)].map((item) => item[1])) result[key] = match[2]
  }
  return result
}

async function currentClientActions() {
  const config = await readFile(catalogConfigPath, 'utf8').then(JSON.parse).catch(() => null)
  if (config?.schemaVersion === 1 && config?.mode === 'full' && Array.isArray(config.actions) && config.actions.length) {
    return config.actions
  }
  return parseStaticCatalog(await readFile(catalogSourcePath, 'utf8'))
}

const [clientActions, gifPaths, localizations, instructionProfiles, catalogSource] = await Promise.all([
  currentClientActions(),
  collectGIFPaths(gifRoot),
  Promise.all(Object.entries(localizationFiles).map(async ([locale, filename]) => [
    locale,
    JSON.parse(await readFile(join(localizationRoot, filename), 'utf8')),
  ])).then(Object.fromEntries),
  Promise.all(Object.entries(instructionFolders).map(([locale, folder]) => collectInstructionProfiles(locale, folder))).then(Object.fromEntries),
  readFile(catalogSourcePath, 'utf8'),
])

function localizedValues(key, fallback = '') {
  return Object.fromEntries(Object.entries(localizations).map(([locale, values]) => [locale, String(values[key] || fallback || key)]))
}

function configuredLocaleMap(rawValue, fallback) {
  return Object.fromEntries(Object.keys(localizationFiles).map((locale) => {
    const value = rawValue?.[locale] ?? (locale === 'zh-Hans' ? rawValue?.zh : undefined)
    return [locale, value ?? fallback(locale)]
  }))
}

function mappingLocaleValue(value, locale) {
  return value?.[locale] ?? (locale === 'zh-Hans' ? value?.zh : undefined)
}

function containsAny(value, keywords) {
  return keywords.some((keyword) => value.includes(keyword))
}

function inferredSubcategory(categoryKey, searchableText) {
  const value = searchableText.toLocaleLowerCase()
  if (categoryKey === 'exercise_category_chest') {
    if (containsAny(value, ['upper chest', 'incline', 'low fly', 'low cable fly', 'decline push-up', 'decline push up', '上胸'])) return 'exercise_chest_upper'
    if (containsAny(value, ['lower chest', 'decline', 'dip', '下胸'])) return 'exercise_chest_lower'
    return 'exercise_chest_middle'
  }
  if (categoryKey === 'exercise_category_shoulder') {
    if (containsAny(value, ['rear', 'reverse fly', 'revers fly', 'rear delt', 'rear deltoid', 'deltoid rear', 'behind neck', 'behind head', '后束', '後束'])) return 'exercise_shoulder_rear'
    if (containsAny(value, ['front raise', 'forward raise', 'front shoulder', 'arnold press', 'shoulder press', 'overhead press', 'military press', 'push press', 'thruster', 'jerk', 'snatch', 'clean', 'w-press', '前束'])) return 'exercise_shoulder_front'
    return 'exercise_shoulder_middle'
  }
  if (categoryKey === 'exercise_category_upper_arm') {
    return containsAny(value, ['bicep', 'biceps', 'curl', 'preacher', 'hammer', '弯举', '彎舉', '二头', '二頭']) ? 'watch_tag_biceps' : 'watch_tag_triceps'
  }
  if (categoryKey === 'exercise_category_thigh') {
    return containsAny(value, ['glute', 'glutes', 'gluteus', 'piriformis', 'hip thrust', 'hip lift', 'hip extension', 'hip abduction', 'hip adduction', 'hip internal rotation', 'outer hip', 'bridge', 'reverse hyper', 'pull through', 'monster walk', 'pelvic tilt', '臀']) ? 'watch_tag_glutes' : 'watch_tag_legs'
  }
  return ''
}

const displayTypeMappings = parseDisplayTypeMappings(catalogSource)
const clientTypeKeys = parseClientKeyList(catalogSource, 'typeKeys')

const actions = clientActions.map((raw, index) => {
  const gifName = String(raw.gifName || '').trim()
  const gifRelativePath = String(raw.gifRelativePath || gifPaths.get(gifName) || '').trim()
  const sourceGif = String(raw.sourceGif || gifRelativePath).trim()
  if (!gifName || !raw.nameKey || !raw.categoryKey || !raw.typeKey || !gifRelativePath) {
    throw new Error(`第 ${index + 1} 个客户端动作缺少必要配置`)
  }
  const profiles = Object.fromEntries(Object.keys(localizationFiles).map((locale) => [locale, instructionProfiles[locale].get(gifName)]))
  const names = configuredLocaleMap(raw.names, (locale) => {
    const mappedNames = profiles[locale]?.mapping?.projectDetailDisplayNames || profiles[locale]?.mapping?.localizedNames
    return String(mappingLocaleValue(mappedNames, locale) || localizations[locale][raw.nameKey] || raw.fallbackName || gifName).trim()
  })
  const libraryNames = configuredLocaleMap(raw.libraryNames, (locale) => {
    const mappedNames = profiles[locale]?.mapping?.projectLibraryDisplayNames
    return String(mappingLocaleValue(mappedNames, locale) || names[locale]).trim()
  })
  const fallbackName = String(names.en || raw.fallbackName || localizations.en[raw.nameKey] || gifName).trim()
  const descriptions = configuredLocaleMap(raw.descriptions, () => '')
  const instructions = configuredLocaleMap(raw.instructions, (locale) => Array.isArray(profiles[locale]?.instructions) ? profiles[locale].instructions : [])
  const targetMuscles = configuredLocaleMap(raw.targetMuscles, (locale) => Array.isArray(profiles[locale]?.targetMuscles) ? profiles[locale].targetMuscles : [])
  const secondaryMuscles = configuredLocaleMap(raw.secondaryMuscles, (locale) => Array.isArray(profiles[locale]?.secondaryMuscles) ? profiles[locale].secondaryMuscles : [])
  const englishProfile = profiles.en
  const typeKey = String(raw.typeKey).trim()
  const displayTypeKey = displayTypeMappings[typeKey] || typeKey
  return {
    gifName,
    nameKey: String(raw.nameKey).trim(),
    fallbackName,
    categoryKey: String(raw.categoryKey).trim(),
    typeKey,
    displayTypeKey,
    sourceGif,
    gifRelativePath,
    enabled: raw.enabled !== false,
    sortOrder: Number.isInteger(Number(raw.sortOrder)) ? Number(raw.sortOrder) : index,
    subcategoryKey: String(raw.subcategoryKey || inferredSubcategory(raw.categoryKey, `${raw.nameKey} ${gifName}`)).trim(),
    sourceExerciseID: String(raw.sourceExerciseID || englishProfile?.mapping?.sourceId || englishProfile?.exerciseId || '').trim(),
    sourceName: String(raw.sourceName || englishProfile?.mapping?.sourceName || englishProfile?.name || gifName).trim(),
    sourceType: typeKey,
    names,
    libraryNames,
    descriptions,
    instructions,
    targetMuscles,
    secondaryMuscles,
    categoryNames: localizedValues(raw.categoryKey),
    sourceTypeNames: localizedValues(typeKey),
    typeNames: localizedValues(displayTypeKey),
  }
})

const availableTypeKeys = new Set(actions.map((action) => action.typeKey))
const sourceTypeDefinitions = clientTypeKeys.filter((key) => availableTypeKeys.has(key)).map((key) => {
  const displayKey = displayTypeMappings[key] || key
  return {
    key,
    names: localizedValues(key),
    displayKey,
    displayNames: localizedValues(displayKey),
  }
})
const seenDisplayTypeKeys = new Set()
const typeDefinitions = []
let otherTypeDefinition = null
for (const sourceType of sourceTypeDefinitions) {
  if (seenDisplayTypeKeys.has(sourceType.displayKey)) continue
  seenDisplayTypeKeys.add(sourceType.displayKey)
  const definition = {
    key: sourceType.displayKey,
    names: sourceType.displayNames,
  }
  if (sourceType.displayKey === 'exercise_type_other') otherTypeDefinition = definition
  else typeDefinitions.push(definition)
}
if (otherTypeDefinition) typeDefinitions.push(otherTypeDefinition)

await mkdir(dirname(output), { recursive: true })
await writeFile(output, `${JSON.stringify({
  schemaVersion: 1,
  mode: 'full',
  generatedAt: new Date().toISOString(),
  taxonomy: { types: typeDefinitions, sourceTypes: sourceTypeDefinitions },
  actions,
}, null, 2)}\n`)
console.log(`已从客户端生成 ${actions.length} 个动作的全量后台配置：${output}`)
