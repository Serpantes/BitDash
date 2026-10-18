<template>
  <section class="mi" :aria-label="t('marketItem.aria', { name: meta.name })" @keydown.esc="$emit('close')">
    <header class="mi-head">
      <span class="mi-thumb">
        <ItemCard
          variant="mini"
          badge
          :name="meta.name"
          :icon-asset-name="meta.iconAssetName"
          :item-type="meta.itemType"
          :tier="meta.tier"
          :rarity-str="meta.rarityStr"
        />
      </span>
      <span class="mi-title">
        <span class="mi-name" :title="meta.name">{{ meta.name }}</span>
        <span class="mi-meta">{{ goodMeta(meta) || t('marketItem.item') }}</span>
      </span>
      <span v-if="loading && detail" class="mi-spinner" role="status" :aria-label="t('marketItem.updating')"></span>
      <a
        class="bc-icon-btn"
        :href="bitjitaUrl"
        target="_blank"
        rel="noopener noreferrer"
        :title="t('marketItem.openBitjita')"
        :aria-label="t('marketItem.openBitjita')"
      ><AppIcon name="external" :size="14" /></a>
      <button type="button" class="bc-icon-btn" :title="t('common.closeEsc')" :aria-label="t('common.close')" @click="$emit('close')">
        <AppIcon name="close" :size="14" />
      </button>
    </header>

    <div class="mi-body">
      <div v-if="!detail && loading" class="mi-state">{{ t('marketItem.loadingPrices') }}</div>
      <div v-else-if="!detail" class="mi-state">
        <span class="bc-error">{{ t('marketItem.loadFailed', { error }) }}</span>
        <button type="button" class="bc-btn" @click="load()">{{ t('common.retry') }}</button>
      </div>

      <template v-else>
        <div class="mi-kpis">
          <div v-for="kpi in kpis" :key="kpi.id" class="mi-kpi" :class="`mi-kpi--${kpi.id}`" :title="kpi.title">
            <span class="mi-kpi-label">{{ kpi.label }}</span>
            <span class="mi-kpi-value">{{ kpi.value }}<small v-if="kpi.unit">{{ kpi.unit }}</small></span>
            <span class="mi-kpi-sub">{{ kpi.sub }}</span>
          </div>
        </div>

        <ul v-if="places.length" class="mi-mine">
          <li v-for="place in places" :key="place.row.key">
            <b>{{ place.side === 'buy' ? t('marketItem.youBuy') : t('marketItem.youSell') }}</b>
            ×{{ formatAmount(place.row.quantity) }} {{ t('marketBlock.at', { price: formatPrice(place.row.price) }) }}
            <span class="mi-mine-place">— {{ placeText(place) }}</span>
          </li>
        </ul>

        <div class="mi-books">
          <div v-for="book in books" :key="book.side" class="mi-book" :class="`mi-book--${book.side}`">
            <h4 class="mi-section mi-section--book">
              {{ book.title }}
              <span class="mi-section-note">{{ book.rows.length ? t('marketItem.orders', { n: book.rows.length }) : '' }}</span>
            </h4>
            <p v-if="!book.rows.length" class="mi-note">{{ book.empty }}</p>
            <div v-else class="mi-offers">
              <div class="mi-offer mi-offer--head" aria-hidden="true">
                <span>{{ t('marketBlock.price') }}</span><span>{{ t('marketItem.qty') }}</span><span>{{ t('marketItem.where') }}</span>
              </div>
              <div
                v-for="row in book.visible"
                :key="row.key"
                class="mi-offer"
                :class="{ 'mi-offer--mine': row.mine, 'mi-offer--best': row.price === book.rows[0].price }"
                :title="offerTitle(row, book.side)"
              >
                <span class="mi-offer-price">{{ formatPrice(row.price) }}</span>
                <span class="mi-offer-qty">×{{ formatAmount(row.quantity) }}</span>
                <span class="mi-offer-place">
                  {{ row.claim.name || t('marketItem.market') }}<small v-if="row.claim.regionName"> · {{ row.claim.regionName }}</small>
                </span>
                <span v-if="row.mine" class="mi-you">{{ t('marketItem.you') }}</span>
              </div>
              <button
                v-if="book.rows.length > BOOK_ROWS"
                type="button"
                class="mi-more"
                @click="expanded[book.side] = !expanded[book.side]"
              >{{ expanded[book.side] ? t('common.collapse') : t('marketItem.showAll', { n: book.rows.length }) }}</button>
            </div>
          </div>
        </div>

        <div class="mi-block">
          <h4 class="mi-section">
            {{ t('marketItem.dailyPrice') }}
            <span v-if="series.length >= 2" class="mi-section-note">{{ t('marketItem.tradingDays', { n: series.length }) }}</span>
          </h4>
          <MarketPriceChart v-if="series.length >= 2" :points="series" />
          <p v-else class="mi-note">{{ series.length ? t('marketItem.oneDay') : t('marketItem.noTrades') }}</p>
        </div>

        <button v-if="related" type="button" class="mi-related" @click="$emit('open', related.good)">
          <span class="mi-related-thumb">
            <ItemCard
              variant="mini"
              :name="related.good.name"
              :icon-asset-name="related.good.iconAssetName"
              :item-type="related.good.itemType"
              :tier="related.good.tier"
            />
          </span>
          <span class="mi-related-text">
            <span class="mi-related-label">{{ related.label }}</span>
            <span class="mi-related-name">{{ related.good.name }}</span>
          </span>
          <span v-if="related.price" class="mi-related-price">{{ related.price }}</span>
        </button>

        <div v-if="trades.length" class="mi-block">
          <h4 class="mi-section">{{ t('marketItem.recentTrades') }}</h4>
          <div class="mi-trades">
            <div class="mi-trade mi-trade--head" aria-hidden="true">
              <span>{{ t('marketBlock.price') }}</span><span>{{ t('marketItem.qty') }}</span><span>{{ t('marketItem.region') }}</span><span>{{ t('marketItem.when') }}</span>
            </div>
            <div v-for="trade in trades" :key="trade.key" class="mi-trade" :title="tradeTitle(trade)">
              <span class="mi-trade-price">{{ formatPrice(trade.unitPrice) }}</span>
              <span class="mi-trade-qty">×{{ formatAmount(trade.quantity) }}</span>
              <span class="mi-trade-place">{{ trade.regionName || '' }}</span>
              <span class="mi-trade-time">{{ formatAgo(trade.at, now) }}</span>
            </div>
          </div>
        </div>
      </template>
    </div>
  </section>
</template>

<script>
import { computed, reactive, ref, shallowRef, watch } from 'vue'
import { fetchBitjita, postBitjita } from '../api'
import { goodMeta } from '../marketSearch'
import {
  bookPlaces,
  bookRows,
  formatAgo,
  formatAmount,
  formatPrice,
  priceSeries,
  recentTradeRows,
} from '../marketOrders'
import { intlLocale, t } from '../i18n'
import AppIcon from './AppIcon.vue'
import ItemCard from './ItemCard.vue'
import MarketPriceChart from './MarketPriceChart.vue'

const BOOK_ROWS = 5
const TRADE_ROWS = 8

export default {
  name: 'MarketItemPanel',
  components: { AppIcon, ItemCard, MarketPriceChart },
  props: {
    good: { type: Object, required: true },
    playerEntityId: { type: [String, Number], default: null },
    refreshKey: { type: [String, Number], default: null },
  },
  emits: ['close', 'open'],
  setup(props) {
    const detail = shallowRef(null)
    const history = shallowRef(null)
    const prices = shallowRef(null)
    const loading = ref(false)
    const error = ref(null)
    const now = ref(Date.now())
    const expanded = reactive({ sell: false, buy: false })
    let generation = 0

    const kind = computed(() => (Number(props.good.itemType) === 1 ? 'cargo' : 'item'))
    const bitjitaUrl = computed(() => `https://bitjita.com/market/${kind.value}/${encodeURIComponent(props.good.id)}`)

    async function load({ quiet = false } = {}) {
      const good = props.good
      const run = ++generation
      if (!quiet) {
        detail.value = null
        history.value = null
        prices.value = null
        expanded.sell = false
        expanded.buy = false
      }
      loading.value = true
      error.value = null
      const id = String(good.id)
      const base = `/market/${kind.value}/${encodeURIComponent(id)}`
      const isCargo = kind.value === 'cargo'
      const [detailRes, historyRes, pricesRes] = await Promise.allSettled([
        fetchBitjita(base),
        fetchBitjita(`${base}/price-history`, { bucket: '1 day', limit: '30' }),
        postBitjita('/market/prices/bulk', isCargo ? { itemIds: [], cargoIds: [Number(id)] } : { itemIds: [Number(id)], cargoIds: [] }),
      ])
      if (run !== generation) return
      loading.value = false
      now.value = Date.now()
      if (detailRes.status === 'fulfilled') detail.value = detailRes.value
      else if (!detail.value) error.value = detailRes.reason?.message ?? t('common.networkError')
      if (historyRes.status === 'fulfilled') history.value = historyRes.value
      if (pricesRes.status === 'fulfilled') {
        const maps = pricesRes.value?.data ?? {}
        prices.value = (isCargo ? maps.cargo : maps.items)?.[id] ?? null
      }
    }
    watch(() => `${props.good.itemType}:${props.good.id}`, () => load(), { immediate: true })
    watch(() => props.refreshKey, () => { if (detail.value) load({ quiet: true }) })

    const meta = computed(() => {
      const item = detail.value?.item ?? {}
      return {
        id: String(props.good.id),
        itemType: kind.value === 'cargo' ? 1 : 0,
        name: item.name ?? props.good.name ?? `#${props.good.id}`,
        iconAssetName: item.iconAssetName ?? props.good.iconAssetName ?? null,
        tier: item.tier ?? props.good.tier ?? null,
        rarityStr: item.rarityStr ?? props.good.rarityStr ?? null,
        tag: item.tag ?? props.good.tag ?? null,
      }
    })

    const sellBook = computed(() => bookRows(detail.value?.sellOrders, 'sell', props.playerEntityId))
    const buyBook = computed(() => bookRows(detail.value?.buyOrders, 'buy', props.playerEntityId))
    const books = computed(() => [
      { side: 'sell', title: t('marketItem.selling'), empty: t('marketItem.nobodySells'), rows: sellBook.value },
      { side: 'buy', title: t('marketItem.buying'), empty: t('marketItem.nobodyBuys'), rows: buyBook.value },
    ].map((book) => ({ ...book, visible: expanded[book.side] ? book.rows : book.rows.slice(0, BOOK_ROWS) })))

    const places = computed(() => [
      ...bookPlaces(sellBook.value, 'sell').map((place) => ({ ...place, side: 'sell' })),
      ...bookPlaces(buyBook.value, 'buy').map((place) => ({ ...place, side: 'buy' })),
    ])

    const series = computed(() => priceSeries(history.value?.priceData))
    const trades = computed(() => recentTradeRows(history.value?.recentTrades).slice(0, TRADE_ROWS))

    const kpis = computed(() => {
      const stats = detail.value?.stats ?? {}
      const priceStats = history.value?.priceStats ?? {}
      const sellCount = Number(stats.sellOrderCount ?? sellBook.value.length) || 0
      const buyCount = Number(stats.buyOrderCount ?? buyBook.value.length) || 0
      const avg7 = prices.value?.vwap7d ?? priceStats.avg7d
      const avg24 = prices.value?.vwap24h ?? priceStats.avg24h
      const volume7 = (Number(priceStats.soldVolume7d) || 0) + (Number(priceStats.boughtVolume7d) || 0)
      const volume24 = (Number(priceStats.soldVolume24h) || 0) + (Number(priceStats.boughtVolume24h) || 0)
      const orders = (n) => t('marketItem.orders', { n, count: formatAmount(n) })
      return [
        {
          id: 'sell',
          label: t('marketItem.sellFrom'),
          value: formatPrice(stats.lowestSell ?? sellBook.value[0]?.price),
          sub: sellCount ? `${orders(sellCount)} · ${formatAmount(stats.totalAvailableSell)} ${t('common.pcs')}` : t('marketItem.nobodySellsLower'),
          title: t('marketItem.sellFromTitle'),
        },
        {
          id: 'buy',
          label: t('marketItem.buyUpTo'),
          value: formatPrice(stats.highestBuy ?? buyBook.value[0]?.price),
          sub: buyCount ? `${orders(buyCount)} · ${formatAmount(stats.totalAvailableBuy)} ${t('common.pcs')}` : t('marketItem.nobodyBuysLower'),
          title: t('marketItem.buyUpToTitle'),
        },
        {
          id: 'avg',
          label: t('marketItem.avg7d'),
          value: formatPrice(avg7),
          sub: avg24 != null ? t('marketItem.last24h', { value: formatPrice(avg24) }) : t('marketItem.no24h'),
          title: t('marketItem.avgTitle'),
        },
        {
          id: 'volume',
          label: t('marketItem.sold7d'),
          value: formatAmount(volume7),
          unit: t('common.pcs'),
          sub: t('marketItem.last24h', { value: `${formatAmount(volume24)} ${t('common.pcs')}` }),
          title: t('marketItem.soldTitle'),
        },
      ]
    })

    const related = computed(() => {
      const info = detail.value?.packageInfo
      if (!info) return null
      const ratio = Number(info.ratio) || 0
      if (kind.value === 'item' && info.cargoId) {
        const lowest = bookRows(detail.value?.packageSellOrders, 'sell')[0]?.price
        return {
          label: t('marketItem.package', { n: formatAmount(ratio) }),
          price: lowest && ratio ? t('marketItem.packagePrice', { price: formatPrice(lowest), unit: formatPrice(lowest / ratio) }) : '',
          good: { id: String(info.cargoId), itemType: 1, name: info.cargoName, iconAssetName: info.cargoIconAssetName, tier: meta.value.tier },
        }
      }
      if (kind.value === 'cargo' && info.itemId) {
        return {
          label: t('marketItem.contains', { n: formatAmount(ratio) }),
          price: '',
          good: { id: String(info.itemId), itemType: 0, name: info.itemName, iconAssetName: info.itemIconAssetName, tier: meta.value.tier },
        }
      }
      return null
    })

    function placeText({ place, of }) {
      if (of === 1) return t('marketItem.onlyOrder')
      return place === 1 ? t('marketItem.bestOf', { of }) : t('marketItem.placeOf', { place, of })
    }
    function offerTitle(row, side) {
      const who = row.mine ? t('marketItem.yourOrder') : t(side === 'buy' ? 'marketItem.buyer' : 'marketItem.seller', { name: row.owner ?? '—' })
      const place = [row.claim.name, row.claim.regionName].filter(Boolean).join(', ')
      const coords = row.claim.x != null && row.claim.z != null ? ` · ${row.claim.x}, ${row.claim.z}` : ''
      return `${who} · ${place}${coords}`
    }
    function tradeTitle(trade) {
      const at = trade.at ? new Date(trade.at).toLocaleString(intlLocale()) : ''
      return [at, trade.seller && trade.buyer ? `${trade.seller} → ${trade.buyer}` : ''].filter(Boolean).join(' · ')
    }

    return {
      BOOK_ROWS, detail, loading, error, now, expanded, meta, bitjitaUrl, books, places, series, trades, kpis, related,
      load, placeText, offerTitle, tradeTitle, goodMeta, formatAmount, formatPrice, formatAgo,
    }
  },
}
</script>

<style scoped>
.mi {
  container: mi / inline-size;
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--frame-soft);
  border-radius: 4px;
  background: rgba(20, 18, 30, 0.3);
  overflow: hidden;
}
.mi-head {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px 6px 7px 8px;
  border-bottom: 1px solid var(--frame-soft);
  background: linear-gradient(180deg, rgba(31, 89, 135, 0.28), rgba(31, 89, 135, 0.08));
}
.mi-thumb { display: inline-flex; flex-shrink: 0; }
.mi-thumb :deep(.item-card--mini .item-card__icon),
.mi-thumb :deep(.item-card--mini.item-card--cargo .item-card__icon) { width: 34px; height: 40px; flex-basis: 34px; border-radius: 5px; }
.mi-title { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 1px; }
.mi-name { font-size: 14px; font-weight: 800; color: var(--text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mi-meta { font-size: 11px; color: var(--muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mi-spinner {
  flex-shrink: 0;
  width: 13px;
  height: 13px;
  border: 2px solid rgba(233, 223, 196, 0.2);
  border-top-color: var(--cream);
  border-radius: 50%;
  animation: mi-spin 0.8s linear infinite;
}
@keyframes mi-spin { to { transform: rotate(360deg); } }

.mi-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 10px;
}
.mi-state { display: flex; flex-direction: column; align-items: flex-start; gap: 8px; font-size: 12px; color: var(--muted); }

.mi-kpis { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 6px; }
.mi-kpi {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
  padding: 6px 9px 7px;
  border: 1px solid var(--frame-soft);
  border-radius: 4px;
  background: rgba(20, 18, 30, 0.4);
}
.mi-kpi--sell,
.mi-kpi--buy { border-top: 2px solid var(--book-color); }
.mi-kpi--sell,
.mi-book--sell { --book-color: var(--danger); }
.mi-kpi--buy,
.mi-book--buy { --book-color: var(--success); }
.mi-kpi--sell .mi-kpi-label,
.mi-kpi--buy .mi-kpi-label { color: var(--text-soft); }
.mi-kpi-label { font-size: 11px; color: var(--muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.mi-kpi-value { font-size: 19px; font-weight: 800; line-height: 1.15; color: var(--text); white-space: nowrap; }
.mi-kpi-value small { margin-left: 4px; font-size: 11px; font-weight: 700; color: var(--muted); }
.mi-kpi-sub { font-size: 10.5px; color: var(--muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

.mi-mine {
  margin: 0;
  padding: 6px 9px;
  list-style: none;
  border-left: 2px solid var(--gold);
  border-radius: 0 4px 4px 0;
  background: rgba(217, 180, 92, 0.08);
  font-size: 12px;
  color: var(--text-soft);
}
.mi-mine b { color: var(--text); }
.mi-mine-place { color: var(--muted-strong); }

.mi-block { display: flex; flex-direction: column; gap: 6px; min-width: 0; }
.mi-section {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin: 0;
  font-family: var(--font-title);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--text-soft);
}
.mi-section-note { font-family: var(--font-body); font-size: 11px; letter-spacing: 0; text-transform: none; color: var(--muted); }
.mi-section--book::before {
  content: '';
  align-self: center;
  width: 7px;
  height: 7px;
  margin-right: -2px;
  border-radius: 50%;
  background: var(--book-color);
}
.mi-note { margin: 0; font-size: 12px; color: var(--muted); }

.mi-books { display: grid; grid-template-columns: minmax(0, 1fr); gap: 12px; }
.mi-book { min-width: 0; display: flex; flex-direction: column; gap: 4px; }
.mi-offers { display: flex; flex-direction: column; }
.mi-offer {
  display: grid;
  grid-template-columns: minmax(48px, auto) minmax(52px, auto) minmax(0, 1fr) auto;
  align-items: baseline;
  gap: 8px;
  padding: 4px 6px;
  border-bottom: 1px solid rgba(233, 223, 196, 0.07);
  font-size: 12px;
}
.mi-offer--mine { background: rgba(217, 180, 92, 0.1); border-radius: 3px; }
.mi-offer--head,
.mi-trade--head {
  padding-top: 0;
  padding-bottom: 3px;
  border-bottom-color: var(--border);
  font-size: 10px;
  color: var(--muted);
}
.mi-offer--head span:nth-child(-n+2),
.mi-trade--head span:nth-child(-n+2) { text-align: right; }
.mi-offer-price { font-weight: 800; color: var(--text); text-align: right; font-variant-numeric: tabular-nums; }
.mi-offer--best .mi-offer-price { color: var(--book-color); }
.mi-offer-qty { color: var(--muted-strong); text-align: right; font-variant-numeric: tabular-nums; }
.mi-offer-place { min-width: 0; color: var(--text-soft); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mi-offer-place small { font-size: 11px; color: var(--muted); }
.mi-you {
  padding: 0 5px;
  border-radius: 7px;
  background: var(--gold);
  color: #1b1726;
  font-size: 10px;
  font-weight: 800;
  line-height: 15px;
}
.mi-more {
  align-self: flex-start;
  margin-top: 3px;
  padding: 2px 4px;
  border: none;
  background: none;
  color: var(--accent);
  font-size: 11px;
  font-weight: 700;
  cursor: pointer;
}
.mi-more:hover { color: var(--text); text-decoration: underline; }

.mi-related {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 5px 8px;
  border: 1px solid var(--frame-soft);
  border-radius: 4px;
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.mi-related:hover { border-color: var(--frame); background: rgba(233, 223, 196, 0.05); }
.mi-related-thumb { display: inline-flex; flex-shrink: 0; }
.mi-related-thumb :deep(.item-card--mini .item-card__icon),
.mi-related-thumb :deep(.item-card--mini.item-card--cargo .item-card__icon) { width: 26px; height: 30px; flex-basis: 26px; border-width: 1px; border-radius: 4px; }
.mi-related-text { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.mi-related-label { font-size: 10px; color: var(--muted); }
.mi-related-name { font-size: 12px; font-weight: 700; color: var(--text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mi-related-price { flex-shrink: 0; font-size: 11px; font-weight: 700; color: var(--text-soft); font-variant-numeric: tabular-nums; }

.mi-trades { display: flex; flex-direction: column; }
.mi-trade {
  display: grid;
  grid-template-columns: minmax(48px, auto) minmax(52px, auto) minmax(0, 1fr) auto;
  align-items: baseline;
  gap: 8px;
  padding: 3px 6px;
  border-bottom: 1px solid rgba(233, 223, 196, 0.07);
  font-size: 12px;
}
.mi-trade-price { font-weight: 800; color: var(--text); text-align: right; font-variant-numeric: tabular-nums; }
.mi-trade-qty { color: var(--muted-strong); text-align: right; font-variant-numeric: tabular-nums; }
.mi-trade-place { min-width: 0; color: var(--muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mi-trade-time { font-size: 11px; color: var(--muted); white-space: nowrap; }

@container mi (max-width: 420px) {
  .mi-kpi { padding: 5px 8px 6px; }
  .mi-kpi-value { font-size: 17px; }
}
@container mi (min-width: 560px) {
  .mi-kpis { grid-template-columns: repeat(4, minmax(0, 1fr)); }
  .mi-books { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
}
</style>
