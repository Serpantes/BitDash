export function itemCraftKey(itemType, itemId) {
  return `${itemType}:${itemId}`
}

function sumConsumedQuantity(recipe) {
  return (recipe?.consumedItems ?? []).reduce((sum, raw) => sum + (Number(raw.quantity) || 0), 0)
}

function isRecraftRecipe(recipe, targetId) {
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

/** Prefer full craft chains for material planning (not recraft/unpack shortcuts). */
export function pickPlannerCraftingRecipe(recipes, targetId) {
  if (!Array.isArray(recipes) || !recipes.length) return null
  const matches = recipes.filter((r) =>
    r.craftedItems?.some((c) => String(c.id) === String(targetId)),
  )
  let pool = matches.length ? matches : recipes

  const noRecraft = pool.filter((r) => !isRecraftRecipe(r, targetId))
  if (noRecraft.length) pool = noRecraft

  const noUnpack = pool.filter((r) => !isUnpackRecipe(r))
  if (noUnpack.length) pool = noUnpack

  return pool.reduce((best, recipe) => {
    if (!best) return recipe
    const bestConsumed = best.consumedItems?.length ?? 0
    const nextConsumed = recipe.consumedItems?.length ?? 0
    if (nextConsumed > bestConsumed) return recipe
    if (nextConsumed < bestConsumed) return best
    const bestQty = sumConsumedQuantity(best)
    const nextQty = sumConsumedQuantity(recipe)
    return nextQty > bestQty ? recipe : best
  }, null)
}

export function pickCraftingRecipe(recipes, targetId) {
  if (!Array.isArray(recipes) || !recipes.length) return null
  const matches = recipes.filter((r) =>
    r.craftedItems?.some((c) => String(c.id) === String(targetId)),
  )
  const pool = matches.length ? matches : recipes
  return pool.reduce((best, recipe) => {
    if (!best) return recipe
    const bestConsumed = best.consumedItems?.length ?? 999
    const nextConsumed = recipe.consumedItems?.length ?? 999
    if (nextConsumed < bestConsumed) return recipe
    if (nextConsumed > bestConsumed) return best
    const bestQty = sumConsumedQuantity(best)
    const nextQty = sumConsumedQuantity(recipe)
    return nextQty < bestQty ? recipe : best
  }, null)
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
    name: recipe.name ?? 'Рецепт',
    outputQuantity: recipeOutputQuantity(recipe, targetId),
    buildingName: recipe.buildingName ?? null,
  }
}

export function summarizeRecipeOption(recipe, targetId) {
  const base = summarizeRecipe(recipe, targetId)
  if (!base) return null
  const tier = recipe.buildingRequirementTier ?? recipe.buildingTier ?? null
  return {
    ...base,
    ingredientCount: recipe.consumedItems?.length ?? 0,
    buildingTier: tier != null && Number(tier) > 0 ? Number(tier) : null,
    timeRequirement: recipe.timeRequirement ?? null,
  }
}

/** Crafting recipes that produce the target item (with ingredients). */
export function listRecipeOptions(recipes, targetId) {
  if (!Array.isArray(recipes) || !recipes.length) return []
  const matches = recipes.filter((r) =>
    r.craftedItems?.some((c) => String(c.id) === String(targetId)),
  )
  const pool = matches.length ? matches : recipes
  return pool
    .filter((r) => r.consumedItems?.length)
    .map((r) => summarizeRecipeOption(r, targetId))
    .filter(Boolean)
    .sort((a, b) => {
      const recipeA = pool.find((r) => String(r.id) === String(a.id))
      const recipeB = pool.find((r) => String(r.id) === String(b.id))
      const aRecraft = recipeA ? isRecraftRecipe(recipeA, targetId) : false
      const bRecraft = recipeB ? isRecraftRecipe(recipeB, targetId) : false
      if (aRecraft !== bRecraft) return aRecraft ? 1 : -1
      const aUnpack = recipeA ? isUnpackRecipe(recipeA) : false
      const bUnpack = recipeB ? isUnpackRecipe(recipeB) : false
      if (aUnpack !== bUnpack) return aUnpack ? 1 : -1
      const buildingCmp = String(a.buildingName ?? a.name).localeCompare(String(b.buildingName ?? b.name), 'ru')
      if (buildingCmp !== 0) return buildingCmp
      return (a.buildingTier ?? 0) - (b.buildingTier ?? 0)
    })
}

export function resolveRecipeId(recipes, targetId, preferredRecipeId) {
  const options = listRecipeOptions(recipes, targetId)
  if (!options.length) return null
  if (preferredRecipeId) {
    const preferred = recipes.find((r) => String(r.id) === String(preferredRecipeId))
    if (preferred && options.some((o) => String(o.id) === String(preferredRecipeId))) {
      const craftOptions = options.filter((o) => {
        const recipe = recipes.find((r) => String(r.id) === String(o.id))
        return recipe && !isRecraftRecipe(recipe, targetId)
      })
      if (!craftOptions.length || !isRecraftRecipe(preferred, targetId)) {
        return String(preferredRecipeId)
      }
    }
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
  }
}

export function aggregateMaterialNeeds(needsMap) {
  return [...needsMap.values()].sort((a, b) => {
    const idDiff = Number(a.id) - Number(b.id)
    if (idDiff !== 0) return idDiff
    return a.itemType - b.itemType
  })
}

function materialKey(type, id) {
  return `${type}:${id}`
}

export function buildCraftTreeFromRecipes({
  itemId,
  itemType,
  quantity,
  recipes,
  recipeId,
  recipeCache = {},
  visited = null,
}) {
  const id = String(itemId ?? '').trim()
  const qty = Math.max(1, Math.floor(Number(quantity) || 1))
  const key = materialKey(itemType, id)
  const pathVisited = visited ?? new Set()

  if (pathVisited.has(key)) {
    return {
      id,
      itemType,
      quantity: qty,
      hasRecipe: false,
      cycle: true,
      children: [],
    }
  }

  const nextVisited = new Set(pathVisited)
  nextVisited.add(key)

  const list = Array.isArray(recipes) ? recipes : []
  const result = computeCraftMaterialsFromRecipes({
    itemId: id,
    itemType,
    quantity: qty,
    recipes: list,
    recipeId,
  })

  if (!result.hasRecipe) {
    return {
      id,
      itemType,
      quantity: qty,
      hasRecipe: false,
      children: [],
    }
  }

  const children = result.materials.map((mat) => {
    const matKey = materialKey(mat.itemType, mat.id)
    const matRecipes = recipeCache[matKey] ?? []
    return buildCraftTreeFromRecipes({
      itemId: mat.id,
      itemType: mat.itemType,
      quantity: mat.quantity,
      recipes: matRecipes,
      recipeId: null,
      recipeCache,
      visited: nextVisited,
    })
  })

  return {
    id,
    itemType,
    quantity: qty,
    hasRecipe: true,
    recipe: result.recipe,
    children,
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

export function collectCraftTreeItemRefs(nodes) {
  const refs = []
  const seen = new Set()
  function walk(list) {
    if (!Array.isArray(list)) return
    for (const node of list) {
      if (!node?.id) continue
      const key = materialKey(node.itemType, node.id)
      if (!seen.has(key)) {
        seen.add(key)
        refs.push({ id: node.id, itemType: node.itemType })
      }
      walk(node.children)
    }
  }
  walk(nodes)
  return refs
}

/** Fetch recipes for all craftable nodes in the tree (breadth via recursion). */
export async function ensureCraftTreeRecipeCache({
  itemId,
  itemType,
  quantity,
  recipes,
  recipeId,
  recipeCache,
  fetchRecipes,
  visited = null,
}) {
  const id = String(itemId ?? '').trim()
  const key = materialKey(itemType, id)
  const pathVisited = visited ?? new Set()
  if (pathVisited.has(key)) return
  const nextVisited = new Set(pathVisited)
  nextVisited.add(key)

  const list = Array.isArray(recipes) ? recipes : []
  const result = computeCraftMaterialsFromRecipes({
    itemId: id,
    itemType,
    quantity,
    recipes: list,
    recipeId,
  })

  if (!result.hasRecipe) return

  for (const mat of result.materials) {
    const matKey = materialKey(mat.itemType, mat.id)
    if (!(matKey in recipeCache)) {
      try {
        recipeCache[matKey] = await fetchRecipes(mat.itemType, mat.id)
      } catch {
        recipeCache[matKey] = []
      }
    }
    const matRecipes = recipeCache[matKey] ?? []
    const sub = computeCraftMaterialsFromRecipes({
      itemId: mat.id,
      itemType: mat.itemType,
      quantity: mat.quantity,
      recipes: matRecipes,
      recipeId: null,
    })
    if (sub.hasRecipe) {
      await ensureCraftTreeRecipeCache({
        itemId: mat.id,
        itemType: mat.itemType,
        quantity: mat.quantity,
        recipes: matRecipes,
        recipeId: null,
        recipeCache,
        fetchRecipes,
        visited: nextVisited,
      })
    }
  }
}

export function computeCraftMaterialsFromRecipes({
  itemId,
  itemType,
  quantity,
  recipes,
  recipeId,
}) {
  const id = String(itemId ?? '').trim()
  if (!/^\d+$/.test(id)) {
    throw new Error('Некорректный ID предмета')
  }

  const qty = Math.max(1, Math.floor(Number(quantity) || 1))
  const list = Array.isArray(recipes) ? recipes : []
  const recipe = recipeId
    ? list.find((r) => String(r.id) === String(recipeId))
    : pickPlannerCraftingRecipe(list, id)

  if (!recipe?.consumedItems?.length) {
    return {
      materials: [{ id, itemType, quantity: qty }],
      recipe: recipe ? summarizeRecipe(recipe, id) : null,
      hasRecipe: false,
    }
  }

  const outputQty = recipeOutputQuantity(recipe, id)
  const batches = Math.ceil(qty / outputQty)
  const needsMap = new Map()

  for (const raw of recipe.consumedItems) {
    const c = normalizeConsumedItem(raw)
    if (!c.id || !/^\d+$/.test(c.id) || c.quantity <= 0) continue
    const amount = c.quantity * batches
    const key = materialKey(c.itemType, c.id)
    const prev = needsMap.get(key)
    if (prev) {
      prev.quantity += amount
      continue
    }
    needsMap.set(key, {
      id: c.id,
      itemType: c.itemType,
      quantity: amount,
    })
  }

  return {
    materials: aggregateMaterialNeeds(needsMap),
    recipe: summarizeRecipe(recipe, id),
    hasRecipe: true,
  }
}

/** Direct recipe ingredients only (no tier/rarity expansion). */
export async function computeCraftMaterials({
  itemId,
  itemType,
  quantity,
  fetchDetails,
  recipeId,
}) {
  const id = String(itemId ?? '').trim()
  if (!/^\d+$/.test(id)) {
    throw new Error('Некорректный ID предмета')
  }

  const data = await fetchDetails(itemType, id)
  const recipes = data.craftingRecipes ?? []
  return computeCraftMaterialsFromRecipes({
    itemId: id,
    itemType,
    quantity,
    recipes,
    recipeId,
  })
}
