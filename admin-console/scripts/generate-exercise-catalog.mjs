import { mkdir, readFile, readdir, writeFile } from 'node:fs/promises'
import { basename, dirname, join, relative, resolve, sep } from 'node:path'

const source = process.argv[2]
const output = resolve(process.argv[3] || 'public/exercise-catalog.json')
const gifRoot = resolve(process.env.EXERCISE_GIF_ROOT || '../../zlx/spider/spider/Resources/ExerciseGIFs')
const localizationRoot = resolve(process.env.EXERCISE_LOCALIZATION_ROOT || '../../zlx/spider/spider/Resources/Localizations')

const localizationFiles = {
  en: 'i18n_en.json',
  'zh-Hans': 'i18n_zh.json',
  'zh-Hant': 'i18n_zh-Hant.json',
  ja: 'i18n_ja.json',
  ko: 'i18n_ko.json',
}

const subcategoryKeysByCategory = {
  exercise_category_chest: ['exercise_chest_upper', 'exercise_chest_middle', 'exercise_chest_lower'],
  exercise_category_shoulder: ['exercise_shoulder_front', 'exercise_shoulder_middle', 'exercise_shoulder_rear'],
  exercise_category_upper_arm: ['watch_tag_biceps', 'watch_tag_triceps'],
  exercise_category_thigh: ['watch_tag_legs', 'watch_tag_glutes'],
}

if (!source) {
  console.error('Usage: npm run catalog:generate -- /path/to/exercise_instruction_mapping.json [output]')
  process.exit(1)
}

const mapping = JSON.parse(await readFile(resolve(source), 'utf8'))
if (!mapping.projectActions || typeof mapping.projectActions !== 'object') {
  throw new Error('Mapping file does not contain projectActions')
}

const localizations = Object.fromEntries(await Promise.all(
  Object.entries(localizationFiles).map(async ([locale, filename]) => [
    locale,
    JSON.parse(await readFile(join(localizationRoot, filename), 'utf8')),
  ]),
))

function displayTypeKey(typeKey) {
  switch (typeKey) {
    case 'exercise_type_ez_bar': return 'exercise_type_barbell'
    case 'exercise_type_cable': return 'exercise_type_rope'
    case 'exercise_type_sled':
    case 'exercise_type_stationary_bike':
    case 'exercise_type_stair_climber':
    case 'exercise_type_elliptical':
      return 'exercise_type_machine'
    case 'exercise_type_assisted':
    case 'exercise_type_bosu_ball':
    case 'exercise_type_medicine_ball':
    case 'exercise_type_stability_ball':
    case 'exercise_type_roller':
      return 'exercise_type_other'
    default: return typeKey
  }
}

function localizedValues(key) {
  return Object.fromEntries(Object.entries(localizations).map(([locale, values]) => [
    locale,
    String(values[key] || key),
  ]))
}

function localizedActionNames(action) {
  const names = action.projectLibraryDisplayNames || action.projectDetailDisplayNames || {}
  return {
    en: String(names.en || ''),
    'zh-Hans': String(names['zh-Hans'] || names.zh || ''),
    'zh-Hant': String(names['zh-Hant'] || ''),
    ja: String(names.ja || ''),
    ko: String(names.ko || ''),
  }
}

function containsAny(value, keywords) {
  return keywords.some(keyword => value.includes(keyword))
}

function subcategoryKey(category, rawSearchableText) {
  const searchableText = rawSearchableText.toLocaleLowerCase().trim()
  if (category === 'exercise_category_chest') {
    if (containsAny(searchableText, ['upper chest', 'incline', 'low fly', 'low cable fly', 'decline push-up', 'decline push up', '上胸'])) return 'exercise_chest_upper'
    if (containsAny(searchableText, ['lower chest', 'decline', 'dip', '下胸'])) return 'exercise_chest_lower'
    return 'exercise_chest_middle'
  }
  if (category === 'exercise_category_shoulder') {
    if (containsAny(searchableText, ['rear', 'reverse fly', 'revers fly', 'rear delt', 'rear deltoid', 'deltoid rear', 'behind neck', 'behind head', '后束', '後束', '后三角', '後三角'])) return 'exercise_shoulder_rear'
    if (containsAny(searchableText, ['front raise', 'forward raise', 'front shoulder', 'arnold press', 'shoulder press', 'overhead press', 'military press', 'push press', 'thruster', 'jerk', 'snatch', 'clean', 'w-press', '前束'])) return 'exercise_shoulder_front'
    return 'exercise_shoulder_middle'
  }
  if (category === 'exercise_category_upper_arm') {
    if (containsAny(searchableText, ['bicep', 'biceps', 'curl', 'preacher', 'hammer', '弯举', '彎舉', '二头', '二頭'])) return 'watch_tag_biceps'
    return 'watch_tag_triceps'
  }
  if (category === 'exercise_category_thigh') {
    if (containsAny(searchableText, ['glute', 'glutes', 'gluteus', 'piriformis', 'hip thrust', 'hip lift', 'hip extension', 'hip abduction', 'hip adduction', 'hip internal rotation', 'outer hip', 'bridge', 'reverse hyper', 'pull through', 'monster walk', 'pelvic tilt', 'lying lifting (on hip)', 'twist hip lift', '臀'])) return 'watch_tag_glutes'
    return 'watch_tag_legs'
  }
  return ''
}

async function collectGIFPaths(directory) {
  const paths = new Map()
  async function walk(current) {
    for (const entry of await readdir(current, { withFileTypes: true })) {
      const fullPath = join(current, entry.name)
      if (entry.isDirectory()) {
        await walk(fullPath)
      } else if (entry.isFile() && entry.name.toLocaleLowerCase().endsWith('.gif')) {
        const id = basename(entry.name, '.gif')
        paths.set(id, relative(directory, fullPath).split(sep).join('/'))
      }
    }
  }
  await walk(directory)
  return paths
}

const gifPaths = await collectGIFPaths(gifRoot)
const items = Object.entries(mapping.projectActions).map(([id, action]) => {
  const category = action.projectCategoryKey || ''
  const sourceType = action.projectTypeKey || ''
  const type = displayTypeKey(sourceType)
  const subcategory = subcategoryKey(category, `${action.projectNameKey || ''} ${id}`)
  const categorySubcategories = subcategoryKeysByCategory[category] || []
  return {
    id,
    nameKey: action.projectNameKey || '',
    category,
    categoryNames: localizedValues(category),
    subcategory,
    subcategoryNames: subcategory ? localizedValues(subcategory) : {},
    subcategoryRank: subcategory ? categorySubcategories.indexOf(subcategory) : -1,
    sourceType,
    type,
    typeNames: localizedValues(type),
    gifPath: gifPaths.get(id) || '',
    names: localizedActionNames(action),
  }
}).sort((left, right) => left.id.localeCompare(right.id, 'en'))

await mkdir(dirname(output), { recursive: true })
await writeFile(output, `${JSON.stringify({ version: 1, items }, null, 2)}\n`)
console.log(`Generated ${items.length} actions at ${output}`)
console.log(`Matched ${items.filter(item => item.gifPath).length} GIF files from ${gifRoot}`)
console.log(`Loaded client taxonomy labels from ${localizationRoot}`)
