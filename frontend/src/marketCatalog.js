import { ref, shallowRef } from 'vue'
import { fetchMarketCatalog } from './api'
import { prepareGoods } from './marketSearch'
import { t } from './i18n'

const CATALOG_TTL_MS = 10 * 60 * 1000

export const marketGoods = shallowRef(null)
export const marketGoodsLoading = ref(false)
export const marketGoodsError = ref(null)

let byKey = new Map()
let loadedAt = 0
let pending = null

export function ensureMarketGoods() {
  if (pending) return pending
  if (marketGoods.value && Date.now() - loadedAt < CATALOG_TTL_MS) return Promise.resolve(marketGoods.value)
  marketGoodsLoading.value = true
  marketGoodsError.value = null
  pending = fetchMarketCatalog()
    .then((payload) => {
      const goods = prepareGoods(payload?.goods)
      byKey = new Map(goods.map((good) => [good.key, good]))
      marketGoods.value = goods
      loadedAt = Date.now()
      return goods
    })
    .catch((e) => {
      marketGoodsError.value = e.message || t('common.loadError')
      return marketGoods.value
    })
    .finally(() => {
      marketGoodsLoading.value = false
      pending = null
    })
  return pending
}

export function marketGoodByKey(key) {
  return byKey.get(key) ?? null
}
