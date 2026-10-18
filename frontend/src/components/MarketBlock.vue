<template>
  <div class="market">
    <MarketSearch ref="searchRef" :recent="recentGoods" @select="onSearchSelect" @clear-recent="clearRecent">
      <template #aside>
        <span v-if="market?.marketBalance != null" class="market-balance" :title="t('marketBlock.balanceTitle')">
          <AppIcon name="coin" :size="13" />{{ formatAmount(market.marketBalance) }}
        </span>
      </template>
    </MarketSearch>

    <div class="bc-seg market-tabs" role="tablist" :aria-label="t('marketBlock.sections')">
      <button
        v-for="option in viewOptions"
        :key="option.id"
        type="button"
        role="tab"
        class="bc-seg-btn market-tab"
        :class="{ active: !lookup && view === option.id }"
        :aria-selected="!lookup && view === option.id ? 'true' : 'false'"
        :title="option.title"
        @click="setView(option.id)"
      >{{ option.label }}<span v-if="option.count" class="market-count" :class="{ 'market-count--gold': option.gold }">{{ option.count }}</span></button>
    </div>

    <MarketItemPanel
      v-if="lookup"
      :good="lookup"
      :player-entity-id="block.playerEntityId ?? null"
      :refresh-key="market?.updatedAt ?? null"
      @close="lookup = null"
      @open="openGood"
    />

    <template v-else-if="view !== 'trades'">
      <div v-if="!ordersLoaded" class="market-state">
        <template v-if="market?.error">
          <span class="bc-error">{{ t('marketBlock.ordersFailed', { error: market.error }) }}</span>
          <button type="button" class="bc-btn" @click="$emit('load')">{{ t('common.retry') }}</button>
        </template>
        <span v-else class="bc-muted-note">{{ t('marketBlock.loadingOrders') }}</span>
      </div>

      <template v-else>
        <p v-if="market?.error" class="market-warn" :title="market.error">{{ t('marketBlock.staleData') }}</p>

        <div v-if="!rows.length" class="market-empty">
          <AppIcon name="market" :size="26" />
          <p>{{ t('marketBlock.noSellOrders') }}</p>
          <button type="button" class="bc-btn" @click="searchRef?.focus()">
            <AppIcon name="search" :size="13" />{{ t('marketBlock.checkPrice') }}
          </button>
        </div>

        <div v-else class="market-scroll">
          <div class="mrow mrow--head" aria-hidden="true">
            <span></span>
            <span>{{ t('common.item') }}</span>
            <span class="mrow-col mrow-col--mine" :title="t('marketBlock.yourPriceTitle')">{{ t('marketBlock.yourPrice') }}</span>
            <span class="mrow-col mrow-col--wide" :title="t('marketBlock.lowestTitle')">{{ t('marketBlock.lowest') }}</span>
            <span class="mrow-col mrow-col--wide" :title="t('marketBlock.averageTitle')">{{ t('marketBlock.average') }}</span>
            <span class="mrow-col mrow-col--wide" :title="t('marketBlock.lotTitle')">{{ t('marketBlock.lot') }}</span>
          </div>
          <section v-for="group in groups" :key="group.key" class="market-group">
            <h3 class="market-group-title">
              <span class="market-group-place">{{ claimLabel(group.claim) }}</span>
              <span v-if="claimCoords(group.claim)" class="market-group-coords">{{ claimCoords(group.claim) }}</span>
              <span class="market-group-sum" :title="t('marketBlock.groupSumTitle')">{{ formatAmount(group.total) }}</span>
            </h3>
            <button
              v-for="row in group.rows"
              :key="row.key"
              type="button"
              class="mrow"
              :title="t('marketBlock.openItem', { name: row.good.name })"
              @click="openGood(row.good)"
            >
              <span class="mrow-thumb">
                <ItemCard
                  variant="mini"
                  badge
                  :name="row.good.name"
                  :icon-asset-name="row.good.iconAssetName"
                  :item-type="row.good.itemType"
                  :tier="row.good.tier"
                  :rarity-str="row.good.rarityStr"
                />
                <span class="mrow-qty">{{ formatAmount(row.quantity) }}</span>
              </span>
              <span class="mrow-main">
                <span class="mrow-name">{{ row.good.name }}</span>
                <span class="mrow-sub mrow-sub--wide">{{ goodMeta(row.good) }}</span>
                <span class="mrow-sub mrow-sub--narrow">{{ narrowSub(row) }}</span>
              </span>
              <span class="mrow-col mrow-col--mine mrow-cell">
                <span class="mrow-value">{{ formatPrice(row.unitPrice) }}</span>
                <span class="mrow-tag mrow-tag--narrow" :class="lowestTone(row)" :title="lowestHint(row)">{{ lowestTag(row, true) }}</span>
                <span class="mrow-tag mrow-tag--muted mrow-tag--wide">{{ t('marketBlock.perUnit') }}</span>
              </span>
              <span class="mrow-col mrow-col--wide mrow-cell" :title="lowestHint(row)">
                <span class="mrow-value mrow-value--market">{{ formatPrice(row.market.lowest) }}</span>
                <span class="mrow-tag" :class="lowestTone(row)">{{ lowestTag(row) }}</span>
              </span>
              <span class="mrow-col mrow-col--wide mrow-cell" :title="averageHint(row)">
                <span class="mrow-value mrow-value--market">{{ formatPrice(row.market.average) }}<small v-if="row.market.averageWindow === '24h'">{{ t('marketBlock.window24h') }}</small></span>
                <span class="mrow-tag" :class="averageTone(row)">{{ averageTag(row) }}</span>
              </span>
              <span class="mrow-col mrow-col--wide mrow-cell">
                <span class="mrow-value">{{ formatAmount(row.total) }}</span>
                <span class="mrow-tag mrow-tag--muted">{{ formatAmount(row.quantity) }} × {{ formatPrice(row.unitPrice) }}</span>
              </span>
            </button>
          </section>
        </div>

        <footer v-if="rows.length" class="market-foot">
          <span>{{ t('marketBlock.lots', { n: totals.count }) }} · {{ t('marketBlock.markets', { n: totals.claims }) }}</span>
          <span class="market-foot-total" :title="totalsTitle">
            {{ t('marketBlock.onSale') }}
            <b><AppIcon name="coin" :size="12" />{{ formatAmount(totals.total) }}</b>
            <span v-if="totals.priced" class="market-foot-avg">{{ t('marketBlock.atAverage', { amount: formatAmount(totals.atAverage) }) }}</span>
          </span>
        </footer>
      </template>
    </template>

    <template v-else>
      <div v-if="!market?.marketCollections && !market?.marketTrades" class="market-state">
        <span class="bc-muted-note">{{ t('marketBlock.loadingTrades') }}</span>
      </div>
      <template v-else>
        <div class="collect" :title="collect.coins ? t('marketBlock.collectTitle') : ''">
          <div class="collect-total">
            <span class="collect-label">{{ t('marketBlock.toCollect') }}</span>
            <span class="collect-value"><AppIcon name="coin" :size="16" />{{ formatAmount(collect.coins) }}</span>
          </div>
          <div v-if="collect.piles" class="collect-meta">
            <span>{{ t('marketBlock.payouts', { n: collect.coinPiles }) }}</span>
            <span>{{ t('marketBlock.markets', { n: collect.claims }) }}</span>
            <span v-if="collect.itemPiles">{{ t('marketBlock.withGoods', { n: collect.itemPiles }) }}</span>
          </div>
        </div>

        <div class="market-scroll">
          <section v-for="group in collectGroupList" :key="`c${group.key}`" class="market-group">
            <h3 class="market-group-title">
              <span class="market-group-place">{{ claimLabel(group.claim) }}</span>
              <span v-if="claimCoords(group.claim)" class="market-group-coords">{{ claimCoords(group.claim) }}</span>
              <span class="market-group-sum">{{ formatAmount(group.coins) }}</span>
            </h3>
            <component
              :is="pileGood(row) ? 'button' : 'div'"
              v-for="row in group.rows"
              :key="row.key"
              :type="pileGood(row) ? 'button' : undefined"
              class="mrow mrow--trade"
              @click="pileGood(row) && openGood(pileGood(row))"
            >
              <span class="mrow-thumb">
                <ItemCard
                  variant="mini"
                  :name="pileName(row)"
                  :icon-asset-name="row.item.iconAssetName"
                  :item-type="row.item.itemType"
                  :tier="row.item.tier"
                  :rarity-str="row.item.rarityStr"
                />
              </span>
              <span class="mrow-main">
                <span class="mrow-name">{{ pileName(row) }}</span>
                <span class="mrow-sub">{{ pileDetail(row) }}</span>
              </span>
              <span class="mrow-amount" :class="row.isCoins ? 'mrow-amount--coins' : ''">
                {{ row.isCoins ? formatAmount(row.amount) : `×${formatAmount(row.amount)}` }}
              </span>
              <span class="mrow-time">{{ formatMoment(row.at) }}</span>
            </component>
          </section>
          <p v-if="!collectGroupList.length" class="market-note">{{ t('marketBlock.nothingToCollect') }}</p>

          <h3 class="market-group-title">
            <span class="market-group-place">{{ t('marketBlock.completedTrades') }}</span>
            <span v-if="tradeList.length" class="market-group-coords">{{ t('marketBlock.ofTotal', { n: tradeList.length, total: tradesTotal }) }}</span>
            <button
              type="button"
              role="switch"
              class="trade-filter"
              :aria-checked="showBuys ? 'true' : 'false'"
              :title="t('marketBlock.showBuysTitle')"
              @click="toggleBuys"
            >{{ t('marketBlock.showBuys') }}<span class="bc-toggle" :class="{ on: showBuys }"></span></button>
          </h3>
          <p v-if="!shownTrades.length" class="market-note">{{ !market?.marketTrades ? t('marketBlock.loadingTrades') : tradeList.length ? t('marketBlock.noSalesYet') : t('marketBlock.noTradesYet') }}</p>
          <component
            :is="tradeGood(row) ? 'button' : 'div'"
            v-for="row in shownTrades"
            :key="row.key"
            :type="tradeGood(row) ? 'button' : undefined"
            class="mrow mrow--trade"
            @click="tradeGood(row) && openGood(tradeGood(row))"
          >
            <span class="mrow-thumb">
              <ItemCard
                variant="mini"
                :name="row.item.name"
                :icon-asset-name="row.item.iconAssetName"
                :item-type="row.item.itemType"
                :tier="row.item.tier"
                :rarity-str="row.item.rarityStr"
              />
            </span>
            <span class="mrow-main">
              <span class="mrow-name">{{ row.item.name }}</span>
              <span class="mrow-sub">{{ tradeDetail(row) }}</span>
            </span>
            <span class="mrow-amount" :class="row.side === 'sell' ? 'mrow-amount--coins' : 'mrow-amount--spent'">
              {{ row.side === 'sell' ? '+' : '−' }}{{ formatAmount(row.totalPrice) }}
            </span>
            <span class="mrow-time">{{ formatMoment(row.at) }}</span>
          </component>
        </div>

        <footer class="market-foot" :title="t('marketBlock.weekTitle')">
          <span>{{ t('marketBlock.week') }}</span>
          <span class="market-foot-total">
            {{ t('marketBlock.sold') }} <b><AppIcon name="coin" :size="12" />{{ formatAmount(week.sold) }}</b>
            <span v-if="showBuys && week.bought" class="market-foot-avg">{{ t('marketBlock.bought', { amount: formatAmount(week.bought) }) }}</span>
          </span>
        </footer>
      </template>
    </template>
  </div>
</template>

<script>
import { computed, ref } from 'vue'
import { useItemCache } from '../composables/useItemCache'
import { goodMeta } from '../marketSearch'
import { collectGroups, collectRows, collectSummary, tradeRows, tradeTotals } from '../marketTrades'
import {
  DEVIATION_PCT,
  formatAmount,
  formatPct,
  formatPrice,
  groupByClaim,
  orderTotals,
  playerOrderRows,
  priceStanding,
} from '../marketOrders'
import { formatClock, formatDay } from '../xpHistory'
import { t } from '../i18n'
import AppIcon from './AppIcon.vue'
import ItemCard from './ItemCard.vue'
import MarketItemPanel from './MarketItemPanel.vue'
import MarketSearch from './MarketSearch.vue'

const RECENT_LIMIT = 8
const COIN_ITEM_ID = '1'

export default {
  name: 'MarketBlock',
  components: { AppIcon, ItemCard, MarketItemPanel, MarketSearch },
  props: {
    block: { type: Object, required: true },
    market: { type: Object, default: null },
  },
  emits: ['update', 'load'],
  setup(props, { emit }) {
    const { resolveItem } = useItemCache()
    const searchRef = ref(null)
    const lookup = ref(null)

    const view = computed(() => (props.block.marketTab === 'trades' ? 'trades' : 'sell'))
    const ordersLoaded = computed(() => Array.isArray(props.market?.marketOrders))
    const rows = computed(() => playerOrderRows(props.market?.marketOrders, 'sell', props.market?.marketBulkMaps, resolveItem))
    const groups = computed(() => groupByClaim(rows.value))
    const totals = computed(() => orderTotals(rows.value))
    const totalsTitle = computed(() => {
      const what = t('marketBlock.totalsTitle')
      if (!totals.value.priced) return what
      return t('marketBlock.totalsTitleAvg', { what, amount: formatAmount(totals.value.atAverage), priced: totals.value.priced, n: totals.value.count })
    })

    const collectRowList = computed(() => collectRows(props.market?.marketCollections, props.market?.marketTrades, props.block.playerEntityId))
    const collectGroupList = computed(() => collectGroups(collectRowList.value))
    const collect = computed(() => collectSummary(collectRowList.value))
    const tradeList = computed(() => tradeRows(props.market?.marketTrades, props.block.playerEntityId))
    const tradesTotal = computed(() => props.market?.marketTradesTotal ?? tradeList.value.length)
    const showBuys = computed(() => props.block.marketShowBuys !== false)
    const shownTrades = computed(() => (showBuys.value ? tradeList.value : tradeList.value.filter((row) => row.side !== 'buy')))
    const week = computed(() => tradeTotals(tradeList.value, Date.now() - 7 * 86400000))
    const viewOptions = computed(() => [
      { id: 'sell', label: t('marketBlock.sellTab'), count: rows.value.length, title: t('marketBlock.sellTabTitle') },
      {
        id: 'trades',
        label: t('marketBlock.tradesTab'),
        count: collect.value.piles,
        gold: true,
        title: t('marketBlock.tradesTabTitle'),
      },
    ])

    const recentGoods = computed(() => (Array.isArray(props.block.marketRecent) ? props.block.marketRecent : []))

    function setView(next) {
      lookup.value = null
      if (next === view.value) return
      if (next !== 'trades') {
        emit('update', { marketTab: 'orders' })
        return
      }
      emit('update', { marketTab: 'trades' })
      if (!props.market?.marketTrades) emit('load')
    }
    function goodOf(good) {
      return {
        id: String(good.id),
        itemType: Number(good.itemType) === 1 ? 1 : 0,
        name: good.name ?? null,
        iconAssetName: good.iconAssetName ?? null,
        tier: good.tier ?? null,
        rarityStr: good.rarityStr ?? null,
        tag: good.tag ?? null,
      }
    }
    function openGood(good) {
      if (!good?.id) return
      lookup.value = goodOf(good)
    }
    function onSearchSelect(good) {
      const picked = goodOf(good)
      lookup.value = picked
      const sameGood = (other) => String(other.id) === picked.id && Number(other.itemType) === picked.itemType
      emit('update', { marketRecent: [picked, ...recentGoods.value.filter((other) => !sameGood(other))].slice(0, RECENT_LIMIT) })
    }
    function toggleBuys() {
      emit('update', { marketShowBuys: !showBuys.value })
    }
    function clearRecent() {
      emit('update', { marketRecent: [] })
    }

    function claimLabel(claim) {
      return [claim?.name, claim?.regionName].filter(Boolean).join(' · ') || t('marketBlock.noPlace')
    }
    function claimCoords(claim) {
      return claim?.x != null && claim?.z != null ? `${claim.x}, ${claim.z}` : ''
    }
    function formatMoment(ms) {
      if (!ms) return '—'
      return `${formatDay(ms / 1000)} ${formatClock(ms / 1000)}`
    }
    function pileName(row) {
      if (row.isCoins && !row.trade) return t('marketBlock.coins')
      return row.item.name ?? '?'
    }
    function pileDetail(row) {
      if (!row.trade) return row.isCoins ? t('marketBlock.outsideHistory') : t('marketBlock.orderClosed')
      const trade = row.trade
      const parts = [t('marketBlock.qtyAt', { qty: formatAmount(trade.quantity), price: formatPrice(trade.unitPrice) })]
      if (trade.counterparty) parts.push(t(trade.side === 'sell' ? 'marketBlock.boughtBy' : 'marketBlock.soldBy', { name: trade.counterparty }))
      return parts.join(' · ')
    }
    function pileGood(row) {
      const item = row.trade?.item ?? row.item
      return item?.id && item.id !== COIN_ITEM_ID ? { ...item } : null
    }
    function tradeDetail(row) {
      const parts = [t('marketBlock.qtyAt', { qty: formatAmount(row.quantity), price: formatPrice(row.unitPrice) })]
      if (row.counterparty) parts.push(t(row.side === 'sell' ? 'marketBlock.boughtBy' : 'marketBlock.soldBy', { name: row.counterparty }))
      if (row.claim?.name) parts.push(row.claim.name)
      return parts.join(' · ')
    }
    function lowestTone(row) {
      if (row.market.lowest == null) return 'mrow-tag--muted'
      return row.market.isLowest ? 'mrow-tag--good' : 'mrow-tag--warn'
    }
    function lowestTag(row, short = false) {
      const { lowest, isLowest, lowestPct } = row.market
      if (lowest == null) return short ? '' : t('marketBlock.noSellers')
      if (isLowest) return t('marketBlock.lowestIsYou')
      return short ? t('marketBlock.lowestShort', { price: formatPrice(lowest) }) : t('marketBlock.youAreHigher', { pct: formatPct(lowestPct).replace('+', '') })
    }
    function lowestHint(row) {
      const { lowest, isLowest } = row.market
      if (lowest == null) return t('marketBlock.lowestTitle')
      if (isLowest) return t('marketBlock.lowestIsYouTitle')
      return t('marketBlock.youAreHigherTitle', { lowest: formatPrice(lowest), price: formatPrice(row.unitPrice) })
    }
    function averageTone(row) {
      const pct = row.market.averagePct
      if (pct == null || Math.abs(pct) <= DEVIATION_PCT) return 'mrow-tag--muted'
      return pct > 0 ? 'mrow-tag--bad' : 'mrow-tag--good'
    }
    function averageTag(row) {
      const pct = row.market.averagePct
      if (pct == null) return t('marketBlock.noTrades')
      if (Math.abs(pct) < 0.05) return t('marketBlock.atMarket')
      return `${pct > 0 ? '▲' : '▼'} ${formatPct(Math.abs(pct)).replace('+', '')}`
    }
    function averageHint(row) {
      const { average, averagePct, averageWindow } = row.market
      if (average == null) return t('marketBlock.noTradesTitle')
      const basis = t(averageWindow === '24h' ? 'marketRef.avg24h' : 'marketRef.avg7d')
      const standing = priceStanding({ side: 'sell', diffPct: averagePct })
      const pct = formatPct(Math.abs(averagePct)).replace('+', '')
      const text = standing === 'fair' ? t('standing.fair', { pct: DEVIATION_PCT }) : t(averagePct > 0 ? 'standing.sellAbove' : 'standing.sellBelow', { pct })
      return `${text}. ${basis.charAt(0).toUpperCase()}${basis.slice(1)}: ${formatPrice(average)}`
    }
    function narrowSub(row) {
      const parts = [t('marketBlock.lotSum', { amount: formatAmount(row.total) })]
      if (row.market.average != null) parts.push(t('marketBlock.avgShort', { price: formatPrice(row.market.average) }))
      return parts.join(' · ')
    }
    function tradeGood(row) {
      return row.item?.id ? { ...row.item } : null
    }

    return {
      searchRef, lookup, view, viewOptions, ordersLoaded, rows, groups, totals, totalsTitle,
      collect, collectGroupList, tradeList, tradesTotal, showBuys, shownTrades, week, recentGoods,
      setView, openGood, onSearchSelect, clearRecent, toggleBuys,
      claimLabel, claimCoords, formatMoment, pileName, pileDetail, pileGood, tradeDetail, tradeGood,
      lowestTone, lowestTag, lowestHint, averageTone, averageTag, averageHint, narrowSub,
      goodMeta, formatAmount, formatPrice
    }
  },
}
</script>

<style scoped>
.market {
  container: market / inline-size;
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.market-tabs { flex-shrink: 0; display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); }
.market-tab { display: inline-flex; align-items: center; justify-content: center; gap: 6px; min-width: 0; padding: 4px 6px; }
.market-count {
  padding: 0 5px;
  border-radius: 7px;
  background: rgba(233, 223, 196, 0.14);
  color: var(--text);
  font-family: var(--font-body);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0;
  line-height: 14px;
  font-variant-numeric: tabular-nums;
}
.bc-seg-btn.active .market-count { background: rgba(27, 23, 38, 0.16); color: var(--on-cream); }
.market-count--gold { background: var(--gold); color: #1b1726; }
.bc-seg-btn.active .market-count--gold { background: #1b1726; color: var(--gold); }
.market-balance {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 14px;
  font-weight: 800;
  color: var(--gold);
  white-space: nowrap;
}

.market-state { display: flex; flex-direction: column; align-items: flex-start; gap: 8px; padding: 4px 2px; }
.market-warn { margin: 0; font-size: 11px; color: var(--warning); }
.market-empty {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--muted);
  text-align: center;
}
.market-empty p { margin: 0; font-size: 13px; color: var(--text-soft); }
.market-note { margin: 4px 0 6px; padding: 0 4px; font-size: 12px; color: var(--muted); font-style: italic; }

.market-scroll { flex: 1; min-height: 0; overflow-y: auto; overflow-x: hidden; margin-right: -6px; padding-right: 6px; }
.market-group-title {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin: 10px 0 2px;
  padding: 0 4px;
  font-family: var(--font-title);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--gold);
}
.market-group:first-of-type .market-group-title { margin-top: 4px; }
.market-group-place { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.market-group-coords { flex: 1; font-family: var(--font-body); font-size: 10px; letter-spacing: 0; text-transform: none; color: var(--muted); white-space: nowrap; }
.market-group-sum { margin-left: auto; font-family: var(--font-body); font-size: 12px; font-weight: 800; letter-spacing: 0; color: var(--gold); font-variant-numeric: tabular-nums; }

.mrow {
  display: grid;
  grid-template-columns: 32px minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
  width: 100%;
  min-height: 46px;
  padding: 4px 6px;
  border: none;
  border-bottom: 1px solid rgba(233, 223, 196, 0.07);
  border-radius: 3px;
  background: transparent;
  color: var(--text);
  font: inherit;
  font-size: 12px;
  text-align: left;
}
button.mrow { cursor: pointer; transition: background 0.12s; }
button.mrow:hover { background: rgba(233, 223, 196, 0.06); }
button.mrow:focus-visible { outline: 2px solid var(--cream); outline-offset: -2px; }
.mrow .mrow-col--wide,
.mrow-tag--wide,
.mrow-sub--wide { display: none; }
.mrow--head {
  position: sticky;
  top: 0;
  z-index: 1;
  min-height: 0;
  padding-top: 2px;
  padding-bottom: 5px;
  border-bottom-color: var(--border);
  border-radius: 0;
  background: var(--panel);
  font-family: var(--font-title);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--muted);
}
.mrow--head > span { font: inherit; letter-spacing: inherit; color: inherit; }
.mrow--head .mrow-col { text-align: right; cursor: help; }
.mrow--head .mrow-col--mine { color: var(--gold); }
.mrow-thumb { position: relative; display: inline-flex; }
.mrow-thumb :deep(.item-card--mini .item-card__icon),
.mrow-thumb :deep(.item-card--mini.item-card--cargo .item-card__icon) { width: 30px; height: 36px; flex-basis: 30px; border-radius: 5px; }
.mrow-thumb :deep(.item-card__badge) { display: none; }
.mrow-qty {
  position: absolute;
  right: 2px;
  bottom: 1px;
  color: #fff;
  font-size: 10px;
  font-weight: 800;
  line-height: 1;
  text-shadow: -1px -1px 0 #000, 1px -1px 0 #000, -1px 1px 0 #000, 1px 1px 0 #000;
  pointer-events: none;
}
.mrow-main { min-width: 0; display: flex; flex-direction: column; gap: 1px; }
.mrow-name { font-size: 13px; font-weight: 700; color: var(--text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mrow-sub { font-size: 11px; color: var(--muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-variant-numeric: tabular-nums; }
.mrow-cell { min-width: 0; display: flex; flex-direction: column; align-items: flex-end; gap: 2px; text-align: right; white-space: nowrap; font-variant-numeric: tabular-nums; }
.mrow-value { font-size: 14px; font-weight: 800; line-height: 1.1; color: var(--text); }
.mrow-value small { margin-left: 2px; font-size: 9.5px; font-weight: 700; color: var(--muted); }
.mrow-value--market { font-size: 13px; font-weight: 700; color: var(--muted-strong); }
.mrow-tag { max-width: 100%; overflow: hidden; text-overflow: ellipsis; font-size: 10.5px; font-weight: 700; line-height: 1.2; }
.mrow-tag:empty { display: none; }
.mrow-tag--muted { color: var(--muted); font-weight: 600; }
.mrow-tag--good { color: var(--success); }
.mrow-tag--warn { color: var(--warning); }
.mrow-tag--bad { color: var(--danger); }

.trade-filter {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 0;
  border: none;
  background: transparent;
  color: var(--muted-strong);
  font: inherit;
  cursor: pointer;
}
.trade-filter:hover { color: var(--text); }
.trade-filter:focus-visible { outline: 2px solid var(--cream); outline-offset: 2px; }

.mrow--trade { grid-template-columns: 32px minmax(0, 1fr) auto auto; }
.mrow-amount { font-size: 13px; font-weight: 800; text-align: right; white-space: nowrap; font-variant-numeric: tabular-nums; color: var(--text-soft); }
.mrow-amount--coins { color: var(--gold); }
.mrow-amount--spent { color: var(--muted); }
.mrow-time { min-width: 62px; font-size: 10px; color: var(--muted); text-align: right; white-space: nowrap; font-variant-numeric: tabular-nums; }

.market-foot {
  flex-shrink: 0;
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  align-items: baseline;
  gap: 2px 12px;
  padding: 6px 2px 0;
  border-top: 1px solid var(--frame-soft);
  font-size: 11px;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}
.market-foot-total { display: inline-flex; align-items: baseline; flex-wrap: wrap; justify-content: flex-end; gap: 0 6px; }
.market-foot-total b { display: inline-flex; align-items: center; gap: 4px; align-self: center; font-size: 14px; font-weight: 800; color: var(--gold); }
.market-foot-avg { color: var(--muted); }

.collect {
  flex-shrink: 0;
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  gap: 10px;
  padding: 7px 10px;
  border: 1px solid var(--frame-soft);
  border-radius: 4px;
  background: rgba(20, 18, 30, 0.35);
}
.collect-total { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.collect-label { font-size: 11px; color: var(--muted); }
.collect-value { display: inline-flex; align-items: center; gap: 6px; font-size: 22px; font-weight: 800; line-height: 1; color: var(--gold); }
.collect-meta { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 2px 10px; font-size: 11px; color: var(--muted); font-variant-numeric: tabular-nums; }

@container market (min-width: 600px) {
  .mrow { grid-template-columns: 34px minmax(0, 1fr) 84px 112px 96px 104px; min-height: 46px; gap: 12px; }
  .mrow-thumb :deep(.item-card--mini .item-card__icon),
  .mrow-thumb :deep(.item-card--mini.item-card--cargo .item-card__icon) { width: 32px; height: 38px; flex-basis: 32px; }
  .mrow-thumb :deep(.item-card__badge) { display: block; }
  .mrow .mrow-col--wide { display: flex; }
  .mrow--head .mrow-col--wide { display: block; }
  .mrow-sub--wide,
  .mrow-tag--wide { display: block; }
  .mrow-sub--narrow,
  .mrow-tag--narrow { display: none; }
  .mrow--trade { grid-template-columns: 34px minmax(0, 1fr) 120px 96px; }
}
</style>
