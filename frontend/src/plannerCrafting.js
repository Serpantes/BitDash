import { t } from './i18n'

export function itemCraftKey(itemType, itemId) {
  return `${itemType}:${itemId}`
}

export const PLANNER_BASE_OVERRIDE = 'base'

const LEGACY_RECIPE_ID_MAX = 10_000_000
const TREE_ROW_LIMIT = 600

function sumConsumedQuantity(recipe) {
  return (recipe?.consumedItems ?? []).reduce((sum, raw) => sum + (Number(raw.quantity) || 0), 0)
}

function isRecraftRecipe(recipe) {
  if (/recraft/i.test(recipe?.name ?? '')) return true
  const craftedIds = new Set(
    (recipe?.craftedItems ?? []).map((c) => String(c.id ?? c.item_id)),
  )
  for (const raw of recipe?.consumedItems ?? []) {
    const id = String(raw.id ?? raw.item_id ?? '')
    if (craftedIds.has(id)) return true
  }
  return false
}

function isUnpackRecipe(recipe) {
  return /unpack/i.test(recipe?.name ?? '')
}

function isSalvageRecipe(recipe) {
  return /salvage/i.test(recipe?.name ?? '')
}

function isShortcutRecipe(recipe) {
  return isUnpackRecipe(recipe) || isSalvageRecipe(recipe) || isRecraftRecipe(recipe)
}

function craftsTarget(recipe, targetId) {
  return recipe?.craftedItems?.some((c) => String(c.id) === String(targetId)) ?? false
}

function compareRecipePreference(a, b) {
  const aLegacy = Number(a.id) < LEGACY_RECIPE_ID_MAX
  const bLegacy = Number(b.id) < LEGACY_RECIPE_ID_MAX
  if (aLegacy !== bLegacy) return aLegacy ? -1 : 1
  const countDiff = (a.consumedItems?.length ?? 0) - (b.consumedItems?.length ?? 0)
  if (countDiff !== 0) return countDiff
  const qtyDiff = sumConsumedQuantity(a) - sumConsumedQuantity(b)
  if (qtyDiff !== 0) return qtyDiff
  return Number(a.id) - Number(b.id)
}

function craftCandidates(recipes, targetId) {
  if (!Array.isArray(recipes)) return []
  return recipes
    .filter((r) => r.consumedItems?.length && craftsTarget(r, targetId) && !isShortcutRecipe(r))
    .sort(compareRecipePreference)
}

export function pickPlannerCraftingRecipe(recipes, targetId) {
  const [best] = craftCandidates(recipes, targetId)
  if (best) return best
  if (!Array.isArray(recipes)) return null
  return recipes.find((r) => r.consumedItems?.length && craftsTarget(r, targetId)) ?? null
}

export function recipeOutputQuantity(recipe, targetId) {
  if (!recipe) return 1
  const crafted = recipe.craftedItems?.find((c) => String(c.id) === String(targetId)) ?? recipe.craftedItems?.[0]
  const q = Number(crafted?.quantity ?? recipe.outputQuantity)
  return Number.isFinite(q) && q > 0 ? q : 1
}

export function summarizeRecipe(recipe, targetId) {
  if (!recipe) return null
  return {
    id: recipe.id,
    name: recipe.name ?? t('common.recipe'),
    outputQuantity: recipeOutputQuantity(recipe, targetId),
    buildingName: recipe.buildingName ?? null,
  }
}

function recipeSkill(recipe) {
  const req = recipe?.levelRequirements?.[0]
  const skillId = req?.skill_id ?? req?.skill?.id ?? recipe?.experiencePerProgress?.[0]?.skill_id
  if (skillId == null) return { skillId: null, skillLevel: null }
  const level = Number(req?.level)
  return { skillId: Number(skillId), skillLevel: level > 0 ? level : null }
}

export function summarizeRecipeOption(recipe, targetId) {
  const base = summarizeRecipe(recipe, targetId)
  if (!base) return null
  const tier = recipe.buildingRequirementTier ?? recipe.buildingTier ?? null
  const toolTier = Number(recipe.toolRequirements?.[0]?.level)
  return {
    ...base,
    ...recipeSkill(recipe),
    ingredientCount: recipe.consumedItems?.length ?? 0,
    buildingTier: tier != null && Number(tier) > 0 ? Number(tier) : null,
    toolTier: toolTier > 0 ? toolTier : null,
    passive: Boolean(recipe.isPassive),
    timeRequirement: recipe.timeRequirement ?? null,
  }
}

export const SKILL_NAMES = {
  2: 'Forestry',
  3: 'Carpentry',
  4: 'Masonry',
  5: 'Mining',
  6: 'Smithing',
  7: 'Scholar',
  8: 'Leatherworking',
  9: 'Hunting',
  10: 'Tailoring',
  11: 'Farming',
  12: 'Fishing',
  13: 'Cooking',
  14: 'Foraging',
  15: 'Construction',
  17: 'Taming',
  18: 'Slayer',
  19: 'Merchanting',
  21: 'Sailing',
  22: 'Hexite Gathering',
}
const SKILL_ID_BY_NAME = Object.fromEntries(Object.entries(SKILL_NAMES).map(([id, name]) => [name, Number(id)]))

const GATHER_NAME_HINTS = [
  [/\b(ore|chunk|pebbles|braxite|gypsite|stone|boulder|geode)\b/i, 5],
  [/\b(log|bark|sap|trunk|wood)\b/i, 2],
  [/\b(pelt|hide|meat|animal hair|fang|bone)\b/i, 9],
  [/\b(fish|filet|oil|shell|crab|chum|bait)\b/i, 12],
  [/\b(fiber|roots|sand|clay|berry|berries|flower|mushroom|seed|salt|reed|bulb)\b/i, 14],
]

function skillFromBuilding(buildingName) {
  const match = Object.keys(SKILL_ID_BY_NAME).find((name) => (buildingName ?? '').includes(name))
  return match ? SKILL_ID_BY_NAME[match] : null
}

export function gatherSourceFromName(name) {
  const hint = GATHER_NAME_HINTS.find(([re]) => re.test(name ?? ''))
  return hint ? { kind: 'skill', skillId: hint[1], level: null } : { kind: 'other' }
}

export function gatherSourceFromDetails(data) {
  const entity = data?.item ?? data?.cargo ?? null
  let best = null
  for (const ex of data?.extractionRecipes ?? []) {
    const req = ex.levelRequirements?.[0]
    const skillId = req?.skill_id ?? ex.experiencePerProgress?.[0]?.skill_id
    if (skillId == null) continue
    const level = Number(req?.level) > 0 ? Number(req.level) : null
    if (!best || (level ?? 0) < (best.level ?? 0)) best = { kind: 'skill', skillId: Number(skillId), level }
  }
  if (best) return best
  if (entity?.tag === 'Trade Good') return { kind: 'trader' }
  for (const recipe of data?.craftingRecipes ?? []) {
    if (!isUnpackRecipe(recipe)) continue
    const skillId = skillFromBuilding(recipe.buildingName)
    if (skillId) return { kind: 'skill', skillId, level: null }
  }
  return gatherSourceFromName(entity?.name)
}

export function listRecipeOptions(recipes, targetId) {
  if (!Array.isArray(recipes) || !recipes.length) return []
  return recipes
    .filter((r) => r.consumedItems?.length && craftsTarget(r, targetId))
    .sort((a, b) => {
      const aShortcut = isShortcutRecipe(a)
      const bShortcut = isShortcutRecipe(b)
      if (aShortcut !== bShortcut) return aShortcut ? 1 : -1
      return compareRecipePreference(a, b)
    })
    .map((r) => summarizeRecipeOption(r, targetId))
    .filter(Boolean)
}

export function resolveRecipeId(recipes, targetId, preferredRecipeId) {
  const options = listRecipeOptions(recipes, targetId)
  if (!options.length) return null
  if (preferredRecipeId && options.some((o) => String(o.id) === String(preferredRecipeId))) {
    return String(preferredRecipeId)
  }
  const picked = pickPlannerCraftingRecipe(recipes, targetId)
  if (picked?.id) return String(picked.id)
  return String(options[0].id)
}

function normalizeConsumedItem(raw) {
  const id = raw.id ?? raw.item_id
  const itemType = raw.itemType === 1 || raw.item_type === 'cargo' || raw.item_type === 1 ? 1 : 0
  return {
    id: String(id),
    itemType,
    quantity: Number(raw.quantity) || 0,
    name: raw.name ?? null,
    iconAssetName: raw.iconAssetName ?? null,
  }
}

function recipeIngredients(recipe) {
  const byKey = new Map()
  for (const raw of recipe?.consumedItems ?? []) {
    const c = normalizeConsumedItem(raw)
    if (!/^\d+$/.test(c.id) || c.quantity <= 0) continue
    const key = itemCraftKey(c.itemType, c.id)
    const prev = byKey.get(key)
    if (prev) prev.quantity += c.quantity
    else byKey.set(key, { key, ...c })
  }
  return [...byKey.values()]
}

function recipeLoader(recipeCache, fetchRecipes) {
  return async function load(key, itemType, id) {
    if (!(key in recipeCache)) {
      try {
        recipeCache[key] = await fetchRecipes(itemType, id)
      } catch {
        recipeCache[key] = []
      }
    }
    return recipeCache[key] ?? []
  }
}

async function chooseRecipe({ key, id, recipes, preferredId, path, load }) {
  const preferred = preferredId
    ? recipes.find((r) => String(r.id) === String(preferredId) && r.consumedItems?.length)
    : null
  const candidates = craftCandidates(recipes, id).filter((r) => r !== preferred)
  if (preferred) candidates.unshift(preferred)

  let reverseFallback = null
  for (const recipe of candidates) {
    const ingredients = recipeIngredients(recipe)
    if (!ingredients.length) continue
    if (ingredients.some((ing) => ing.key === key || path.has(ing.key))) continue
    const ingredientRecipes = await Promise.all(
      ingredients.map((ing) => load(ing.key, ing.itemType, ing.id)),
    )
    if (recipe === preferred) return recipe
    const reverse = ingredients.some((ing, i) =>
      craftCandidates(ingredientRecipes[i], ing.id).some((r) =>
        recipeIngredients(r).some((x) => x.key === key),
      ),
    )
    if (!reverse) return recipe
    reverseFallback ??= recipe
  }
  return reverseFallback
}

export async function resolveDefaultRecipeId({ target, recipes, recipeCache, fetchRecipes }) {
  const key = itemCraftKey(target.itemType, target.id)
  const recipe = await chooseRecipe({
    key,
    id: String(target.id),
    recipes: Array.isArray(recipes) ? recipes : [],
    preferredId: null,
    path: new Set(),
    load: recipeLoader(recipeCache, fetchRecipes),
  })
  return recipe ? String(recipe.id) : resolveRecipeId(recipes, target.id, null)
}

export async function resolveCraftGraph({
  root,
  rootRecipeId = null,
  overrides = {},
  recipeCache,
  fetchRecipes,
}) {
  const rootKey = itemCraftKey(root.itemType, root.id)
  const nodes = new Map()
  const load = recipeLoader(recipeCache, fetchRecipes)

  async function visit({ itemType, id, name = null, iconAssetName = null }, path) {
    const key = itemCraftKey(itemType, id)
    if (nodes.has(key)) return
    const recipes = await load(key, itemType, id)
    const node = {
      key,
      id: String(id),
      itemType,
      name,
      iconAssetName,
      recipe: null,
      options: listRecipeOptions(recipes, id),
      override: overrides[key] ?? null,
    }
    nodes.set(key, node)

    const preferredId = key === rootKey ? rootRecipeId : overrides[key]
    if (preferredId === PLANNER_BASE_OVERRIDE) return
    node.recipe = await chooseRecipe({ key, id, recipes, preferredId, path, load })
    if (!node.recipe) return

    const nextPath = new Set(path)
    nextPath.add(key)
    for (const ing of recipeIngredients(node.recipe)) {
      await visit(ing, nextPath)
    }
  }

  await visit({ itemType: root.itemType, id: String(root.id) }, new Set())
  return { rootKey, nodes }
}

function consumerFirstOrder(nodes, rootKey) {
  const order = []
  const done = new Set()
  const active = new Set()
  function walk(key) {
    if (done.has(key) || active.has(key)) return
    active.add(key)
    const node = nodes.get(key)
    for (const ing of recipeIngredients(node?.recipe)) walk(ing.key)
    active.delete(key)
    done.add(key)
    order.push(key)
  }
  walk(rootKey)
  return order.reverse()
}

function buildCraftTree(nodes, key, quantity, budget, ancestors = new Set()) {
  const node = nodes.get(key)
  const row = { id: node.id, itemType: node.itemType, quantity, crafts: 0, children: [] }
  budget.rows += 1
  if (!node.recipe || ancestors.has(key)) return row
  const crafts = Math.ceil(quantity / recipeOutputQuantity(node.recipe, node.id))
  row.crafts = crafts
  const nextAncestors = new Set(ancestors)
  nextAncestors.add(key)
  for (const ing of recipeIngredients(node.recipe)) {
    if (budget.rows >= TREE_ROW_LIMIT) {
      row.truncated = true
      break
    }
    row.children.push(buildCraftTree(nodes, ing.key, ing.quantity * crafts, budget, nextAncestors))
  }
  return row
}

export function computeCraftPlan({ graph, quantity, sources = {} }) {
  const { nodes, rootKey } = graph
  const qty = Math.max(1, Math.floor(Number(quantity) || 1))
  const order = consumerFirstOrder(nodes, rootKey)
  const position = new Map(order.map((key, i) => [key, i]))
  const demand = new Map([[rootKey, qty]])
  const steps = []
  const base = []

  for (const key of order) {
    const node = nodes.get(key)
    const needed = demand.get(key) ?? 0
    if (needed <= 0) continue
    if (!node.recipe) {
      base.push({
        key,
        id: node.id,
        itemType: node.itemType,
        quantity: needed,
        override: node.override,
        source: sources[key] ?? gatherSourceFromName(node.name),
      })
      continue
    }
    const outputQty = recipeOutputQuantity(node.recipe, node.id)
    const crafts = Math.ceil(needed / outputQty)
    for (const ing of recipeIngredients(node.recipe)) {
      if ((position.get(ing.key) ?? -1) <= position.get(key)) continue
      demand.set(ing.key, (demand.get(ing.key) ?? 0) + ing.quantity * crafts)
    }
    steps.push({
      key,
      id: node.id,
      itemType: node.itemType,
      needed,
      crafts,
      produced: crafts * outputQty,
      surplus: crafts * outputQty - needed,
      recipe: summarizeRecipeOption(node.recipe, node.id),
      options: node.options,
      override: node.override,
      isRoot: key === rootKey,
    })
  }

  steps.reverse()
  base.sort((a, b) => b.quantity - a.quantity)
  const budget = { rows: 0 }
  const tree = buildCraftTree(nodes, rootKey, qty, budget)

  const rootNode = nodes.get(rootKey)
  return {
    root: {
      id: rootNode.id,
      itemType: rootNode.itemType,
      quantity: qty,
      recipe: rootNode.recipe ? summarizeRecipeOption(rootNode.recipe, rootNode.id) : null,
    },
    hasRecipe: Boolean(rootNode.recipe),
    base,
    steps,
    tree,
  }
}

export function flattenCraftTree(nodes, depth = 0) {
  if (!Array.isArray(nodes)) return []
  const rows = []
  for (const node of nodes) {
    if (!node?.id) continue
    rows.push({ ...node, depth })
    if (node.children?.length) {
      rows.push(...flattenCraftTree(node.children, depth + 1))
    }
  }
  return rows
}

export function collectCraftGraphItemRefs(graph) {
  return [...graph.nodes.values()].map((node) => ({
    id: node.id,
    itemType: node.itemType,
    name: node.name,
    iconAssetName: node.iconAssetName,
  }))
}

const SOURCE_ORDER = { skill: 0, trader: 1, other: 2 }

function groupId(kind, skillId) {
  return kind === 'skill' && skillId != null ? `skill:${skillId}` : kind
}

export function groupBaseResources(base) {
  const groups = new Map()
  for (const item of base ?? []) {
    const source = item.source ?? { kind: 'other' }
    const kind = source.kind === 'skill' && source.skillId == null ? 'other' : source.kind
    const id = groupId(kind, source.skillId)
    if (!groups.has(id)) groups.set(id, { id, kind, skillId: source.skillId ?? null, items: [] })
    groups.get(id).items.push(item)
  }
  return [...groups.values()].sort((a, b) => {
    const kindDiff = SOURCE_ORDER[a.kind] - SOURCE_ORDER[b.kind]
    if (kindDiff !== 0) return kindDiff
    return b.items.length - a.items.length
  })
}

export function groupCraftSteps(steps, tierOf = () => 0) {
  const groups = new Map()
  for (const step of steps ?? []) {
    const skillId = step.recipe?.skillId ?? null
    const id = groupId(skillId != null ? 'skill' : 'other', skillId)
    if (!groups.has(id)) {
      groups.set(id, { id, kind: skillId != null ? 'skill' : 'other', skillId, maxLevel: null, items: [] })
    }
    const group = groups.get(id)
    group.items.push(step)
    const level = step.recipe?.skillLevel ?? null
    if (level != null && (group.maxLevel == null || level > group.maxLevel)) group.maxLevel = level
  }
  const tier = (step) => Math.max(0, Number(tierOf(step)) || 0)
  for (const group of groups.values()) {
    group.items = group.items
      .map((step, index) => ({ step, index }))
      .sort((a, b) => tier(a.step) - tier(b.step) || a.index - b.index)
      .map(({ step }) => step)
  }
  return [...groups.values()]
}
