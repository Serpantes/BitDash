export function itemCraftKey(itemType, itemId) {
  return `${itemType}:${itemId}`
}

function sumConsumedQuantity(recipe) {
  return (recipe?.consumedItems ?? []).reduce((sum, raw) => sum + (Number(raw.quantity) || 0), 0)
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
      const buildingCmp = String(a.buildingName ?? a.name).localeCompare(String(b.buildingName ?? b.name), 'ru')
      if (buildingCmp !== 0) return buildingCmp
      return (a.buildingTier ?? 0) - (b.buildingTier ?? 0)
    })
}

export function resolveRecipeId(recipes, targetId, preferredRecipeId) {
  const options = listRecipeOptions(recipes, targetId)
  if (!options.length) return null
  if (preferredRecipeId && options.some((o) => String(o.id) === String(preferredRecipeId))) {
    return String(preferredRecipeId)
  }
  const picked = pickCraftingRecipe(recipes, targetId)
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
    : pickCraftingRecipe(list, id)

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
