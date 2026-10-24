<template>
  <div ref="rootRef" class="pinned-dashboard">
    <Teleport defer to="#app-actions">
      <button
        type="button"
        class="bc-icon-btn dash-refresh"
        :class="{ 'dash-refresh--busy': dashboardRefreshing }"
        :title="dashboardUpdatedLabel"
        :disabled="pinned.length === 0"
        @click="refreshAll"
      >
        <AppIcon name="refresh" />
      </button>
      <button type="button" class="bc-btn bc-btn--primary" @click="openAddWizard">
        <AppIcon name="plus" :size="14" />{{ t('dashboard.addBlock') }}
      </button>
    </Teleport>

    <section v-if="pinned.length === 0" class="bc-panel dash-empty">
      <header class="bc-panel-head">
        <h2 class="bc-title">{{ t('dashboard.emptyTitle') }}</h2>
        <span class="bc-ornament"></span>
      </header>
      <div class="bc-panel-body dash-empty-body">
        <p>{{ t('dashboard.emptyText') }}</p>
        <button type="button" class="bc-btn bc-btn--primary" @click="openAddWizard">
          <AppIcon name="plus" :size="14" />{{ t('dashboard.addBlock') }}
        </button>
      </div>
    </section>

    <div
      v-else
      ref="gridRef"
      class="dashboard-grid"
      :class="{ 'dashboard-grid--dragging': Boolean(drag.id) }"
      :style="{ '--dashboard-cols': layoutInfo.cols }"
    >
      <template v-if="drag.id">
        <div
          v-for="cell in dropCells"
          :key="`cell-${cell.x}-${cell.y}`"
          class="dashboard-cell"
          :style="gridArea(cell.x, cell.y, 1)"
        ></div>
        <div
          v-if="drag.target && !drag.target.same"
          class="drop-preview"
          :class="drag.target.valid ? 'drop-preview--ok' : 'drop-preview--blocked'"
          :style="gridArea(drag.target.x, drag.target.y, drag.target.span)"
        ></div>
        <div
          v-if="drag.target?.valid && drag.target.swap"
          class="drop-preview drop-preview--swap"
          :style="gridArea(drag.target.swap.x, drag.target.swap.y, drag.target.swap.span)"
        ></div>
      </template>

      <section
        v-for="item in pinned"
        :key="item.inventoryId"
        class="bc-panel dash-card"
        :class="dashboardCardClass(item.inventoryId)"
        :style="dashboardCardStyle(item.inventoryId)"
        :data-tile-id="item.inventoryId"
      >
        <header
          class="bc-panel-head dash-card-head"
          :title="t('dashboard.dragTitle')"
          @pointerdown="onTilePointerDown($event, item.inventoryId)"
        >
          <span v-if="blockHeading(item).kicker" class="bc-kicker">{{ blockHeading(item).kicker }}</span>
          <h2 class="bc-title" :title="blockHeading(item).title">{{ blockHeading(item).title }}</h2>
          <span class="bc-ornament"></span>
          <div v-if="isMarketBlock(item)" class="dash-card-flags">
            <span
              v-if="marketDeviationCount(item.inventoryId, 'above') > 0"
              class="dash-flag dash-flag--above"
              :title="t('dashboard.flagAbove', { pct: FLAG_PCT })"
            >▲ {{ marketDeviationCount(item.inventoryId, 'above') }}</span>
            <span
              v-if="marketDeviationCount(item.inventoryId, 'below') > 0"
              class="dash-flag dash-flag--below"
              :title="t('dashboard.flagBelow', { pct: FLAG_PCT })"
            >▼ {{ marketDeviationCount(item.inventoryId, 'below') }}</span>
            <span
              v-if="marketCollectStats(item.inventoryId).coins > 0"
              class="dash-flag dash-flag--collect"
              :title="t('dashboard.flagCollect', { coins: formatBig(marketCollectStats(item.inventoryId).coins) })"
            ><AppIcon name="coin" :size="11" />{{ formatXpShort(marketCollectStats(item.inventoryId).coins) }}</span>
          </div>
          <div class="dash-card-tools">
            <button
              v-if="isPlannerBlock(item)"
              type="button"
              class="bc-icon-btn"
              :title="t('dashboard.edit')"
              @click.stop="openEditPlannerWizard(item.inventoryId)"
            ><AppIcon name="edit" :size="14" /></button>
            <button
              v-if="isSkillBlock(item)"
              type="button"
              class="bc-icon-btn"
              :title="t('dashboard.changeSkill')"
              @click.stop="openEditSkillWizard(item.inventoryId)"
            ><AppIcon name="swap" :size="14" /></button>
            <button
              v-if="blockCanResize(item)"
              type="button"
              class="bc-icon-btn"
              :title="blockIsLarge(item) ? t('common.collapse') : t('dashboard.expand')"
              @click.stop="toggleBlockSize(item)"
            ><AppIcon :name="blockIsLarge(item) ? 'collapse' : 'expand'" :size="14" /></button>
            <button type="button" class="bc-close" :title="t('dashboard.unpin')" @click.stop="unpin(item.inventoryId)">
              <AppIcon name="close" :size="12" />
            </button>
          </div>
        </header>

        <div class="bc-panel-body dash-card-body" :class="`dash-card-body--${blockType(item)}`">
          <div v-if="state[item.inventoryId]?.loading && blockIsEmpty(item)" class="bc-muted-note">{{ t('common.loading') }}</div>
          <div v-else-if="state[item.inventoryId]?.error && blockIsEmpty(item)" class="bc-error">{{ state[item.inventoryId].error }}</div>

          <MapTile
            v-else-if="isMapBlock(item)"
            :map-state="item.mapState ?? null"
            :legacy-url="item.mapUrl ?? null"
            @update="updatePinnedBlock(item.inventoryId, $event)"
          />

          <template v-else-if="isCharacterBlock(item)">
            <div v-if="!characterBars(item.inventoryId)" class="bc-muted-note">{{ t('common.noData') }}</div>
            <div v-else class="vitals">
              <div v-for="bar in vitalsRows(item.inventoryId)" :key="bar.id" class="vital">
                <div class="vital-head">
                  <span class="vital-label"><span class="vital-mark" :style="{ background: bar.color }"></span>{{ bar.label }}</span>
                  <span v-if="bar.warn" class="vital-warn">{{ bar.warn }}</span>
                </div>
                <div class="bc-bar vital-bar" :style="{ '--bar-color': bar.color }">
                  <div class="bc-bar-track"><div class="bc-bar-fill" :style="{ width: `${bar.pct}%` }"></div></div>
                  <span class="bc-bar-text">{{ bar.text }}</span>
                </div>
              </div>
              <div class="buffs">
                <h3 class="bc-section">{{ t('dashboard.buffs') }}</h3>
                <div v-if="!characterActiveBuffs(item.inventoryId).length" class="bc-muted-note">
                  {{ characterBuffsLoaded(item.inventoryId) ? t('dashboard.noActiveBuffs') : '—' }}
                </div>
                <ul v-else class="buff-list">
                  <li
                    v-for="buff in characterActiveBuffs(item.inventoryId)"
                    :key="buff.id"
                    class="buff"
                    :class="{ 'buff--food': buff.isFoodBuff, 'buff--bad': !buff.beneficial }"
                  >
                    <span class="buff-name">{{ buff.description }}</span>
                    <span class="buff-time">{{ buff.timeLabel }}</span>
                  </li>
                </ul>
              </div>
            </div>
          </template>

          <MarketBlock
            v-else-if="isMarketBlock(item)"
            :block="item"
            :market="state[item.inventoryId] ?? null"
            @update="updatePinnedBlock(item.inventoryId, $event)"
            @load="loadMarketForPlayer(item.playerEntityId)"
          />

          <SkillProgressCard
            v-else-if="isSkillBlock(item)"
            :level-table="levelTable"
            :xp="skillXp(item)"
            :rank="state[item.inventoryId]?.skillRanking?.rank ?? null"
            :rank-loading="Boolean(state[item.inventoryId]?.loading)"
            :total-players="state[item.inventoryId]?.skillRanking?.totalPlayers ?? null"
            :above="state[item.inventoryId]?.skillAbove ?? null"
            :rank-scope="skillRankScope(item)"
            :active-ranking="skillActiveRanking(item)"
            :active-days="ACTIVE_DAYS"
            :skill-id="item.skillId"
            :player-entity-id="item.playerEntityId ? String(item.playerEntityId) : null"
            :size="skillTileSize(item.inventoryId)"
            :history="skillHistory(item)"
            :tracked-since="skillTrackedSince(item)"
            :chart-tab="item.skillChartTab"
            :pace-period="item.skillPacePeriod"
            :ore-tier="item.skillOreTier ?? null"
            @update-view="updatePinnedBlock(item.inventoryId, $event)"
          />

          <PlannerBlock
            v-else-if="isPlannerBlock(item)"
            :target="plannerTarget(item.inventoryId)"
            :plan="plannerPlan(item.inventoryId)"
            :quantity="plannerQuantity(item.inventoryId)"
            :loading="plannerLoading(item.inventoryId)"
            :error="plannerBlockError(item.inventoryId)"
            :view="item.plannerView ?? 'base'"
            :done="item.plannerDone ?? {}"
            :item-meta="plannerItemMeta"
            :wide="plannerTileSize(item.inventoryId) === 'large'"
            @edit="openEditPlannerWizard(item.inventoryId)"
            @update-view="updatePinnedBlock(item.inventoryId, { plannerView: $event })"
            @update-quantity="setPlannerQuantity(item.inventoryId, $event)"
            @override="setPlannerRecipeOverride(item.inventoryId, $event)"
            @toggle-done="togglePlannerDone(item.inventoryId, $event)"
            @reset-done="updatePinnedBlock(item.inventoryId, { plannerDone: {} })"
          />

          <template v-else>
            <div v-if="!hasInventoryGrid(item.inventoryId)" class="bc-muted-note">{{ t('common.empty') }}</div>
            <div v-else class="inventory-grid" :style="inventoryGridStyle(item.inventoryId)">
              <div
                v-for="(slot, i) in displayInventorySlots(item.inventoryId)"
                :key="`${slot.kind}-${i}`"
                class="inventory-slot"
                :class="[`inventory-slot--${slot.kind}`, { 'inventory-slot--empty': !hasContents(slot.pocket) }]"
              >
                <template v-if="hasContents(slot.pocket)">
                  <ItemCard
                    variant="mini"
                    badge
                    :name="itemMeta(slot.pocket.contents).name"
                    :icon-asset-name="itemMeta(slot.pocket.contents).iconAssetName"
                    :item-type="itemMeta(slot.pocket.contents).type"
                    :tier="itemMeta(slot.pocket.contents).tier"
                    :rarity-str="itemMeta(slot.pocket.contents).rarityStr"
                  />
                  <span class="slot-qty">{{ quantityDisplay(slot.pocket.contents.quantity) }}</span>
                </template>
              </div>
            </div>
          </template>
        </div>
      </section>
    </div>

    <div v-if="wizard.open" class="modal-backdrop" @click.self="closeAddWizard">
      <section class="bc-panel wizard" role="dialog" aria-modal="true" :aria-label="wizardTitle">
        <header class="bc-panel-head">
          <span v-if="wizardKicker" class="bc-kicker">{{ wizardKicker }}</span>
          <h2 class="bc-title">{{ wizardTitle }}</h2>
          <span class="bc-ornament"></span>
          <div class="dash-card-tools wizard-tools">
            <button type="button" class="bc-close" :title="t('common.closeEsc')" @click="closeAddWizard">
              <AppIcon name="close" :size="12" />
            </button>
          </div>
        </header>

        <div class="bc-panel-body wizard-body">
          <div v-if="wizard.step === 'type'" class="wizard-types">
            <button
              v-for="type in BLOCK_TYPES"
              :key="type.id"
              type="button"
              class="wizard-type"
              @click="chooseBlockType(type.id)"
            >
              <span class="wizard-type-icon"><AppIcon :name="type.icon" :size="22" /></span>
              <span class="wizard-type-text">
                <span class="wizard-type-title">{{ type.title }}</span>
                <span class="wizard-type-desc">{{ type.desc }}</span>
              </span>
            </button>
          </div>

          <template v-else-if="wizard.step === 'search'">
            <div class="wizard-search-row">
              <div v-if="wizard.blockType === 'inventory'" class="bc-seg wizard-source">
                <button type="button" class="bc-seg-btn" :class="{ active: wizard.sourceType === 'player' }" @click="chooseSourceType('player')">{{ t('wizard.player') }}</button>
                <button type="button" class="bc-seg-btn" :class="{ active: wizard.sourceType === 'claim' }" @click="chooseSourceType('claim')">{{ t('wizard.claim') }}</button>
              </div>
              <label class="bc-search wizard-search">
                <AppIcon name="search" :size="15" />
                <input
                  ref="wizardSearchInput"
                  v-model="wizard.query"
                  v-focus
                  class="bc-input"
                  type="text"
                  :placeholder="wizardSearchesPlayers ? t('wizard.playerName') : t('wizard.claimName')"
                />
              </label>
            </div>
            <div v-if="wizard.searching" class="bc-muted-note">{{ t('common.searching') }}</div>
            <div v-else-if="wizard.error" class="bc-error">{{ wizard.error }}</div>
            <div v-if="wizard.results.length" class="wizard-list">
              <button
                v-for="result in wizard.results"
                :key="result.entityId"
                type="button"
                class="wizard-row"
                @click="selectWizardEntity(result)"
              >
                <AppIcon :name="wizardSearchesPlayers ? 'player' : 'claim'" :size="16" class="wizard-row-icon" />
                <span class="wizard-row-title">{{ wizardResultTitle(result) }}</span>
                <span v-if="wizardSearchesPlayers" class="wizard-row-meta">
                  <span v-if="result.signedIn" class="wizard-online">{{ t('common.inGame') }}</span>
                </span>
                <span v-else class="wizard-row-meta">{{ wizardResultMeta(result) }}</span>
              </button>
            </div>
          </template>

          <template v-else-if="wizard.step === 'inventory'">
            <div v-if="wizard.loadingInventories" class="bc-muted-note">{{ t('wizard.loadingInventories') }}</div>
            <div v-else-if="wizard.error" class="bc-error">{{ wizard.error }}</div>
            <div v-else-if="wizard.inventories.length === 0" class="bc-muted-note">{{ t('wizard.noInventories') }}</div>
            <div v-else class="wizard-groups">
              <section v-for="group in wizardInventoryGroups" :key="group.id" class="wizard-group">
                <h3 class="bc-section">{{ group.label }}</h3>
                <button
                  v-for="inv in group.items"
                  :key="inv.entityId"
                  type="button"
                  class="wizard-row wizard-inv"
                  :disabled="isPinned(inv.entityId)"
                  @click="addWizardInventory(inv)"
                >
                  <span class="wizard-inv-head">
                    <span class="wizard-row-title">{{ wizardInventoryTitle(inv) }}</span>
                    <span class="wizard-row-meta">{{ isPinned(inv.entityId) ? t('wizard.alreadyPinnedLower') : wizardInventoryMeta(inv) }}</span>
                  </span>
                  <span v-if="wizardPreviewPockets(inv).length" class="wizard-inv-preview">
                    <span
                      v-for="(pocket, idx) in wizardPreviewPockets(inv).slice(0, WIZARD_PREVIEW_MAX)"
                      :key="`${inv.entityId}-${idx}`"
                      class="wizard-preview-slot"
                    >
                      <ItemCard
                        variant="mini"
                        :name="itemMeta(pocket.contents).name"
                        :icon-asset-name="itemMeta(pocket.contents).iconAssetName"
                        :item-type="itemMeta(pocket.contents).type"
                        :tier="itemMeta(pocket.contents).tier"
                        :rarity-str="itemMeta(pocket.contents).rarityStr"
                      />
                      <span class="wizard-preview-qty">{{ wizardPreviewQty(pocket.contents) }}</span>
                    </span>
                    <span v-if="wizardPreviewPockets(inv).length > WIZARD_PREVIEW_MAX" class="wizard-preview-more">
                      +{{ wizardPreviewPockets(inv).length - WIZARD_PREVIEW_MAX }}
                    </span>
                  </span>
                </button>
              </section>
            </div>
          </template>

          <template v-else-if="wizard.step === 'skill'">
            <div v-if="wizard.loadingSkills" class="bc-muted-note">{{ t('wizard.loadingSkills') }}</div>
            <div v-else-if="wizard.error" class="bc-error">{{ wizard.error }}</div>
            <div v-else class="wizard-groups">
              <section v-for="group in wizardSkillGroups" :key="group.id" class="wizard-group">
                <h3 class="bc-section">{{ group.label }}</h3>
                <div class="wizard-skill-grid">
                  <button
                    v-for="skill in group.items"
                    :key="skill.id"
                    type="button"
                    class="wizard-row wizard-skill"
                    :class="{ 'wizard-skill--current': skillBlockId(wizard.selectedEntity?.entityId, skill.id) === wizard.editInventoryId }"
                    :disabled="isPinned(skillBlockId(wizard.selectedEntity?.entityId, skill.id))"
                    :title="wizardSkillTitle(skill)"
                    @click="addWizardSkill(skill)"
                  >
                    <span class="wizard-skill-glyph">{{ skillGlyph(skill.id) }}</span>
                    <span class="wizard-row-title">{{ skill.name }}</span>
                    <span class="wizard-skill-level">{{ skill.level }}</span>
                    <span class="wizard-row-meta wizard-skill-rank">{{ skill.rank ? `#${formatNumber(skill.rank)}` : '' }}</span>
                  </button>
                </div>
              </section>
            </div>
          </template>

          <template v-else-if="wizard.step === 'planner'">
            <label class="bc-search">
              <AppIcon name="search" :size="15" />
              <input
                v-focus
                :value="wizard.plannerQuery"
                class="bc-input"
                type="text"
                :placeholder="t('wizard.craftItem')"
                @input="setWizardPlannerQuery($event.target.value)"
              />
            </label>
            <div v-if="wizard.plannerSearching" class="bc-muted-note">{{ t('common.searching') }}</div>
            <div v-else-if="wizard.error" class="bc-error">{{ wizard.error }}</div>
            <div v-if="wizard.plannerResults.length" class="wizard-list wizard-planner-results">
              <button
                v-for="result in wizard.plannerResults"
                :key="`${result.itemType}-${result.id}`"
                type="button"
                class="wizard-row"
                @click="selectWizardPlannerTarget(result)"
              >
                <ItemCard
                  variant="mini"
                  :name="plannerItemMeta(result).name"
                  :icon-asset-name="plannerItemMeta(result).iconAssetName"
                  :item-type="plannerItemMeta(result).type"
                  :tier="plannerItemMeta(result).tier"
                />
                <span class="wizard-row-title">{{ plannerItemMeta(result).name }}</span>
              </button>
            </div>
            <div v-if="wizard.plannerTarget" class="wizard-planner-target">
              <div class="wizard-planner-head">
                <ItemCard
                  variant="mini"
                  badge
                  :name="plannerItemMeta(wizard.plannerTarget).name"
                  :icon-asset-name="plannerItemMeta(wizard.plannerTarget).iconAssetName"
                  :item-type="plannerItemMeta(wizard.plannerTarget).type"
                  :tier="plannerItemMeta(wizard.plannerTarget).tier"
                />
                <span class="wizard-row-title">{{ plannerItemMeta(wizard.plannerTarget).name }}</span>
                <label class="wizard-qty">
                  {{ t('marketItem.qty') }}
                  <input
                    class="bc-input wizard-qty-input"
                    type="number"
                    min="1"
                    step="1"
                    :value="wizard.plannerQuantity"
                    @change="wizard.plannerQuantity = Math.max(1, Math.floor(Number($event.target.value) || 1))"
                  />
                </label>
              </div>
              <div v-if="wizard.plannerLoadingRecipes" class="bc-muted-note">{{ t('wizard.loadingRecipes') }}</div>
              <template v-else-if="wizard.plannerRecipeOptions.length > 1">
                <h3 class="bc-section">{{ t('common.recipe') }}</h3>
                <div class="wizard-recipes">
                  <button
                    v-for="recipe in wizard.plannerRecipeOptions"
                    :key="recipe.id"
                    type="button"
                    class="wizard-recipe"
                    :class="{ active: wizard.plannerSelectedRecipeId === String(recipe.id) }"
                    :title="plannerRecipeOptionTitle(recipe)"
                    @click="wizard.plannerSelectedRecipeId = String(recipe.id)"
                  >
                    <span class="wizard-recipe-title">{{ plannerRecipeOptionTitle(recipe) }}</span>
                    <span class="wizard-recipe-meta">{{ plannerRecipeOptionMeta(recipe) }}</span>
                  </button>
                </div>
              </template>
              <div v-else-if="wizard.plannerRecipeOptions.length === 1" class="bc-muted-note">
                {{ plannerRecipeOptionTitle(wizard.plannerRecipeOptions[0]) }} · {{ plannerRecipeOptionMeta(wizard.plannerRecipeOptions[0]) }}
              </div>
            </div>
          </template>
        </div>

        <footer v-if="wizard.step !== 'type' && !(wizard.editInventoryId && wizard.step === 'skill')" class="wizard-foot">
          <button v-if="!wizard.editInventoryId" type="button" class="bc-btn" @click="wizardBack">
            <AppIcon name="back" :size="14" />{{ t('wizard.back') }}
          </button>
          <button
            v-if="wizard.step === 'planner'"
            type="button"
            class="bc-btn bc-btn--primary wizard-confirm"
            :disabled="!wizardPlannerCanConfirm || wizard.plannerLoadingRecipes"
            @click="confirmWizardPlanner"
          >
            {{ wizard.editInventoryId ? t('wizard.save') : t('wizard.add') }}
          </button>
        </footer>
      </section>
    </div>
  </div>
</template>

<script>
import { computed, markRaw, reactive, ref, watch, onMounted, onUnmounted } from 'vue'
import { fetchBitjita, fetchXpHistory, postBitjita } from '../api'
import { usePinned } from '../composables/usePinned'
import { useItemCache } from '../composables/useItemCache'
import { useBitjitaLive } from '../composables/useBitjitaLive'
import { useStaminaAlerts } from '../composables/useStaminaAlerts'
import { activeBuffsFromPayload, hasActiveFoodBuff } from '../playerBuffs'
import { characterVitalsFromStats, formatVitalsValue, vitalsBarPercent } from '../playerStats'
import {
  collectCraftGraphItemRefs,
  computeCraftPlan,
  gatherSourceFromDetails,
  itemCraftKey,
  listRecipeOptions,
  resolveCraftGraph,
  resolveDefaultRecipeId,
} from '../plannerCrafting'
import { dedupePlannerSearchResults, normalizePlannerSearchEntry } from '../plannerSearch'
import { SKILL_CATEGORIES, formatXpShort, levelForXp, parseLevelTable, skillGlyph } from '../skillProgress'
import { ACTIVE_DAYS, activeSkillPlayers, activeSkillRank, skillPlace } from '../skillLeaderboard'
import { carriedCoins, collectRows, collectSummary } from '../marketTrades'
import { FLAG_PCT, marketBookKey, playerOrderRows, sellBook } from '../marketOrders'
import { appendLivePoint, mergeHistoryPoints, splitHistorySamples } from '../xpHistory'
import { applyTileDrop, computeTileLayout, savedTilePosition } from '../dashboardLayout'
import { useTileDrag, useTileFlip } from '../composables/useTileDrag'
import { formatNumber, intlLocale, t } from '../i18n'
import AppIcon from './AppIcon.vue'
import ItemCard from './ItemCard.vue'
import SkillProgressCard from './SkillProgressCard.vue'
import MapTile from './MapTile.vue'
import MarketBlock from './MarketBlock.vue'
import PlannerBlock from './PlannerBlock.vue'

const SEARCH_DEBOUNCE_MS = 500
const TILE_WIDTH_PX = 380
const TILE_GAP_PX = 14
const WIZARD_PREVIEW_MAX = 24
const MARKET_TRADES_LIMIT = 100
const MARKET_BOOK_CONCURRENCY = 4
const BLOCK_TYPES = [
  'inventory',
  'market',
  'character',
  'skill',
  'planner',
  'map',
].map((id) => ({
  id,
  icon: id,
  get title() { return t(`blockTypes.${id}.title`) },
  get desc() { return t(`blockTypes.${id}.desc`) },
}))

export default {
  name: 'PinnedDashboard',
  components: { AppIcon, ItemCard, MapTile, MarketBlock, PlannerBlock, SkillProgressCard },
  directives: {
    focus: { mounted: (el) => el.focus() },
  },
  setup() {
    const { pinned, isPinned, unpin, pinBlock, setDashboardLayouts, updatePinnedBlock } = usePinned()
    const state = reactive({})
    const rootRef = ref(null)
    const gridRef = ref(null)
    const wizardSearchInput = ref(null)
    const dashboardCols = ref(Math.max(1, Math.floor((window.innerWidth - 32 + TILE_GAP_PX) / (TILE_WIDTH_PX + TILE_GAP_PX))))
    const wizard = reactive({
      open: false,
      step: 'type',
      blockType: 'inventory',
      sourceType: 'player',
      query: '',
      searching: false,
      error: null,
      results: [],
      selectedEntity: null,
      inventories: [],
      loadingInventories: false,
      editInventoryId: null,
      plannerQuery: '',
      plannerSearching: false,
      plannerResults: [],
      plannerTarget: null,
      plannerRecipeOptions: [],
      plannerSelectedRecipeId: null,
      plannerCraftRecipes: [],
      plannerQuantity: 1,
      plannerLoadingRecipes: false,
      skillOptions: [],
      loadingSkills: false,
    })
    const levelTable = ref(null)
    let levelTablePromise = null
    const xpHistory = reactive({})
    const { resolveItem, prefetch, prefetchItems, seedCatalog } = useItemCache()
    const { syncPlayerSubscriptions, syncExperienceSubscriptions, experienceByPlayer, liveSnapshot, staminaByPlayer } = useBitjitaLive()
    const BULK_CHUNK = 100
    const plannerBlockState = reactive({})
    const plannerRecipeCache = markRaw({})
    const plannerSourceCache = markRaw({})
    let wizardPlannerSearchTimer = null
    let wizardPlannerSearchGen = 0
    let wizardSearchTimer = null
    let wizardSearchGen = 0
    const wizardSteps = computed(() => {
      if (wizard.blockType === 'inventory') return ['type', 'search', 'inventory']
      if (wizard.blockType === 'planner') return ['type', 'planner']
      if (wizard.blockType === 'skill') return ['type', 'search', 'skill']
      return ['type', 'search']
    })
    const wizardInventoryCategoryOrder = [
      'inventory',
      'house',
      'bank',
      'recovery',
      'claim',
      'trade',
    ].map((id) => ({ id, get label() { return t(`wizard.categories.${id}`) } }))

    const wizardPlannerCanConfirm = computed(() => {
      if (!wizard.plannerTarget?.id) return false
      if (wizard.plannerRecipeOptions.length > 1 && !wizard.plannerSelectedRecipeId) return false
      return true
    })
    const wizardInventoryGroups = computed(() => {
      const buckets = Object.fromEntries(wizardInventoryCategoryOrder.map((cat) => [cat.id, { ...cat, items: [] }]))
      wizard.inventories.forEach((inv) => {
        const categoryId = wizardInventoryCategory(inv)
        buckets[categoryId]?.items.push(inv)
      })
      return wizardInventoryCategoryOrder
        .map((cat) => buckets[cat.id])
        .filter((group) => group.items.length > 0)
    })
    const wizardSkillGroups = computed(() => SKILL_CATEGORIES
      .map((cat) => ({ ...cat, items: wizard.skillOptions.filter((skill) => skill.category === cat.id) }))
      .filter((group) => group.items.length > 0))
    const wizardSearchesPlayers = computed(() => wizard.blockType !== 'inventory' || wizard.sourceType === 'player')
    const wizardTitle = computed(() => (wizard.editInventoryId ? t('dashboard.editBlock') : t('dashboard.addBlock')))
    const wizardKicker = computed(() => {
      if (wizard.step === 'type') return ''
      const type = BLOCK_TYPES.find((option) => option.id === wizard.blockType)?.title ?? ''
      const entity = wizard.selectedEntity
      const entityName = entity ? (entity.username ?? entity.name) : null
      if (wizard.step !== 'search' && entityName) return `${type} · ${entityName}`
      return type
    })
    const dashboardRefreshing = computed(() => pinned.value.some((p) => state[p.inventoryId]?.refreshing || state[p.inventoryId]?.loading))
    const dashboardUpdatedLabel = computed(() => {
      const times = pinned.value.map((p) => state[p.inventoryId]?.updatedAt).filter(Boolean)
      if (!times.length) return t('dashboard.refresh')
      const last = new Date(Math.max(...times)).toLocaleTimeString(intlLocale())
      return t('dashboard.refreshTitle', { time: last })
    })

    const layoutInfo = computed(() => computeTileLayout(
      pinned.value.map((item) => ({
        id: item.inventoryId,
        span: dashboardCardSpan(item.inventoryId),
        saved: savedTilePosition(item, dashboardCols.value),
      })),
      dashboardCols.value,
    ))
    const dropCells = computed(() => {
      const cells = []
      for (let y = 0; y <= layoutInfo.value.rows; y += 1) {
        for (let x = 0; x < layoutInfo.value.cols; x += 1) cells.push({ x, y })
      }
      return cells
    })
    const { drag, onTilePointerDown } = useTileDrag({ gridRef, rootRef, layout: layoutInfo, onDrop: onTileDrop })
    const tileFlip = useTileFlip(gridRef, computed(() => `${layoutInfo.value.cols}|${JSON.stringify(layoutInfo.value.positions)}`))
    function onTileDrop(id, target, from) {
      if (!target) {
        tileFlip.slideBack(id, from)
        return
      }
      const cols = layoutInfo.value.cols
      const next = applyTileDrop(layoutInfo.value, id, target)
      const legacy = legacyLayout()
      if (legacy && legacy.cols !== cols) setDashboardLayouts(legacy.cols, legacy.positions)
      tileFlip.landFrom(id, from)
      setDashboardLayouts(cols, next)
    }
    function legacyLayout() {
      const positions = {}
      let cols = 0
      pinned.value.forEach((item) => {
        if (item.dashboardLayouts) return
        const pos = savedTilePosition({ dashboardLayout: item.dashboardLayout }, 1)
        if (!pos) return
        positions[item.inventoryId] = pos
        cols = Math.max(cols, pos.x + dashboardCardSpan(item.inventoryId))
      })
      return cols ? { cols, positions } : null
    }
    function gridArea(x, y, span) {
      return { gridColumn: `${x + 1} / span ${span}`, gridRow: `${y + 1} / span 1` }
    }

    function ensureInventoryState(inventoryId) {
      if (!state[inventoryId]) {
        state[inventoryId] = { loading: true, refreshing: false, error: null, inventory: null, pockets: [], updatedAt: null }
      }
      return state[inventoryId]
    }
    function sourceType(pinItem) {
      return pinItem.sourceType ?? 'player'
    }
    function blockType(pinItem) {
      return pinItem.blockType ?? 'inventory'
    }
    function isMarketBlock(pinItem) {
      return blockType(pinItem) === 'market'
    }
    function isPlannerBlock(pinItem) {
      return blockType(pinItem) === 'planner'
    }
    function ensurePlannerState(inventoryId) {
      if (!plannerBlockState[inventoryId]) {
        plannerBlockState[inventoryId] = {
          target: null,
          quantity: 1,
          craftRecipes: [],
          selectedRecipeId: null,
          plan: null,
          loading: false,
          error: null,
          calcGen: 0,
        }
      }
      return plannerBlockState[inventoryId]
    }
    function syncPlannerFromPin(inventoryId, pinItem) {
      const st = ensurePlannerState(inventoryId)
      if (pinItem?.plannerTarget?.id) {
        st.target = {
          id: String(pinItem.plannerTarget.id),
          itemType: pinItem.plannerTarget.itemType ?? 0,
        }
        st.quantity = Math.max(1, Number(pinItem.plannerQuantity) || 1)
        st.selectedRecipeId = pinItem.plannerRecipeId ? String(pinItem.plannerRecipeId) : null
      }
    }
    function plannerTarget(inventoryId) {
      return ensurePlannerState(inventoryId).target
    }
    function plannerQuantity(inventoryId) {
      return ensurePlannerState(inventoryId).quantity
    }
    function plannerBlockError(inventoryId) {
      return ensurePlannerState(inventoryId).error
    }
    function plannerRecipeOptionTitle(recipe) {
      if (!recipe) return t('common.recipe')
      if (recipe.buildingName) {
        return recipe.buildingTier ? `${recipe.buildingName} · T${recipe.buildingTier}` : recipe.buildingName
      }
      return recipe.name
    }
    function plannerRecipeOptionMeta(recipe) {
      if (!recipe) return ''
      const parts = [t('planner.output', { n: recipe.outputQuantity }), t('planner.ingredients', { n: recipe.ingredientCount })]
      if (!recipe.buildingName && recipe.name) parts.push(recipe.name)
      return parts.join(' · ')
    }
    function plannerPlan(inventoryId) {
      return ensurePlannerState(inventoryId).plan
    }
    function plannerTileSize(inventoryId) {
      const block = pinned.value.find((p) => p.inventoryId === inventoryId)
      return block?.plannerTileSize === 'large' ? 'large' : 'small'
    }
    function setPlannerTileSize(inventoryId, size) {
      if (size !== 'small' && size !== 'large') return
      updatePinnedBlock(inventoryId, { plannerTileSize: size })
    }
    function setPlannerRecipeOverride(inventoryId, { key, value, isRoot }) {
      const pin = pinned.value.find((p) => p.inventoryId === inventoryId)
      if (!pin) return
      const st = ensurePlannerState(inventoryId)
      if (isRoot) {
        st.selectedRecipeId = value ? String(value) : null
        updatePinnedBlock(inventoryId, { plannerRecipeId: st.selectedRecipeId })
      } else {
        const overrides = { ...(pin.plannerRecipeOverrides ?? {}) }
        if (value) overrides[key] = String(value)
        else delete overrides[key]
        updatePinnedBlock(inventoryId, { plannerRecipeOverrides: overrides })
      }
      calculatePlannerMaterials(inventoryId, { reuseRecipes: true })
    }
    function setPlannerQuantity(inventoryId, quantity) {
      const qty = Math.max(1, Math.floor(Number(quantity) || 1))
      const st = ensurePlannerState(inventoryId)
      if (st.quantity === qty) return
      st.quantity = qty
      updatePinnedBlock(inventoryId, { plannerQuantity: qty })
      calculatePlannerMaterials(inventoryId, { reuseRecipes: true })
    }
    function togglePlannerDone(inventoryId, key) {
      const pin = pinned.value.find((p) => p.inventoryId === inventoryId)
      if (!pin) return
      const done = { ...(pin.plannerDone ?? {}) }
      if (done[key]) delete done[key]
      else done[key] = true
      updatePinnedBlock(inventoryId, { plannerDone: done })
    }
    function plannerLoading(inventoryId) {
      return ensurePlannerState(inventoryId).loading
    }
    async function fetchCraftDetails(itemType, itemId) {
      const id = String(itemId ?? '').trim()
      if (!/^\d+$/.test(id)) {
        throw new Error(t('wizard.badItemId'))
      }
      const path = itemType === 1 ? `/cargo/${id}` : `/items/${id}`
      const data = await fetchBitjita(path)
      const entity = data.item ?? data.cargo
      if (entity) {
        seedCatalog(itemType === 1 ? { cargos: [entity] } : { items: [entity] })
      }
      return data
    }
    async function fetchPlannerTargetDetails(target) {
      let resolvedTarget = { id: String(target.id), itemType: target.itemType ?? 0 }
      let data = await fetchCraftDetails(resolvedTarget.itemType, resolvedTarget.id)
      let recipes = data.craftingRecipes ?? []

      const canCraftSelf = recipes.some((r) => {
        if (!r.consumedItems?.length) return false
        if (!r.craftedItems?.some((c) => String(c.id) === String(resolvedTarget.id))) return false
        return !/recraft/i.test(r.name ?? '')
      })

      if (!canCraftSelf) {
        const name = plannerItemMeta(resolvedTarget).name
        const tier = plannerItemMeta(resolvedTarget).tier
        try {
          const search = await fetchBitjita('/items', { q: name })
          for (const raw of search.items ?? []) {
            const candidate = { id: String(raw.id), itemType: 0 }
            if (plannerItemMeta(candidate).name !== name) continue
            if (Number(plannerItemMeta(candidate).tier) !== Number(tier)) continue
            if (String(candidate.id) === String(resolvedTarget.id)) continue
            try {
              const altData = await fetchCraftDetails(0, candidate.id)
              const altRecipes = altData.craftingRecipes ?? []
              const altCanCraft = altRecipes.some((r) =>
                r.consumedItems?.length &&
                r.craftedItems?.some((c) => String(c.id) === String(candidate.id)) &&
                !/recraft/i.test(r.name ?? ''),
              )
              if (altCanCraft) {
                resolvedTarget = candidate
                data = altData
                recipes = altRecipes
                break
              }
            } catch {
            }
          }
        } catch {
        }
      }

      return { target: resolvedTarget, recipes, data }
    }
    async function fetchCraftRecipesSafe(itemType, itemId) {
      const id = String(itemId ?? '').trim()
      if (!/^\d+$/.test(id)) return []
      try {
        const data = await fetchBitjita(itemType === 1 ? `/cargo/${id}` : `/items/${id}`)
        const entity = data.item ?? data.cargo
        if (entity) {
          seedCatalog(itemType === 1 ? { cargos: [entity] } : { items: [entity] })
        }
        plannerSourceCache[itemCraftKey(itemType, id)] = gatherSourceFromDetails(data)
        return data.craftingRecipes ?? []
      } catch {
        return []
      }
    }
    async function calculatePlannerMaterials(inventoryId, options = {}) {
      const st = ensurePlannerState(inventoryId)
      const target = st.target
      if (!target?.id) return
      const gen = ++st.calcGen
      st.loading = true
      st.error = null
      try {
        let recipes
        if (options.reuseRecipes && st.craftRecipes?.length) {
          recipes = st.craftRecipes
        } else {
          const resolved = await fetchPlannerTargetDetails(target)
          if (gen !== st.calcGen) return
          if (resolved.target.id !== target.id || resolved.target.itemType !== target.itemType) {
            st.target = resolved.target
          }
          recipes = resolved.recipes
          st.craftRecipes = recipes

          plannerRecipeCache[itemCraftKey(st.target.itemType, st.target.id)] = recipes
          const pin = pinned.value.find((p) => p.inventoryId === inventoryId)
          const preferredId = st.selectedRecipeId ?? pin?.plannerRecipeId ?? null
          st.selectedRecipeId = preferredId && listRecipeOptions(recipes, st.target.id).some((o) => String(o.id) === String(preferredId))
            ? String(preferredId)
            : await resolveDefaultRecipeId({
              target: st.target,
              recipes,
              recipeCache: plannerRecipeCache,
              fetchRecipes: fetchCraftRecipesSafe,
            })
          if (gen !== st.calcGen) return
        }

        const activeTarget = st.target
        const pin = pinned.value.find((p) => p.inventoryId === inventoryId)
        plannerRecipeCache[itemCraftKey(activeTarget.itemType, activeTarget.id)] = recipes
        const graph = await resolveCraftGraph({
          root: activeTarget,
          rootRecipeId: st.selectedRecipeId,
          overrides: pin?.plannerRecipeOverrides ?? {},
          recipeCache: plannerRecipeCache,
          fetchRecipes: fetchCraftRecipesSafe,
        })
        if (gen !== st.calcGen) return

        const refs = collectCraftGraphItemRefs(graph)
        await prefetchItems(refs)
        const unknown = refs.filter((ref) => ref.name && plannerItemMeta(ref).name === `#${ref.id}`)
        if (unknown.length) {
          seedCatalog({
            items: unknown.filter((ref) => ref.itemType !== 1),
            cargos: unknown.filter((ref) => ref.itemType === 1),
          })
        }
        if (gen !== st.calcGen) return

        const plan = computeCraftPlan({ graph, quantity: st.quantity, sources: plannerSourceCache })
        st.plan = plan
      } catch (e) {
        if (gen !== st.calcGen) return
        st.error = e.message
        st.plan = null
      } finally {
        if (gen === st.calcGen) st.loading = false
      }
    }
    function resetWizardPlannerDraft() {
      clearTimeout(wizardPlannerSearchTimer)
      wizardPlannerSearchGen = 0
      wizard.plannerQuery = ''
      wizard.plannerSearching = false
      wizard.plannerResults = []
      wizard.plannerTarget = null
      wizard.plannerRecipeOptions = []
      wizard.plannerSelectedRecipeId = null
      wizard.plannerCraftRecipes = []
      wizard.plannerQuantity = 1
      wizard.plannerLoadingRecipes = false
    }
    function setWizardPlannerQuery(value) {
      wizard.plannerQuery = value
      clearTimeout(wizardPlannerSearchTimer)
      const q = value.trim()
      if (q.length < 2) {
        wizard.plannerResults = []
        wizard.plannerSearching = false
        return
      }
      wizardPlannerSearchTimer = setTimeout(() => {
        searchWizardPlannerItems()
      }, SEARCH_DEBOUNCE_MS)
    }
    async function searchWizardPlannerItems() {
      const q = wizard.plannerQuery.trim()
      if (q.length < 2) {
        wizard.plannerResults = []
        wizard.plannerSearching = false
        return
      }
      const gen = ++wizardPlannerSearchGen
      wizard.plannerSearching = true
      wizard.error = null
      wizard.plannerResults = []
      try {
        const [itemsRes, cargoRes] = await Promise.all([
          fetchBitjita('/items', { q }),
          fetchBitjita('/cargo', { q }),
        ])
        if (gen !== wizardPlannerSearchGen) return
        seedCatalog({ items: itemsRes.items, cargos: cargoRes.cargos })
        const items = (itemsRes.items ?? []).map((item) => normalizePlannerSearchEntry(item, 0))
        const cargos = (cargoRes.cargos ?? []).map((cargo) => normalizePlannerSearchEntry(cargo, 1))
        wizard.plannerResults = dedupePlannerSearchResults([...items, ...cargos], plannerItemMeta)
        if (!wizard.plannerResults.length) wizard.error = t('common.nothingFound')
      } catch (e) {
        if (gen !== wizardPlannerSearchGen) return
        wizard.error = e.message
      } finally {
        if (gen === wizardPlannerSearchGen) wizard.plannerSearching = false
      }
    }
    async function loadWizardPlannerRecipes() {
      if (!wizard.plannerTarget?.id) return
      wizard.plannerLoadingRecipes = true
      wizard.error = null
      try {
        const target = wizard.plannerTarget
        const data = await fetchCraftDetails(target.itemType, target.id)
        const recipes = data.craftingRecipes ?? []
        plannerRecipeCache[itemCraftKey(target.itemType, target.id)] = recipes
        const options = listRecipeOptions(recipes, target.id)
        const preferredId = wizard.plannerSelectedRecipeId
        const selectedId = preferredId && options.some((o) => String(o.id) === String(preferredId))
          ? String(preferredId)
          : await resolveDefaultRecipeId({
            target,
            recipes,
            recipeCache: plannerRecipeCache,
            fetchRecipes: fetchCraftRecipesSafe,
          })
        if (wizard.plannerTarget !== target) return
        wizard.plannerCraftRecipes = recipes
        wizard.plannerRecipeOptions = options
        wizard.plannerSelectedRecipeId = selectedId
      } catch (e) {
        wizard.error = e.message
        wizard.plannerRecipeOptions = []
        wizard.plannerSelectedRecipeId = null
      } finally {
        wizard.plannerLoadingRecipes = false
      }
    }
    async function selectWizardPlannerTarget(entry) {
      wizard.plannerTarget = { id: String(entry.id), itemType: entry.itemType }
      wizard.plannerResults = []
      wizard.plannerQuery = plannerItemMeta(entry).name
      wizard.error = null
      await loadWizardPlannerRecipes()
    }
    async function confirmWizardPlanner() {
      if (!wizard.plannerTarget?.id || !wizardPlannerCanConfirm.value) return
      const target = wizard.plannerTarget
      const qty = Math.max(1, Math.floor(Number(wizard.plannerQuantity) || 1))
      const recipeId = wizard.plannerSelectedRecipeId
      const name = plannerItemMeta(target).name
      const patch = {
        plannerTarget: { id: target.id, itemType: target.itemType },
        plannerRecipeId: recipeId,
        plannerQuantity: qty,
        inventoryName: `${t('blockTypes.planner.title')} · ${name}`,
      }
      if (wizard.editInventoryId) {
        const inventoryId = wizard.editInventoryId
        const prev = pinned.value.find((p) => p.inventoryId === inventoryId)?.plannerTarget
        if (String(prev?.id) !== String(target.id) || (prev?.itemType ?? 0) !== target.itemType) {
          patch.plannerDone = {}
          patch.plannerRecipeOverrides = {}
        }
        updatePinnedBlock(inventoryId, patch)
        syncPlannerFromPin(inventoryId, pinned.value.find((p) => p.inventoryId === inventoryId))
        await calculatePlannerMaterials(inventoryId)
        closeAddWizard()
        return
      }
      const blockId = `planner:${Date.now()}`
      pinBlock({
        blockType: 'planner',
        sourceType: 'planner',
        inventoryId: blockId,
        ...patch,
      })
      syncPlannerFromPin(blockId, pinned.value.find((p) => p.inventoryId === blockId))
      await calculatePlannerMaterials(blockId)
      closeAddWizard()
    }
    function openEditPlannerWizard(inventoryId) {
      const pin = pinned.value.find((p) => p.inventoryId === inventoryId)
      if (!pin || !isPlannerBlock(pin)) return
      resetWizard()
      wizard.open = true
      wizard.blockType = 'planner'
      wizard.step = 'planner'
      wizard.editInventoryId = inventoryId
      wizard.plannerQuantity = Math.max(1, Number(pin.plannerQuantity) || 1)
      if (pin.plannerTarget?.id) {
        wizard.plannerTarget = {
          id: String(pin.plannerTarget.id),
          itemType: pin.plannerTarget.itemType ?? 0,
        }
        wizard.plannerQuery = plannerItemMeta(wizard.plannerTarget).name
        wizard.plannerSelectedRecipeId = pin.plannerRecipeId ? String(pin.plannerRecipeId) : null
        loadWizardPlannerRecipes()
      }
    }
    async function initPlannerBlocks() {
      const pins = pinned.value.filter(isPlannerBlock)
      for (const pin of pins) {
        syncPlannerFromPin(pin.inventoryId, pin)
        const st = plannerBlockState[pin.inventoryId]
        if (st?.target) {
          await prefetchItems([st.target])
          await calculatePlannerMaterials(pin.inventoryId)
        }
      }
    }
    function marketTileSize(inventoryId) {
      const block = pinned.value.find((p) => p.inventoryId === inventoryId)
      return block?.marketTileSize === 'small' ? 'small' : 'large'
    }
    function setMarketTileSize(inventoryId, size) {
      if (size !== 'small' && size !== 'large') return
      updatePinnedBlock(inventoryId, { marketTileSize: size })
    }
    function marketTab(inventoryId) {
      const block = pinned.value.find((p) => p.inventoryId === inventoryId)
      return block?.marketTab === 'trades' ? 'trades' : 'orders'
    }
    function marketCollectStats(inventoryId) {
      const st = state[inventoryId]
      const block = pinned.value.find((p) => p.inventoryId === inventoryId)
      return collectSummary(collectRows(st?.marketCollections, st?.marketTrades, block?.playerEntityId))
    }
    function isMapBlock(pinItem) {
      return blockType(pinItem) === 'map'
    }
    function mapTileSize(inventoryId) {
      const block = pinned.value.find((p) => p.inventoryId === inventoryId)
      return block?.mapTileSize === 'small' ? 'small' : 'large'
    }
    function setMapTileSize(inventoryId, size) {
      if (size !== 'small' && size !== 'large') return
      updatePinnedBlock(inventoryId, { mapTileSize: size })
    }
    function normalizePocket(pocket) {
      if (!pocket?.contents) return { ...pocket, contents: null }
      return {
        ...pocket,
        contents: {
          itemId: pocket.contents.itemId ?? pocket.contents.item_id,
          itemType: pocket.contents.itemType ?? (pocket.contents.item_type === 'cargo' ? 1 : 0),
          quantity: pocket.contents.quantity,
        },
      }
    }
    function normalizeClaimBuilding(building, claim) {
      return {
        entityId: building.entityId,
        inventoryName: building.buildingNickname || building.buildingName || 'Building inventory',
        buildingName: building.buildingName ?? null,
        claimName: claim.name ?? null,
        claimEntityId: claim.entityId,
        regionId: claim.regionId ?? null,
        pockets: (building.inventory ?? []).map(normalizePocket),
      }
    }
    function mergeInventoriesById(lists) {
      const byId = new Map()
      lists.flat().forEach((inv) => {
        if (inv?.entityId != null) byId.set(String(inv.entityId), inv)
      })
      return [...byId.values()]
    }
    function findInventoryById(inventories, inventoryId) {
      return inventories.find((inv) => String(inv.entityId) === String(inventoryId))
    }
    function normalizeHouseInventory(hi, detail, houseSummary) {
      return {
        entityId: hi.entityId,
        inventoryName: hi.buildingNickname || hi.buildingName || detail.buildingName || 'House',
        regionId: detail.regionId ?? null,
        claimName: detail.claimName ?? null,
        buildingName: hi.buildingName ?? null,
        houseName: detail.buildingName ?? null,
        houseBuildingEntityId: detail.buildingEntityId ?? houseSummary?.buildingEntityId ?? null,
        pockets: (hi.inventory ?? []).map(normalizePocket),
        _isHousing: true,
      }
    }
    async function fetchPlayerHousingInventories(playerEntityId, housingList, includeBuildingIds) {
      const houses = Array.isArray(housingList) ? housingList : []
      const includeSet = includeBuildingIds instanceof Set
        ? includeBuildingIds
        : new Set((includeBuildingIds ?? []).map((id) => String(id)))
      const toLoad = houses.filter((h) => includeSet.has(String(h.buildingEntityId)) || !h.isEmpty)
      const houseInvs = await Promise.all(toLoad.map((h) =>
        fetchBitjita(`/players/${playerEntityId}/housing/${h.buildingEntityId}`)
          .then((detail) => {
            seedCatalog(detail)
            return (detail.inventories ?? []).map((hi) => normalizeHouseInventory(hi, detail, h))
          })
          .catch(() => [])
      ))
      return houseInvs.flat()
    }
    async function loadPlayerInventoriesCatalog(playerEntityId, pinnedItems = []) {
      const pinnedHouseIds = pinnedItems
        .filter((p) => p._isHousing && p.houseBuildingEntityId)
        .map((p) => String(p.houseBuildingEntityId))
      const [invRes, housingRes] = await Promise.all([
        fetchBitjita(`/players/${playerEntityId}/inventories`),
        fetchBitjita(`/players/${playerEntityId}/housing`).catch(() => null),
      ])
      const invs = invRes.inventories ?? []
      seedCatalog(invRes)
      const houses = Array.isArray(housingRes) ? housingRes : []
      const includeIds = new Set([
        ...pinnedHouseIds,
        ...houses.filter((h) => !h.isEmpty).map((h) => String(h.buildingEntityId)),
      ])
      let all = mergeInventoriesById([
        invs,
        await fetchPlayerHousingInventories(playerEntityId, houses, includeIds),
      ])
      const hasMissingPinned = pinnedItems.some((p) => !findInventoryById(all, p.inventoryId))
      if (hasMissingPinned && houses.length > 0) {
        all = mergeInventoriesById([
          all,
          await fetchPlayerHousingInventories(
            playerEntityId,
            houses,
            new Set(houses.map((h) => String(h.buildingEntityId))),
          ),
        ])
      }
      return all
    }

    function orderItemId(o) { return o.itemId ?? o.item_id }
    function orderItemType(o) {
      const t = o.itemType ?? o.item_type
      return t === 1 || t === '1' || t === 'cargo' ? 1 : 0
    }
    async function fetchBulkMaps(itemIds, cargoIds) {
      const items = {}
      const cargo = {}
      if (itemIds.length > 0) {
        for (let i = 0; i < itemIds.length; i += BULK_CHUNK) {
          const data = await postBitjita('/market/prices/bulk', { itemIds: itemIds.slice(i, i + BULK_CHUNK), cargoIds: [] })
          Object.assign(items, data?.data?.items ?? {})
        }
      }
      if (cargoIds.length > 0) {
        for (let i = 0; i < cargoIds.length; i += BULK_CHUNK) {
          const data = await postBitjita('/market/prices/bulk', { itemIds: [], cargoIds: cargoIds.slice(i, i + BULK_CHUNK) })
          Object.assign(cargo, data?.data?.cargo ?? {})
        }
      }
      return { items, cargo }
    }
    function orderIdsOfType(orders, itemType) {
      return [...new Set(orders
        .filter((o) => orderItemType(o) === itemType)
        .map(orderItemId)
        .filter((id) => id != null)
        .map(Number)
        .filter((n) => Number.isFinite(n)))]
    }
    async function fetchSellBooks(orders) {
      const goods = [...new Map(orders.map((o) => {
        const key = marketBookKey(orderItemId(o), orderItemType(o))
        return [key, { key, id: orderItemId(o), kind: orderItemType(o) === 1 ? 'cargo' : 'item' }]
      })).values()]
      const books = {}
      let next = 0
      async function worker() {
        while (next < goods.length) {
          const good = goods[next++]
          try {
            const data = await fetchBitjita(`/market/${good.kind}/${encodeURIComponent(good.id)}`)
            books[good.key] = sellBook(data?.sellOrders)
          } catch {}
        }
      }
      await Promise.all(Array.from({ length: Math.min(MARKET_BOOK_CONCURRENCY, goods.length) }, worker))
      return books
    }
    async function loadMarketForPlayer(playerEntityId) {
      const marketPinned = pinned.value.filter((p) => isMarketBlock(p) && String(p.playerEntityId) === String(playerEntityId))
      if (!marketPinned.length) return
      marketPinned.forEach((p) => {
        const st = ensureInventoryState(p.inventoryId)
        const loaded = Array.isArray(st.marketOrders)
        st[loaded ? 'refreshing' : 'loading'] = true
        if (!loaded) st.error = null
      })
      try {
        const data = await fetchBitjita(`/players/${playerEntityId}/market`)
        seedCatalog(data)
        const asList = (value) => (Array.isArray(value) ? value : [])
        const sellOrders = asList(data.sellOrders ?? data.sell_orders)
        await prefetch([{
          pockets: sellOrders.map((order) => ({
            contents: { itemId: orderItemId(order), itemType: orderItemType(order) },
          })),
        }])
        const bulkMaps = await fetchBulkMaps(orderIdsOfType(sellOrders, 0), orderIdsOfType(sellOrders, 1))
        const wantsTrades = marketPinned.some((p) => marketTab(p.inventoryId) === 'trades')
        const [collectionsRes, tradesRes, inventoriesRes, sellBooks] = await Promise.all([
          fetchBitjita(`/players/${playerEntityId}/market-collections`).catch(() => null),
          wantsTrades
            ? fetchBitjita(`/market/player/${playerEntityId}/trades`, { limit: String(MARKET_TRADES_LIMIT) }).catch(() => null)
            : Promise.resolve(null),
          fetchBitjita(`/players/${playerEntityId}/inventories`).catch(() => null),
          fetchSellBooks(sellOrders),
        ])
        marketPinned.forEach((p) => {
          const st = ensureInventoryState(p.inventoryId)
          st.loading = false
          st.refreshing = false
          st.error = null
          st.marketOrders = sellOrders
          st.marketBulkMaps = bulkMaps
          st.marketBooks = sellBooks
          if (collectionsRes) st.marketCollections = collectionsRes.collections ?? []
          if (inventoriesRes) st.marketBalance = carriedCoins(inventoriesRes.inventories, playerEntityId)
          if (tradesRes) {
            st.marketTrades = tradesRes.trades ?? []
            st.marketTradesTotal = Number(tradesRes.total) || (tradesRes.trades?.length ?? 0)
          }
          st.updatedAt = Date.now()
        })
      } catch (e) {
        marketPinned.forEach((p) => {
          const st = ensureInventoryState(p.inventoryId)
          st.loading = false
          st.refreshing = false
          st.error = e.message
        })
      }
    }
    async function loadForPlayer(playerEntityId) {
      const playerPinned = pinned.value.filter((p) =>
        sourceType(p) === 'player' && !isMarketBlock(p) && !isCharacterBlock(p) && !isMapBlock(p) && p.playerEntityId === playerEntityId
      )
      if (!playerPinned.length) return
      playerPinned.forEach((p) => {
        const invState = ensureInventoryState(p.inventoryId)
        invState[hasInventoryGrid(p.inventoryId) ? 'refreshing' : 'loading'] = true
        invState.error = null
      })
      try {
        const invs = await loadPlayerInventoriesCatalog(playerEntityId, playerPinned)
        playerPinned.forEach((pinItem) => {
          const inv = findInventoryById(invs, pinItem.inventoryId)
          const invState = ensureInventoryState(pinItem.inventoryId)
          invState.loading = false
          invState.refreshing = false
          invState.error = inv ? null : t('dashboard.inventoryNotFound')
          if (inv) {
            invState.inventory = inv
            invState.pockets = inv.pockets ?? []
            invState.updatedAt = Date.now()
          }
        })
        await prefetch(invs.map((inv) => ({ ...inv, pockets: (inv.pockets ?? []).slice(0, 200) })))
      } catch (e) {
        playerPinned.forEach((p) => {
          const invState = ensureInventoryState(p.inventoryId)
          invState.loading = false
          invState.refreshing = false
          invState.error = e.message
        })
      }
    }
    async function loadForClaim(claimEntityId) {
      const claimPinned = pinned.value.filter((p) => sourceType(p) === 'claim' && p.claimEntityId === claimEntityId)
      if (!claimPinned.length) return
      claimPinned.forEach((p) => {
        const invState = ensureInventoryState(p.inventoryId)
        invState[hasInventoryGrid(p.inventoryId) ? 'refreshing' : 'loading'] = true
        invState.error = null
      })
      try {
        const res = await fetchBitjita(`/claims/${claimEntityId}/inventories`)
        seedCatalog(res)
        const claimMeta = { entityId: claimEntityId, name: claimPinned[0]?.claimName ?? null, regionId: claimPinned[0]?.regionId ?? null }
        const invs = (res.buildings ?? []).map((building) => normalizeClaimBuilding(building, claimMeta))
        claimPinned.forEach((pinItem) => {
          const inv = invs.find((i) => i.entityId === pinItem.inventoryId)
          const invState = ensureInventoryState(pinItem.inventoryId)
          invState.loading = false
          invState.refreshing = false
          invState.error = inv ? null : t('dashboard.inventoryNotFound')
          if (inv) {
            invState.inventory = inv
            invState.pockets = inv.pockets ?? []
            invState.updatedAt = Date.now()
          }
        })
        await prefetch(invs.map((inv) => ({ ...inv, pockets: (inv.pockets ?? []).slice(0, 200) })))
      } catch (e) {
        claimPinned.forEach((p) => {
          const invState = ensureInventoryState(p.inventoryId)
          invState.loading = false
          invState.refreshing = false
          invState.error = e.message
        })
      }
    }
    let pollRun = null
    let lastLoadAllAt = 0
    function poll() {
      if (pollRun) return pollRun
      pollRun = loadAll().finally(() => { pollRun = null })
      return pollRun
    }
    async function loadAll({ fresh = false } = {}) {
      lastLoadAllAt = Date.now()
      const playerIds = [...new Set(pinned.value.filter((p) => sourceType(p) === 'player' && !isMarketBlock(p) && !isCharacterBlock(p) && !isMapBlock(p)).map((p) => p.playerEntityId))]
      const marketPlayerIds = [...new Set(pinned.value.filter((p) => isMarketBlock(p)).map((p) => p.playerEntityId))]
      const characterPlayerIds = [...new Set(pinned.value.filter((p) => isCharacterBlock(p)).map((p) => p.playerEntityId))]
      const skillPlayerIds = [...new Set(pinned.value.filter((p) => isSkillBlock(p)).map((p) => p.playerEntityId))]
      const claimIds = [...new Set(pinned.value.filter((p) => sourceType(p) === 'claim').map((p) => p.claimEntityId))]
      await Promise.all([
        ...playerIds.map(loadForPlayer),
        ...claimIds.map(loadForClaim),
        ...marketPlayerIds.map(loadMarketForPlayer),
        ...characterPlayerIds.map(loadCharacterStats),
        ...skillPlayerIds.map((id) => loadSkillProgress(id, { fresh })),
      ])
    }
    function refreshAll() {
      return loadAll({ fresh: true })
    }

    function inventorySlots(inventoryId) { return state[inventoryId]?.pockets ?? [] }
    function inventoryMeta(inventoryId) { return state[inventoryId]?.inventory ?? null }
    function hasContents(pocket) {
      const contents = pocket?.contents
      return Boolean(contents && (contents.itemId != null || contents.item_id != null))
    }
    function isCargoPocket(pocket) {
      const contents = pocket?.contents
      return contents?.itemType === 1 || contents?.item_type === 'cargo'
    }
    function itemSlots(inventoryId) { return inventorySlots(inventoryId).filter((p) => hasContents(p) && !isCargoPocket(p)) }
    function cargoSlots(inventoryId) { return inventorySlots(inventoryId).filter((p) => hasContents(p) && isCargoPocket(p)) }
    function fillSlots(slots, capacity) {
      if (!capacity) return slots
      const visibleSlots = slots.slice(0, capacity)
      if (capacity <= visibleSlots.length) return visibleSlots
      return [...visibleSlots, ...Array.from({ length: capacity - visibleSlots.length }, () => ({ contents: null }))]
    }
    function itemCapacity(inventoryId) {
      const meta = inventoryMeta(inventoryId)
      const name = `${meta?.inventoryName ?? ''} ${meta?.buildingName ?? ''}`
      const capacity = Number(meta?.cargoIndex)
      if (Number.isFinite(capacity) && capacity > 0) return capacity
      return buildingItemCapacity(name)
    }
    function cargoCapacity(inventoryId) {
      const meta = inventoryMeta(inventoryId)
      const name = `${meta?.inventoryName ?? ''} ${meta?.buildingName ?? ''}`
      if (/personal cache \(iii\)/i.test(name)) return 24
      if (/personal cache \(ii\)/i.test(name)) return 18
      if (/personal cache(?: \(i\))?$/i.test(name)) return 10
      return buildingCargoCapacity(name)
    }
    function tierIndex(name) {
      const tiers = ['rough', 'simple', 'sturdy', 'fine', 'exquisite', 'peerless', 'ornate', 'pristine', 'magnificent', 'flawless']
      const lower = name.toLowerCase()
      return tiers.findIndex((tier) => lower.includes(tier))
    }
    function tierValue(name, values) {
      const index = tierIndex(name)
      return index >= 0 ? values[index] : 0
    }
    function buildingItemCapacity(name) {
      if (/ancient bank|town bank/i.test(name)) return 64
      if (/lost items chest/i.test(name)) return 32
      if (/gothic item storage/i.test(name)) return 72
      if (/skillful leathercraft item storage/i.test(name)) return 60
      if (/cozy cottage item storage/i.test(name)) return 48
      if (/talented woodcraft item storage/i.test(name)) return 36
      if (/logcraft item storage/i.test(name)) return 30
      if (/wicker item storage/i.test(name)) return 24
      if (/item recovery pickup/i.test(name)) return 14
      if (/large (stone )?chest/i.test(name)) return tierValue(name, [18, 24, 30, 36, 42, 48, 54, 60, 66, 72])
      if (/(stone )?chest/i.test(name)) return tierValue(name, [9, 12, 15, 18, 21, 24, 27, 30, 33, 36])
      if (/barter (counter|stall)/i.test(name)) return tierValue(name, [20, 0, 40, 0, 60, 0, 0, 0, 0, 0])
      return 0
    }
    function buildingCargoCapacity(name) {
      if (/ancient bank|town bank|lost items chest/i.test(name)) return 16
      if (/gothic cargo bin/i.test(name)) return 60
      if (/skillful leathercraft cargo bin/i.test(name)) return 48
      if (/cozy cottage cargo bin/i.test(name)) return 36
      if (/talented woodcraft cargo bin/i.test(name)) return 24
      if (/logcraft cargo bin/i.test(name)) return 18
      if (/wicker cargo bin/i.test(name)) return 12
      if (/wicker basket/i.test(name)) return 6
      if (/small stockpile|supplies stockpile/i.test(name)) return 4
      if (/carpentry stockpile/i.test(name)) return tierValue(name, [2, 4, 6, 8, 10, 12, 0, 0, 0, 0])
      if (/fishing stockpile/i.test(name)) return tierValue(name, [1, 2, 3, 4, 5, 6, 0, 0, 0, 0])
      if (/hunting stockpile/i.test(name)) return tierValue(name, [1, 2, 6, 9, 10, 11, 0, 0, 0, 0])
      if (/quarry stockpile/i.test(name)) return tierValue(name, [3, 5, 8, 11, 13, 16, 0, 0, 0, 0])
      if (/stockpile/i.test(name)) return tierValue(name, [6, 12, 18, 24, 30, 36, 42, 48, 54, 60])
      if (/barter (counter|stall)/i.test(name)) return tierValue(name, [6, 0, 12, 0, 18, 0, 0, 0, 0, 0])
      return 0
    }
    function displayItemSlots(inventoryId) { return fillSlots(itemSlots(inventoryId), itemCapacity(inventoryId)) }
    function displayCargoSlots(inventoryId) { return fillSlots(cargoSlots(inventoryId), cargoCapacity(inventoryId)) }
    function displayInventorySlots(inventoryId) {
      return [
        ...displayItemSlots(inventoryId).map((pocket) => ({ kind: 'item', pocket })),
        ...displayCargoSlots(inventoryId).map((pocket) => ({ kind: 'cargo', pocket })),
      ]
    }
    function dashboardCardSpan(inventoryId) {
      const pinItem = pinned.value.find((p) => p.inventoryId === inventoryId)
      if (isMapBlock(pinItem)) return mapTileSize(inventoryId) === 'small' ? 1 : 2
      if (isCharacterBlock(pinItem)) return 1
      if (isSkillBlock(pinItem)) return skillTileSize(inventoryId) === 'large' ? 2 : 1
      if (isPlannerBlock(pinItem)) return plannerTileSize(inventoryId) === 'large' ? 2 : 1
      if (isMarketBlock(pinItem)) return marketTileSize(inventoryId) === 'small' ? 1 : 2
      const itemCount = displayItemSlots(inventoryId).length
      const cargoCount = displayCargoSlots(inventoryId).length
      if (cargoCount > 0) return 2
      if (itemCount > 64) return 2
      return 1
    }
    function dashboardCardStyle(inventoryId) {
      const pos = layoutInfo.value.positions[inventoryId]
      return pos ? gridArea(pos.x, pos.y, pos.span) : {}
    }
    function dashboardCardClass(inventoryId) {
      return {
        'inv-card--large': dashboardCardSpan(inventoryId) === 2,
        'inv-card--small': dashboardCardSpan(inventoryId) === 1,
        'inv-card--dragging': drag.id === inventoryId,
      }
    }
    function optimalGridColumns(slots, maxCols = 16) {
      const units = slots.reduce((sum, slot) => sum + (slot.kind === 'cargo' ? 2 : 1), 0)
      if (units <= 0) return 1
      const minCols = 4
      const desiredCols = Math.max(minCols, Math.min(maxCols, Math.round(Math.sqrt(units) * 1.8)))
      const divisors = []
      for (let cols = minCols; cols <= maxCols; cols += 1) if (units % cols === 0) divisors.push(cols)
      if (divisors.length === 0) return desiredCols
      return divisors.reduce((best, cols) => {
        const bestDiff = Math.abs(best - desiredCols)
        const diff = Math.abs(cols - desiredCols)
        return diff < bestDiff || (diff === bestDiff && cols > best) ? cols : best
      }, divisors[0])
    }
    function inventoryGridStyle(inventoryId) {
      const maxCols = dashboardCardSpan(inventoryId) === 2 ? 16 : 8
      return { '--inventory-cols': optimalGridColumns(displayInventorySlots(inventoryId), maxCols) }
    }
    function hasInventoryGrid(inventoryId) {
      return itemCapacity(inventoryId) > 0 || cargoCapacity(inventoryId) > 0 || inventorySlots(inventoryId).length > 0
    }
    function blockIsEmpty(item) {
      if (isMapBlock(item)) return false
      if (isPlannerBlock(item)) return !item.plannerTarget?.id
      if (isMarketBlock(item)) return false
      if (isCharacterBlock(item)) return !state[item.inventoryId]?.characterStats
      // Rankings can take BitJita close to a minute; level and XP come from live data and history.
      if (isSkillBlock(item)) return !levelTable.value || (!state[item.inventoryId]?.skillRanking && !skillXp(item))
      return !hasInventoryGrid(item.inventoryId)
    }
    function formatBig(value) {
      return value == null ? '—' : value.toString().replace(/\B(?=(\d{3})+(?!\d))/g, '\u202f')
    }
    function marketDeviationCount(inventoryId, direction) {
      const st = state[inventoryId]
      return playerOrderRows(st?.marketOrders, 'sell', st?.marketBulkMaps).filter((row) => {
        if (row.diffPct == null) return false
        return direction === 'above' ? row.diffPct > FLAG_PCT : row.diffPct < -FLAG_PCT
      }).length
    }
    function itemMeta(contents) { return resolveItem(contents) }
    function plannerItemMeta(itemRef) {
      if (!itemRef?.id) return { name: '?', type: 0 }
      return resolveItem({ itemId: itemRef.id, itemType: itemRef.itemType })
    }
    function quantityDisplay(quantity) { return formatNumber(quantity ?? 0) }
    function stripOwner(name, owner) {
      if (!name || !owner) return name
      const prefix = `${owner}'s `
      return name.toLowerCase().startsWith(prefix.toLowerCase()) ? name.slice(prefix.length) : name
    }
    function blockHeading(item) {
      const region = item.regionId ? `R${item.regionId}` : null
      if (isMarketBlock(item)) return { kicker: item.playerName ?? '', title: t('blockTypes.market.title') }
      if (isCharacterBlock(item)) return { kicker: item.playerName ?? '', title: t('blockTypes.character.title') }
      if (isSkillBlock(item)) return { kicker: item.playerName ?? '', title: item.skillName ?? t('dashboard.skill') }
      if (isMapBlock(item)) return { kicker: '', title: t('blockTypes.map.title') }
      if (isPlannerBlock(item)) {
        const target = plannerTarget(item.inventoryId)
        return { kicker: t('blockTypes.planner.title'), title: target ? plannerItemMeta(target).name : t('dashboard.noItemChosen') }
      }
      if (sourceType(item) === 'claim') {
        return { kicker: [item.claimName, region].filter(Boolean).join(' · '), title: item.inventoryName ?? t('common.inventory') }
      }
      return {
        kicker: [item.playerName, item.claimName, region].filter(Boolean).join(' · '),
        title: stripOwner(item.inventoryName, item.playerName) || t('common.inventory'),
      }
    }
    function blockSizeControl(item) {
      if (isMarketBlock(item)) return [marketTileSize, setMarketTileSize]
      if (isPlannerBlock(item)) return [plannerTileSize, setPlannerTileSize]
      if (isSkillBlock(item)) return [skillTileSize, setSkillTileSize]
      if (isMapBlock(item)) return [mapTileSize, setMapTileSize]
      return null
    }
    function blockCanResize(item) {
      return blockSizeControl(item) !== null
    }
    function blockIsLarge(item) {
      const control = blockSizeControl(item)
      return control ? control[0](item.inventoryId) === 'large' : false
    }
    function toggleBlockSize(item) {
      const control = blockSizeControl(item)
      if (!control) return
      const [getSize, setSize] = control
      setSize(item.inventoryId, getSize(item.inventoryId) === 'large' ? 'small' : 'large')
    }
    function vitalsRows(inventoryId) {
      const bars = characterBars(inventoryId)
      if (!bars) return []
      const noFood = characterBuffsLoaded(inventoryId) && !characterHasFoodBuff(inventoryId)
      return [
        { id: 'hp', label: t('dashboard.health'), color: 'var(--hp)', bar: bars.hp },
        { id: 'stamina', label: t('dashboard.stamina'), color: 'var(--stamina)', bar: bars.stamina },
        { id: 'satiation', label: t('dashboard.satiation'), color: 'var(--food)', bar: bars.satiation, warn: noFood ? t('dashboard.noFood') : '' },
      ].map((row) => ({ ...row, pct: vitalsBarPercent(row.bar), text: formatVitalsValue(row.bar) }))
    }
    function resetWizard() {
      clearTimeout(wizardSearchTimer)
      clearTimeout(wizardPlannerSearchTimer)
      wizardSearchGen = 0
      wizardPlannerSearchGen = 0
      Object.assign(wizard, {
        step: 'type',
        blockType: 'inventory',
        sourceType: 'player',
        query: '',
        searching: false,
        error: null,
        results: [],
        selectedEntity: null,
        inventories: [],
        loadingInventories: false,
        editInventoryId: null,
        plannerQuery: '',
        plannerSearching: false,
        plannerResults: [],
        plannerTarget: null,
        plannerRecipeOptions: [],
        plannerSelectedRecipeId: null,
        plannerCraftRecipes: [],
        plannerQuantity: 1,
        plannerLoadingRecipes: false,
        skillOptions: [],
        loadingSkills: false,
      })
    }
    function openAddWizard() { resetWizard(); wizard.open = true }
    function closeAddWizard() { wizard.open = false; resetWizard() }
    function chooseBlockType(type) {
      wizard.blockType = type
      wizard.sourceType = 'player'
      wizard.query = ''
      wizard.error = null
      wizard.results = []
      wizard.inventories = []
      if (type === 'map') {
        addWizardMap()
        return
      }
      if (type === 'planner') {
        wizard.step = 'planner'
        resetWizardPlannerDraft()
        return
      }
      wizard.step = 'search'
    }
    function chooseSourceType(type) {
      wizardSearchInput.value?.focus()
      if (wizard.sourceType === type) return
      wizard.sourceType = type
      wizard.error = null
      wizard.results = []
      wizard.inventories = []
      if (wizard.query.trim()) searchWizard()
    }
    function wizardBack() {
      if (wizard.editInventoryId && wizard.step === 'planner') {
        closeAddWizard()
        return
      }
      if (wizard.blockType === 'market' || wizard.blockType === 'character' || wizard.blockType === 'skill') {
        wizard.step = wizard.step === 'search' ? 'type' : 'search'
        return
      }
      if (wizard.blockType === 'planner' && wizard.step === 'planner') {
        wizard.step = 'type'
        resetWizardPlannerDraft()
        return
      }
      const index = wizardSteps.value.indexOf(wizard.step)
      if (index > 0) wizard.step = wizardSteps.value[index - 1]
    }
    async function searchWizard() {
      const q = wizard.query.trim()
      if (!q) {
        wizard.results = []
        wizard.error = null
        wizard.searching = false
        return
      }
      const gen = ++wizardSearchGen
      wizard.searching = true
      wizard.error = null
      wizard.results = []
      wizard.inventories = []
      try {
        const searchPlayers = wizard.blockType === 'market' || wizard.blockType === 'character' || wizard.blockType === 'skill' || wizard.sourceType === 'player'
        const res = await fetchBitjita(searchPlayers ? '/players' : '/claims', { q })
        if (gen !== wizardSearchGen) return
        wizard.results = searchPlayers ? (res.players ?? []) : (res.claims ?? [])
        if (wizard.results.length === 0) wizard.error = searchPlayers ? t('wizard.playerNotFound') : t('wizard.claimNotFound')
      } catch (e) {
        if (gen !== wizardSearchGen) return
        wizard.error = e.message
      } finally {
        if (gen === wizardSearchGen) wizard.searching = false
      }
    }

    watch(() => wizard.query, () => {
      if (!wizard.open || wizard.step !== 'search') return
      clearTimeout(wizardSearchTimer)
      const q = wizard.query.trim()
      if (!q) {
        wizard.results = []
        wizard.error = null
        wizard.searching = false
        return
      }
      wizardSearchTimer = setTimeout(() => searchWizard(), SEARCH_DEBOUNCE_MS)
    })
    async function selectWizardEntity(entity) {
      wizard.selectedEntity = entity
      wizard.error = null
      if (wizard.blockType === 'market') {
        addWizardMarket(entity)
        return
      }
      if (wizard.blockType === 'character') {
        addWizardCharacter(entity)
        return
      }
      if (wizard.blockType === 'skill') {
        loadWizardSkills(entity)
        return
      }
      wizard.loadingInventories = true
      wizard.inventories = []
      wizard.step = 'inventory'
      try {
        wizard.inventories = wizard.sourceType === 'player'
          ? await loadWizardPlayerInventories(entity)
          : await loadWizardClaimInventories(entity)
        await prefetch(wizard.inventories)
      } catch (e) {
        wizard.error = e.message
      } finally {
        wizard.loadingInventories = false
      }
    }
    async function loadWizardPlayerInventories(player) {
      return loadPlayerInventoriesCatalog(player.entityId)
    }
    async function loadWizardClaimInventories(claim) {
      const res = await fetchBitjita(`/claims/${claim.entityId}/inventories`)
      seedCatalog(res)
      return (res.buildings ?? [])
        .filter((building) => (building.inventory ?? []).length > 0 || isStorageBuilding(building))
        .map((building) => ({ ...normalizeClaimBuilding(building, claim), _isClaim: true }))
    }
    function isStorageBuilding(building) {
      const name = `${building.buildingName ?? ''} ${building.buildingNickname ?? ''}`
      return /bank|chest|stockpile|storage|cargo|cache|barter|basket/i.test(name)
    }
    function addWizardInventory(inv) {
      if (wizard.sourceType === 'player') {
        pinBlock({
          sourceType: 'player',
          inventoryId: inv.entityId,
          playerEntityId: wizard.selectedEntity.entityId,
          playerName: wizard.selectedEntity.username,
          inventoryName: inv.inventoryName,
          regionId: inv.regionId ?? null,
          claimName: inv.claimName ?? null,
          buildingName: inv.buildingName ?? null,
          houseName: inv.houseName ?? null,
          _isHousing: inv._isHousing ?? false,
          houseBuildingEntityId: inv.houseBuildingEntityId ?? null,
        })
      } else {
        pinBlock({
          sourceType: 'claim',
          inventoryId: inv.entityId,
          claimEntityId: wizard.selectedEntity.entityId,
          claimName: wizard.selectedEntity.name,
          regionId: wizard.selectedEntity.regionId ?? inv.regionId ?? null,
          inventoryName: inv.inventoryName,
        })
      }
      closeAddWizard()
      loadAll()
    }
    function addWizardMarket(player) {
      const blockId = `market:${player.entityId}`
      pinBlock({
        blockType: 'market',
        sourceType: 'market',
        inventoryId: blockId,
        playerEntityId: String(player.entityId),
        playerName: player.username,
        inventoryName: `${t('blockTypes.market.title')} · ${player.username}`,
        marketTileSize: 'large',
      })
      closeAddWizard()
      loadAll()
    }
    function addWizardCharacter(player) {
      const blockId = `character:${player.entityId}`
      pinBlock({
        blockType: 'character',
        sourceType: 'player',
        inventoryId: blockId,
        playerEntityId: String(player.entityId),
        playerName: player.username,
        inventoryName: `${t('blockTypes.character.title')} · ${player.username}`,
      })
      closeAddWizard()
      loadAll()
    }
    function addWizardMap() {
      const blockId = `map:${Date.now()}`
      pinBlock({
        blockType: 'map',
        sourceType: 'map',
        inventoryId: blockId,
        inventoryName: t('blockTypes.map.title'),
        mapState: {},
        mapTileSize: 'large',
      })
      closeAddWizard()
    }
    function isCharacterBlock(pinItem) {
      return blockType(pinItem) === 'character'
    }
    async function loadCharacterStats(playerEntityId) {
      const charPinned = pinned.value.filter((p) => isCharacterBlock(p) && String(p.playerEntityId) === String(playerEntityId))
      if (!charPinned.length) return
      charPinned.forEach((p) => {
        const st = ensureInventoryState(p.inventoryId)
        st.loading = !st.characterStats
        st.refreshing = Boolean(st.characterStats)
        st.error = null
      })
      try {
        const [statsData, buffsData] = await Promise.all([
          fetchBitjita(`/players/${playerEntityId}/stats`),
          fetchBitjita(`/players/${playerEntityId}/buffs`).catch(() => null),
        ])
        const stats = statsData?.stats ?? null
        if (!stats?.values) throw new Error(t('dashboard.noStats'))
        const buffs = buffsData?.buffs ?? null
        charPinned.forEach((p) => {
          const st = ensureInventoryState(p.inventoryId)
          st.loading = false
          st.refreshing = false
          st.characterStats = stats
          if (buffs) st.characterBuffs = buffs
          st.updatedAt = Date.now()
        })
      } catch (e) {
        charPinned.forEach((p) => {
          const st = ensureInventoryState(p.inventoryId)
          st.loading = false
          st.refreshing = false
          st.error = e.message
        })
      }
    }
    function characterBars(inventoryId) {
      const pinItem = pinned.value.find((p) => p.inventoryId === inventoryId)
      if (!pinItem) return null
      const st = state[inventoryId]
      if (!st?.characterStats?.values) return null
      return characterVitalsFromStats(st.characterStats, liveSnapshot(pinItem.playerEntityId))
    }
    function characterBuffsLoaded(inventoryId) {
      return Array.isArray(state[inventoryId]?.characterBuffs)
    }
    function characterActiveBuffs(inventoryId) {
      return activeBuffsFromPayload(state[inventoryId]?.characterBuffs)
    }
    function characterHasFoodBuff(inventoryId) {
      if (!characterBuffsLoaded(inventoryId)) return true
      return hasActiveFoodBuff(state[inventoryId].characterBuffs)
    }
    function isSkillBlock(pinItem) {
      return blockType(pinItem) === 'skill'
    }
    function skillBlockId(playerEntityId, skillId) {
      return `skill:${playerEntityId}:${skillId}`
    }
    function skillTileSize(inventoryId) {
      const block = pinned.value.find((p) => p.inventoryId === inventoryId)
      return block?.skillTileSize === 'large' ? 'large' : 'small'
    }
    function setSkillTileSize(inventoryId, size) {
      if (size !== 'small' && size !== 'large') return
      updatePinnedBlock(inventoryId, { skillTileSize: size })
    }
    function ensureLevelTable() {
      if (levelTable.value) return Promise.resolve(levelTable.value)
      if (!levelTablePromise) {
        levelTablePromise = fetchBitjita('/static/experience/levels.json')
          .then((rows) => {
            const table = parseLevelTable(rows)
            if (!table) throw new Error(t('dashboard.noLevelTable'))
            levelTable.value = table
            return table
          })
          .finally(() => { levelTablePromise = null })
      }
      return levelTablePromise
    }
    async function loadSkillProgress(playerEntityId, { fresh = false } = {}) {
      const skillPinned = pinned.value.filter((p) => isSkillBlock(p) && String(p.playerEntityId) === String(playerEntityId))
      if (!skillPinned.length) return
      skillPinned.forEach((p) => {
        const st = ensureInventoryState(p.inventoryId)
        st.loading = !st.skillRanking
        st.refreshing = Boolean(st.skillRanking)
        st.error = null
      })
      try {
        const [res] = await Promise.all([
          fetchBitjita(`/players/${playerEntityId}/skill-rankings`, {}, { fresh }),
          ensureLevelTable(),
          loadXpHistory(playerEntityId),
        ])
        const rankings = res?.rankings
        if (!rankings) throw new Error(t('dashboard.noSkills'))
        skillPinned.forEach((p) => {
          const st = ensureInventoryState(p.inventoryId)
          st.loading = false
          st.refreshing = false
          st.skillRanking = rankings[String(p.skillId)] ?? { xp: 0, rank: null, totalPlayers: null }
          st.updatedAt = Date.now()
        })
        await Promise.all(skillPinned.flatMap((p) => [loadSkillRankAbove(p, fresh), loadActiveSkillRank(p, fresh)]))
      } catch (e) {
        skillPinned.forEach((p) => {
          const st = ensureInventoryState(p.inventoryId)
          st.loading = false
          st.refreshing = false
          st.error = e.message
        })
      }
    }
    async function loadSkillRankAbove(pinItem, fresh = false) {
      const st = ensureInventoryState(pinItem.inventoryId)
      const rank = st.skillRanking?.rank
      if (!rank) {
        st.skillAbove = null
        return
      }
      try {
        st.skillAbove = await skillPlace(pinItem.skillId, rank > 1 ? rank - 1 : 2, null, { fresh })
      } catch {
      }
    }
    async function loadActiveSkillRank(pinItem, fresh = false) {
      const st = ensureInventoryState(pinItem.inventoryId)
      st.skillActiveLoading = !st.skillActiveRanking
      try {
        const player = { entityId: pinItem.playerEntityId, username: pinItem.playerName }
        const [rank, totalPlayers] = await Promise.all([
          activeSkillRank(pinItem.skillId, player, ACTIVE_DAYS, { fresh }),
          activeSkillPlayers(pinItem.skillId, ACTIVE_DAYS, { fresh }),
        ])
        const above = rank ? await skillPlace(pinItem.skillId, rank > 1 ? rank - 1 : 2, ACTIVE_DAYS, { fresh }).catch(() => null) : null
        st.skillActiveRanking = { rank, totalPlayers }
        st.skillActiveAbove = above
      } catch {
      } finally {
        st.skillActiveLoading = false
      }
    }
    function skillRankScope(pinItem) {
      return pinItem.skillRankScope === 'active' ? 'active' : 'all'
    }
    function skillActiveRanking(pinItem) {
      const st = state[pinItem.inventoryId]
      return {
        rank: st?.skillActiveRanking?.rank ?? null,
        totalPlayers: st?.skillActiveRanking?.totalPlayers ?? null,
        above: st?.skillActiveAbove ?? null,
        loading: Boolean(st?.skillActiveLoading) || !st?.skillActiveRanking,
      }
    }
    async function loadXpHistory(playerEntityId) {
      const id = String(playerEntityId)
      if (!xpHistory[id]) xpHistory[id] = { trackedSince: null, lastT: 0, loaded: false, bySkill: {} }
      const entry = xpHistory[id]
      try {
        const res = await fetchXpHistory(id, entry.lastT)
        entry.trackedSince = res.trackedSince || entry.trackedSince
        Object.entries(splitHistorySamples(res.samples)).forEach(([skill, points]) => {
          entry.bySkill[skill] = Object.freeze(mergeHistoryPoints(entry.bySkill[skill], points))
        })
        ;(res.samples ?? []).forEach((sample) => { entry.lastT = Math.max(entry.lastT, sample.t) })
        entry.loaded = true
      } catch {
      }
    }
    function recordLiveXp(playerEntityId) {
      const id = String(playerEntityId)
      const live = experienceByPlayer[id]
      const entry = xpHistory[id]
      if (!live?.xp || !entry?.loaded) return
      const t = Math.floor(Date.now() / 1000)
      pinned.value
        .filter((p) => isSkillBlock(p) && String(p.playerEntityId) === id)
        .forEach((p) => {
          const skill = String(p.skillId)
          const xp = live.xp[skill]
          if (!xp) return
          const next = appendLivePoint(entry.bySkill[skill], t, xp)
          if (next !== entry.bySkill[skill]) entry.bySkill[skill] = Object.freeze(next)
        })
    }
    function skillHistory(pinItem) {
      return xpHistory[String(pinItem.playerEntityId)]?.bySkill[String(pinItem.skillId)] ?? null
    }
    function skillTrackedSince(pinItem) {
      return xpHistory[String(pinItem.playerEntityId)]?.trackedSince ?? null
    }
    function skillXp(pinItem) {
      const skill = String(pinItem.skillId)
      const live = experienceByPlayer[String(pinItem.playerEntityId)]?.xp?.[skill] ?? 0
      const points = skillHistory(pinItem)
      const recorded = points?.length ? points[points.length - 1][1] : 0
      const ranked = state[pinItem.inventoryId]?.skillRanking?.xp ?? 0
      return Math.max(live, recorded, ranked)
    }
    async function loadWizardSkills(player) {
      wizard.step = 'skill'
      wizard.loadingSkills = true
      wizard.skillOptions = []
      try {
        const [skillsRes, rankingsRes, table] = await Promise.all([
          fetchBitjita('/skills'),
          fetchBitjita(`/players/${player.entityId}/skill-rankings`),
          ensureLevelTable(),
        ])
        const rankings = rankingsRes?.rankings ?? {}
        wizard.skillOptions = SKILL_CATEGORIES.flatMap((cat) => (skillsRes?.[cat.id] ?? []).map((skill) => {
          const ranking = rankings[String(skill.id)]
          return {
            id: skill.id,
            name: skill.name,
            title: skill.title,
            category: cat.id,
            xp: ranking?.xp ?? 0,
            rank: ranking?.rank ?? null,
            level: levelForXp(table, ranking?.xp ?? 0),
          }
        })).sort((a, b) => b.xp - a.xp)
        if (!wizard.skillOptions.length) wizard.error = t('wizard.noSkillsFound')
      } catch (e) {
        wizard.error = e.message
      } finally {
        wizard.loadingSkills = false
      }
    }
    function openEditSkillWizard(inventoryId) {
      const pin = pinned.value.find((p) => p.inventoryId === inventoryId)
      if (!pin || !isSkillBlock(pin)) return
      resetWizard()
      wizard.open = true
      wizard.blockType = 'skill'
      wizard.editInventoryId = inventoryId
      wizard.selectedEntity = { entityId: pin.playerEntityId, username: pin.playerName }
      loadWizardSkills(wizard.selectedEntity)
    }
    function wizardSkillTitle(skill) {
      const id = skillBlockId(wizard.selectedEntity?.entityId, skill.id)
      if (id === wizard.editInventoryId) return t('wizard.inBlockNow')
      return isPinned(id) ? t('wizard.alreadyPinned') : ''
    }
    function replaceWizardSkill(skill) {
      const oldId = wizard.editInventoryId
      const pin = pinned.value.find((p) => p.inventoryId === oldId)
      const newId = skillBlockId(pin?.playerEntityId, skill.id)
      if (!pin || isPinned(newId)) return
      updatePinnedBlock(oldId, {
        inventoryId: newId,
        skillId: skill.id,
        skillName: skill.name,
        skillTitle: skill.title,
        inventoryName: `${skill.name} · ${pin.playerName}`,
      })
      delete state[oldId]
      closeAddWizard()
      loadSkillProgress(pin.playerEntityId)
    }
    function addWizardSkill(skill) {
      if (wizard.editInventoryId) {
        replaceWizardSkill(skill)
        return
      }
      const player = wizard.selectedEntity
      if (!player) return
      pinBlock({
        blockType: 'skill',
        sourceType: 'skill',
        inventoryId: skillBlockId(player.entityId, skill.id),
        playerEntityId: String(player.entityId),
        playerName: player.username,
        skillId: skill.id,
        skillName: skill.name,
        skillTitle: skill.title,
        inventoryName: `${skill.name} · ${player.username}`,
        skillTileSize: 'small',
      })
      closeAddWizard()
      loadAll()
    }
    function syncCharacterLiveSubscriptions() {
      const playerIds = pinned.value.filter((p) => isCharacterBlock(p)).map((p) => p.playerEntityId)
      syncPlayerSubscriptions(playerIds)
    }
    const { reset: resetStaminaAlerts } = useStaminaAlerts(() => {
      pinned.value
        .filter((p) => isCharacterBlock(p))
        .forEach((p) => staminaByPlayer[String(p.playerEntityId)])
      return pinned.value
        .filter((p) => isCharacterBlock(p))
        .map((p) => {
          const bars = characterBars(p.inventoryId)
          if (!bars?.stamina) return null
          return {
            playerId: p.playerEntityId,
            current: bars.stamina.current,
            max: bars.stamina.max,
          }
        })
        .filter(Boolean)
    })
    function wizardResultTitle(result) { return wizard.sourceType === 'player' ? result.username : result.name }
    function wizardResultMeta(result) {
      if (wizard.blockType === 'market' || wizard.blockType === 'character' || wizard.blockType === 'skill' || wizard.sourceType === 'player') return result.signedIn ? t('common.online') : t('common.offline')
      return [result.regionName ? `R${result.regionId} · ${result.regionName}` : (result.regionId ? `R${result.regionId}` : null), result.tier ? `T${result.tier}` : null].filter(Boolean).join(' · ')
    }
    function inventoryLabel(inv) {
      return `${inv.inventoryName ?? ''} ${inv.buildingName ?? ''} ${inv.houseName ?? ''}`
    }
    function isBankInventory(inv) {
      return /bank/i.test(inventoryLabel(inv))
    }
    function isRecoveryInventory(inv) {
      return /recovery chest|lost items/i.test(inv.inventoryName ?? '')
    }
    function isBarterInventory(inv) {
      return /barter/i.test(inventoryLabel(inv))
    }
    function wizardInventoryCategory(inv) {
      if (inv._isHousing) return 'house'
      if (inv._isClaim) {
        if (isBankInventory(inv)) return 'bank'
        if (isBarterInventory(inv)) return 'trade'
        return 'claim'
      }
      if (isBankInventory(inv)) return 'bank'
      if (isRecoveryInventory(inv)) return 'recovery'
      return 'inventory'
    }
    function wizardInventoryTitle(inv) {
      const name = inv.inventoryName || inv.buildingNickname || inv.buildingName || inv.houseName || t('common.inventory')
      return stripOwner(name, wizard.selectedEntity?.username)
    }
    function wizardPreviewPockets(inv) {
      const pockets = inv.pockets ?? inv.inventory ?? []
      return pockets.map(normalizePocket).filter((p) => hasContents(p))
    }
    function wizardPreviewQty(contents) {
      const n = Number(contents?.quantity ?? 0)
      if (!Number.isFinite(n)) return '0'
      if (n >= 10000) return `${Math.round(n / 1000)}k`
      return formatNumber(n)
    }
    function wizardInventoryMeta(inv) {
      const parts = []
      if (inv.regionId) parts.push(`R${inv.regionId}`)
      if (inv.claimName) parts.push(inv.claimName)
      if (inv.houseName) parts.push(inv.houseName)
      if (inv.buildingName && inv.buildingName !== inv.inventoryName) parts.push(inv.buildingName)
      if (!wizardPreviewPockets(inv).length) parts.push(t('common.empty'))
      return parts.join(' · ')
    }

    let timer = null
    let resizeObserver = null
    function updateDashboardCols() {
      const width = rootRef.value?.clientWidth
      if (!width) return
      dashboardCols.value = Math.max(1, Math.floor((width + TILE_GAP_PX) / (TILE_WIDTH_PX + TILE_GAP_PX)))
    }
    watch(
      () => pinned.value.filter((p) => isCharacterBlock(p)).map((p) => String(p.playerEntityId)).join(','),
      () => syncCharacterLiveSubscriptions(),
      { immediate: true },
    )
    watch(
      () => [...new Set(pinned.value.filter((p) => isSkillBlock(p)).map((p) => String(p.playerEntityId)))].join(','),
      (ids) => syncExperienceSubscriptions(ids ? ids.split(',') : []),
      { immediate: true },
    )
    watch(
      () => Object.entries(experienceByPlayer).map(([id, live]) => `${id}:${live?.updatedAt ?? 0}`).join(','),
      () => Object.keys(experienceByPlayer).forEach(recordLiveXp),
    )
    function onVisibilityChange() {
      if (document.visibilityState === 'visible' && Date.now() - lastLoadAllAt > 15000) poll()
    }
    function onKeydown(event) {
      if (event.key === 'Escape' && wizard.open) closeAddWizard()
    }
    onMounted(() => {
      window.addEventListener('keydown', onKeydown)
      document.addEventListener('visibilitychange', onVisibilityChange)
      updateDashboardCols()
      resizeObserver = new ResizeObserver(updateDashboardCols)
      if (rootRef.value) resizeObserver.observe(rootRef.value)
      loadAll()
      timer = setInterval(poll, 30000)
      initPlannerBlocks()
    })
    onUnmounted(() => {
      window.removeEventListener('keydown', onKeydown)
      document.removeEventListener('visibilitychange', onVisibilityChange)
      clearInterval(timer)
      clearTimeout(wizardSearchTimer)
      clearTimeout(wizardPlannerSearchTimer)
      resizeObserver?.disconnect()
      syncPlayerSubscriptions([])
      syncExperienceSubscriptions([])
      resetStaminaAlerts()
    })

    return {
      FLAG_PCT, formatNumber, BLOCK_TYPES, WIZARD_PREVIEW_MAX,
      pinned, isPinned, unpin, state, drag, dropCells, gridArea, onTilePointerDown, layoutInfo, rootRef, wizard, wizardTitle, wizardKicker, wizardSearchesPlayers,
      gridRef, wizardSearchInput, dashboardCols, dashboardUpdatedLabel, dashboardRefreshing,
      displayInventorySlots, dashboardCardClass, dashboardCardStyle,
      inventoryGridStyle, hasInventoryGrid, hasContents, itemMeta, quantityDisplay,
      blockType, blockHeading, blockCanResize, blockIsLarge, toggleBlockSize, vitalsRows,
      isCharacterBlock, isMarketBlock, isPlannerBlock, isMapBlock,
      mapTileSize, setMapTileSize, marketTileSize, setMarketTileSize,
      marketCollectStats, marketDeviationCount, loadMarketForPlayer,
      formatBig, formatXpShort, blockIsEmpty,
      plannerTarget, plannerQuantity, plannerBlockError,
      plannerPlan, plannerTileSize, setPlannerTileSize, setPlannerRecipeOverride,
      setPlannerQuantity, togglePlannerDone,
      plannerLoading, plannerItemMeta,
      openEditPlannerWizard, wizardPlannerCanConfirm, confirmWizardPlanner,
      setWizardPlannerQuery, selectWizardPlannerTarget, plannerRecipeOptionTitle, plannerRecipeOptionMeta,
      characterBars, characterBuffsLoaded, characterActiveBuffs, characterHasFoodBuff,
      isSkillBlock, skillBlockId, skillTileSize, setSkillTileSize, levelTable, skillGlyph,
      skillXp, skillHistory, skillTrackedSince, updatePinnedBlock,
      skillRankScope, skillActiveRanking, ACTIVE_DAYS,
      wizardSkillGroups, addWizardSkill, openEditSkillWizard, wizardSkillTitle,
      formatVitalsValue, vitalsBarPercent,
      loadAll, refreshAll,
      openAddWizard, closeAddWizard, chooseBlockType, chooseSourceType, searchWizard,
      selectWizardEntity, addWizardInventory, wizardBack, wizardResultTitle, wizardResultMeta,
      wizardInventoryGroups, wizardInventoryTitle, wizardInventoryMeta, wizardPreviewPockets, wizardPreviewQty,
    }
  },
}
</script>

<style scoped>
.pinned-dashboard {
  --dashboard-cell: min(380px, calc(100vw - 32px));
  --dashboard-row: 446px;
  --dash-slot: 38px;
  --dash-slot-height: 46px;
  --dash-gap: 4px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-width: 0;
}

.dash-refresh { width: 30px; height: 30px; }
.dash-refresh--busy :deep(.app-icon) { animation: dash-spin 1s linear infinite; }
@keyframes dash-spin { to { transform: rotate(360deg); } }

.dash-empty { max-width: 520px; margin: 48px auto 0; }
.dash-empty-body { align-items: center; gap: 16px; padding: 18px 24px 24px; text-align: center; }
.dash-empty-body p { margin: 0; color: var(--text-soft); line-height: 1.5; }

.dashboard-grid {
  display: grid;
  grid-template-columns: repeat(var(--dashboard-cols), var(--dashboard-cell));
  grid-auto-rows: var(--dashboard-row);
  gap: 14px;
  align-items: stretch;
  width: calc((var(--dashboard-cell) * var(--dashboard-cols)) + (14px * (var(--dashboard-cols) - 1)));
  margin: 0 auto;
}
.dashboard-cell,
.drop-preview {
  min-width: 0;
  min-height: 0;
  border-radius: 4px;
  pointer-events: none;
}
.dashboard-cell { border: 1px dashed var(--frame-soft); background: rgba(233, 223, 196, 0.02); }
.drop-preview { z-index: 3; border: 2px solid var(--cream); background: rgba(233, 223, 196, 0.1); box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.5), 0 0 24px rgba(233, 223, 196, 0.15) inset; }
.drop-preview--blocked { border-color: var(--danger); background: rgba(236, 106, 111, 0.1); box-shadow: none; }
.drop-preview--swap { border-style: dashed; background: rgba(233, 223, 196, 0.05); box-shadow: none; }

.dash-card {
  z-index: 1;
  height: 100%;
  overflow: hidden;
  transition: opacity 0.15s;
}
.inv-card--dragging { opacity: 0.3; }
.dash-card-head { cursor: grab; user-select: none; touch-action: none; }
.tile-ghost {
  opacity: 0.94;
  transform-origin: 50% 20%;
  box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.55), 0 26px 60px rgba(0, 0, 0, 0.6);
  cursor: grabbing;
}
.tile-ghost .dash-card-tools { display: none; }
.dash-card-tools,
.dash-card-flags {
  position: absolute;
  top: 8px;
  display: flex;
  align-items: center;
  gap: 2px;
}
.dash-card-tools { right: 8px; }
.dash-card-flags { left: 12px; gap: 5px; }
@media (hover: hover) {
  .dash-card .dash-card-tools { opacity: 0; transition: opacity 0.15s; }
  .dash-card:hover .dash-card-tools,
  .dash-card:focus-within .dash-card-tools { opacity: 1; }
}
.dash-flag {
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 11px;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
  background: rgba(20, 18, 30, 0.45);
}
.dash-flag--above { color: var(--danger); }
.dash-flag--below { color: var(--success); }
.dash-flag--collect { display: inline-flex; align-items: center; gap: 3px; color: var(--gold); }
.dash-card-body { overflow: hidden; }
.dash-card-body--map { padding: 6px 8px 8px; }
.dash-card-body--skill { padding-top: 6px; }

.inventory-grid {
  display: grid;
  grid-template-columns: repeat(var(--inventory-cols), var(--dash-slot));
  gap: var(--dash-gap);
  justify-content: center;
  align-content: start;
  min-height: 0;
  overflow: hidden;
}
.inventory-slot {
  position: relative;
  height: var(--dash-slot-height);
  display: flex;
  align-items: stretch;
  justify-content: stretch;
  border-radius: 6px;
}
.inventory-slot--item { width: var(--dash-slot); }
.inventory-slot--cargo { grid-column: span 2; }
.inventory-slot--empty {
  border: 2px solid var(--slot-edge);
  background: var(--slot);
  box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.35), inset 0 6px 10px rgba(0, 0, 0, 0.18);
}
.inventory-slot :deep(.item-card--mini) { width: 100%; }
.inventory-slot :deep(.item-card--mini .item-card__icon),
.inventory-slot--cargo :deep(.item-card--mini .item-card__icon) {
  width: 100%;
  height: 100%;
  flex-basis: 100%;
}
.slot-qty {
  position: absolute;
  right: 4px;
  bottom: 3px;
  color: #fff;
  font-size: 12px;
  font-weight: 800;
  line-height: 1;
  font-variant-numeric: tabular-nums;
  text-shadow: -1px -1px 0 #000, 1px -1px 0 #000, -1px 1px 0 #000, 1px 1px 0 #000;
  pointer-events: none;
}

.vitals { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 12px; padding-top: 4px; }
.vital { display: flex; flex-direction: column; gap: 5px; }
.vital-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.vital-label {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  font-family: var(--font-title);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--text-soft);
}
.vital-mark { width: 7px; height: 7px; transform: rotate(45deg); }
.vital-warn {
  padding: 1px 7px;
  border-radius: 3px;
  background: rgba(236, 106, 111, 0.16);
  color: var(--danger);
  font-size: 11px;
  font-weight: 800;
}
.vital-bar { height: 22px; }
.vital-bar .bc-bar-text { font-size: 13px; }
.buffs { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 6px; margin-top: 4px; }
.buff-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 4px; overflow-y: auto; }
.buff {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 8px;
  padding: 5px 9px;
  border: 1px solid var(--border);
  border-left: 3px solid var(--border-strong);
  border-radius: 4px;
  background: var(--surface-raised);
  font-size: 12px;
}
.buff--food { border-left-color: var(--hp); }
.buff--bad { border-left-color: var(--danger); }
.buff-name { min-width: 0; color: var(--text-soft); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.buff-time { flex-shrink: 0; font-size: 11px; color: var(--muted); font-variant-numeric: tabular-nums; }

.modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: rgba(8, 7, 14, 0.72);
  backdrop-filter: blur(3px);
}
.wizard { width: min(680px, 100%); max-height: min(760px, calc(100vh - 48px)); }
.wizard-tools { opacity: 1; }
.wizard-body { gap: 12px; min-height: 240px; overflow: auto; padding: 12px 18px 16px; }
.wizard-types { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
.wizard-type {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border: 2px solid var(--border-strong);
  border-radius: 6px;
  background: rgba(20, 18, 30, 0.25);
  color: var(--text);
  text-align: left;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}
.wizard-type:hover { border-color: var(--cream); background: rgba(233, 223, 196, 0.06); }
.wizard-type-icon {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 42px;
  height: 48px;
  border: 2px solid var(--cream);
  border-radius: 6px;
  background: linear-gradient(180deg, #2a6aa0, #1b4a78);
  color: var(--cream);
}
.wizard-type-text { min-width: 0; display: flex; flex-direction: column; gap: 3px; }
.wizard-type-title { font-size: 15px; font-weight: 800; color: var(--text); }
.wizard-type-desc { font-size: 12px; line-height: 1.35; color: var(--muted); }
.wizard-search-row { display: flex; align-items: center; gap: 10px; }
.wizard-source { flex-shrink: 0; height: 32px; }
.wizard-source .bc-seg-btn { padding: 0 12px; }
.wizard-search { flex: 1; }
.wizard-list, .wizard-groups { display: flex; flex-direction: column; gap: 4px; }
.wizard-groups { gap: 16px; }
.wizard-group { display: flex; flex-direction: column; gap: 6px; }
.wizard-row {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 38px;
  padding: 6px 10px;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--surface-raised);
  color: var(--text);
  text-align: left;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}
.wizard-row:hover:not(:disabled) { border-color: var(--cream); background: var(--surface-hover); }
.wizard-row:disabled { opacity: 0.45; cursor: default; }
.wizard-row-icon { color: var(--muted); }
.wizard-row-title { flex: 1; min-width: 0; font-size: 13px; font-weight: 800; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.wizard-row-meta { flex-shrink: 0; font-size: 12px; color: var(--muted); font-variant-numeric: tabular-nums; }
.wizard-online { color: var(--success); }
.wizard-row :deep(.item-card--mini .item-card__icon) { width: 24px; height: 28px; flex-basis: 24px; border-width: 1px; border-radius: 4px; }
.wizard-inv { flex-direction: column; align-items: stretch; gap: 7px; }
.wizard-inv-head { display: flex; align-items: baseline; gap: 10px; }
.wizard-inv-preview { display: flex; flex-wrap: wrap; align-items: center; gap: 3px; }
.wizard-preview-slot { position: relative; }
.wizard-preview-slot :deep(.item-card--mini .item-card__icon) { width: 24px; height: 28px; flex-basis: 24px; border-width: 1px; border-radius: 4px; }
.wizard-preview-qty {
  position: absolute;
  right: 1px;
  bottom: 0;
  color: #fff;
  font-size: 8px;
  font-weight: 800;
  line-height: 1;
  text-shadow: -1px -1px 0 #000, 1px -1px 0 #000, -1px 1px 0 #000, 1px 1px 0 #000;
  pointer-events: none;
}
.wizard-preview-more { padding-left: 4px; font-size: 11px; color: var(--muted); }
.wizard-skill-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 4px; }
.wizard-skill-glyph { width: 20px; text-align: center; font-size: 15px; }
.wizard-skill-level { font-size: 14px; font-weight: 800; color: var(--text); font-variant-numeric: tabular-nums; }
.wizard-skill-rank { width: 58px; text-align: right; }
.wizard-skill--current:disabled { opacity: 1; border-color: var(--gold); }
.wizard-planner-results { max-height: 200px; overflow: auto; }
.wizard-planner-target {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid var(--frame-soft);
  border-radius: 4px;
  background: rgba(20, 18, 30, 0.3);
}
.wizard-planner-head { display: flex; align-items: center; gap: 12px; }
.wizard-planner-head :deep(.item-card--mini .item-card__icon) { width: 38px; height: 46px; flex-basis: 38px; border-width: 2px; }
.wizard-planner-head .wizard-row-title { font-size: 15px; }
.wizard-qty { display: flex; align-items: center; gap: 8px; font-size: 12px; color: var(--muted); }
.wizard-qty-input { width: 84px; height: 30px; text-align: right; }
.wizard-recipes { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 6px; }
.wizard-recipe {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  padding: 7px 10px;
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  background: var(--surface-raised);
  color: var(--text);
  text-align: left;
  cursor: pointer;
}
.wizard-recipe:hover { border-color: var(--frame); }
.wizard-recipe.active { border-color: var(--cream); box-shadow: inset 0 0 0 1px var(--cream); }
.wizard-recipe-title { font-size: 12px; font-weight: 800; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.wizard-recipe-meta { font-size: 11px; color: var(--muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.wizard-foot {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 18px 16px;
  border-top: 1px solid var(--frame-soft);
}
.wizard-confirm { margin-left: auto; }

@media (max-width: 840px) {
  .wizard-types, .wizard-skill-grid { grid-template-columns: 1fr; }
}
</style>
