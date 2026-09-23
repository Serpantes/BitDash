<template>
  <div class="pinned-dashboard">
    <div class="dash-toolbar">
      <span class="dash-hint">{{ dashboardUpdatedLabel }}</span>
      <div class="dash-actions">
        <button class="refresh-btn" @click="loadAll" :disabled="pinned.length === 0">&#x21bb; Обновить</button>
        <button class="add-block-btn" @click="openAddWizard">+ Добавить блок</button>
      </div>
    </div>

    <div v-if="pinned.length === 0" class="card empty-state">
      <div class="empty-title">Нет закреплённых блоков</div>
      <div class="empty-hint">Нажми «Добавить блок», чтобы закрепить инвентарь, рынок, планировщик, персонажа или карту.</div>
    </div>

    <div
      v-else
      ref="gridRef"
      class="dashboard-grid"
      :class="{ 'dashboard-grid--dragging': Boolean(dragState.inventoryId) }"
      :style="{ '--dashboard-cols': dashboardCols }"
      @dragover.prevent
    >
      <div
        v-for="cell in layoutCells"
        :key="`${cell.x}-${cell.y}`"
        class="dashboard-cell"
        :class="dashboardCellClass(cell)"
        :style="dashboardCellStyle(cell)"
        @dragover.prevent="dragOverCell(cell)"
        @drop="dropOnCell($event, cell)"
      ></div>

      <div
        v-for="item in pinned"
        :key="item.inventoryId"
        class="card inv-card"
        :class="dashboardCardClass(item.inventoryId)"
        :style="dashboardCardStyle(item.inventoryId)"
        @dragend="endDrag"
      >
        <div
          class="inv-header"
          draggable="true"
          title="Перетащить блок"
          @dragstart="startDrag($event, item.inventoryId)"
        >
          <div class="inv-meta">
            <span class="inv-name">{{ item.inventoryName }}</span>
            <span v-if="item.regionId" class="inv-region">R{{ item.regionId }}</span>
            <span v-if="item.claimName" class="inv-claim">{{ item.claimName }}</span>
            <template v-if="isMarketBlock(item)">
              <span v-if="marketDeviationCount(item.inventoryId, 'above') > 0" class="market-dev-badge market-dev-badge--above">▲ {{ marketDeviationCount(item.inventoryId, 'above') }}</span>
              <span v-if="marketDeviationCount(item.inventoryId, 'below') > 0" class="market-dev-badge market-dev-badge--below">▼ {{ marketDeviationCount(item.inventoryId, 'below') }}</span>
            </template>
            <span v-else-if="!isCharacterBlock(item) && !isMapBlock(item) && !isPlannerBlock(item) && !isSkillBlock(item)" class="inv-player">{{ blockOwnerLabel(item) }}</span>
          </div>
          <div class="inv-header-actions">
            <div v-if="isMarketBlock(item)" class="tile-size-switch">
              <button
                class="tile-size-btn"
                :class="{ active: marketTileSize(item.inventoryId) === 'small' }"
                @click.stop="setMarketTileSize(item.inventoryId, 'small')"
              >
                Мал.
              </button>
              <button
                class="tile-size-btn"
                :class="{ active: marketTileSize(item.inventoryId) === 'large' }"
                @click.stop="setMarketTileSize(item.inventoryId, 'large')"
              >
                Больш.
              </button>
            </div>
            <div v-if="isPlannerBlock(item)" class="tile-size-switch">
              <button
                class="tile-size-btn"
                :class="{ active: plannerTileSize(item.inventoryId) === 'small' }"
                @click.stop="setPlannerTileSize(item.inventoryId, 'small')"
              >
                Мал.
              </button>
              <button
                class="tile-size-btn"
                :class="{ active: plannerTileSize(item.inventoryId) === 'large' }"
                @click.stop="setPlannerTileSize(item.inventoryId, 'large')"
              >
                Больш.
              </button>
            </div>
            <div v-if="isSkillBlock(item)" class="tile-size-switch">
              <button
                class="tile-size-btn"
                :class="{ active: skillTileSize(item.inventoryId) === 'small' }"
                @click.stop="setSkillTileSize(item.inventoryId, 'small')"
              >
                Мал.
              </button>
              <button
                class="tile-size-btn"
                :class="{ active: skillTileSize(item.inventoryId) === 'large' }"
                @click.stop="setSkillTileSize(item.inventoryId, 'large')"
              >
                Больш.
              </button>
            </div>
            <div v-if="isMapBlock(item)" class="tile-size-switch">
              <button
                class="tile-size-btn"
                :class="{ active: mapTileSize(item.inventoryId) === 'small' }"
                @click.stop="setMapTileSize(item.inventoryId, 'small')"
              >
                Мини
              </button>
              <button
                class="tile-size-btn"
                :class="{ active: mapTileSize(item.inventoryId) === 'large' }"
                @click.stop="setMapTileSize(item.inventoryId, 'large')"
              >
                Больш.
              </button>
            </div>
            <button
              v-if="isPlannerBlock(item)"
              class="edit-block-btn"
              type="button"
              title="Изменить"
              @click.stop="openEditPlannerWizard(item.inventoryId)"
            >✎</button>
            <button class="unpin-btn" @click="unpin(item.inventoryId)" title="Открепить">✕</button>
          </div>
        </div>

        <div v-if="state[item.inventoryId]?.loading && blockIsEmpty(item)" class="placeholder">Загрузка...</div>
        <div v-else-if="state[item.inventoryId]?.error && blockIsEmpty(item)" class="error-msg">{{ state[item.inventoryId].error }}</div>
        <template v-else-if="isMapBlock(item)">
          <div class="map-block" :class="{ 'map-block--small': mapTileSize(item.inventoryId) === 'small' }">
            <iframe
              :ref="(el) => setMapIframeRef(item.inventoryId, el)"
              class="map-iframe"
              :class="{ 'map-iframe--small': mapTileSize(item.inventoryId) === 'small' }"
              :src="mapBlockUrl(item)"
              title="BitCraft Map"
              loading="lazy"
              referrerpolicy="no-referrer"
              allowfullscreen
              @load="onMapIframeLoad(item.inventoryId)"
            />
          </div>
        </template>
        <template v-else-if="isCharacterBlock(item)">
          <div v-if="!characterBars(item.inventoryId)" class="empty-inv">нет данных</div>
          <div
            v-else
            class="char-stats"
            :class="{ 'char-stats--no-food': characterBuffsLoaded(item.inventoryId) && !characterHasFoodBuff(item.inventoryId) }"
          >
            <div class="char-bar-group">
              <div class="char-bar-header">
                <span class="char-bar-label char-bar-label--hp">❤ Здоровье</span>
                <span class="char-bar-value">{{ formatVitalsValue(characterBars(item.inventoryId).hp) }}</span>
              </div>
              <div class="char-bar-track">
                <div class="char-bar-fill char-bar-fill--hp" :style="{ width: `${vitalsBarPercent(characterBars(item.inventoryId).hp)}%` }"></div>
              </div>
            </div>
            <div class="char-bar-group">
              <div class="char-bar-header">
                <span class="char-bar-label char-bar-label--stamina">⚡ Стамина</span>
                <span class="char-bar-value">{{ formatVitalsValue(characterBars(item.inventoryId).stamina) }}</span>
              </div>
              <div class="char-bar-track">
                <div class="char-bar-fill char-bar-fill--stamina" :style="{ width: `${vitalsBarPercent(characterBars(item.inventoryId).stamina)}%` }"></div>
              </div>
            </div>
            <div class="char-bar-group char-bar-group--satiation">
              <div class="char-bar-header">
                <span class="char-bar-label char-bar-label--satiation">🍖 Насыщение</span>
                <span
                  v-if="characterBuffsLoaded(item.inventoryId) && !characterHasFoodBuff(item.inventoryId)"
                  class="char-food-warn"
                >
                  Нет еды
                </span>
                <span class="char-bar-value">{{ formatVitalsValue(characterBars(item.inventoryId).satiation) }}</span>
              </div>
              <div class="char-bar-track">
                <div class="char-bar-fill char-bar-fill--satiation" :style="{ width: `${vitalsBarPercent(characterBars(item.inventoryId).satiation)}%` }"></div>
              </div>
            </div>
            <div class="char-buffs">
              <div class="char-buffs-title">Баффы</div>
              <div v-if="!characterActiveBuffs(item.inventoryId).length" class="char-buffs-empty">
                {{ characterBuffsLoaded(item.inventoryId) ? 'нет активных' : '—' }}
              </div>
              <ul v-else class="char-buff-list">
                <li
                  v-for="buff in characterActiveBuffs(item.inventoryId)"
                  :key="buff.id"
                  class="char-buff-item"
                  :class="{
                    'char-buff-item--food': buff.isFoodBuff,
                    'char-buff-item--bad': !buff.beneficial,
                  }"
                >
                  <span class="char-buff-name">{{ buff.description }}</span>
                  <span class="char-buff-time">{{ buff.timeLabel }}</span>
                </li>
              </ul>
            </div>
            <div class="char-meta">
              <span class="char-meta-updated">Обновлено: {{ state[item.inventoryId]?.updatedAt ?? '—' }}</span>
            </div>
          </div>
        </template>
        <template v-else-if="isMarketBlock(item)">
          <div class="market-block-body">
            <div class="market-item-search">
              <form class="market-item-search-form" @submit.prevent="searchMarketItems(item.inventoryId)">
                <input
                  :value="marketSearchQuery(item.inventoryId)"
                  class="market-item-search-input"
                  type="text"
                  placeholder="Цена предмета..."
                  :disabled="marketSearchSearching(item.inventoryId)"
                  @input="setMarketSearchQuery(item.inventoryId, $event.target.value)"
                />
                <button
                  type="submit"
                  class="market-item-search-btn"
                  :disabled="marketSearchSearching(item.inventoryId) || marketSearchQuery(item.inventoryId).trim().length < 2"
                >
                  {{ marketSearchSearching(item.inventoryId) ? '...' : 'Найти' }}
                </button>
              </form>
              <div v-if="marketSearchError(item.inventoryId)" class="market-item-search-error">{{ marketSearchError(item.inventoryId) }}</div>
              <ul v-if="marketSearchResults(item.inventoryId).length" class="market-item-search-results">
                <li v-for="result in marketSearchResults(item.inventoryId)" :key="`${result.itemType}-${result.id}`">
                  <button type="button" class="market-item-search-result" @click="selectMarketSearchItem(item.inventoryId, result)">
                    <ItemCard
                      variant="mini"
                      :name="result.name"
                      :icon-asset-name="result.iconAssetName"
                      :item-type="result.itemType"
                      :tier="result.tier"
                      :rarity-str="result.rarityStr"
                    />
                    <span class="market-item-search-result-name">{{ result.name }}</span>
                  </button>
                </li>
              </ul>
              <div v-if="marketSearchSelected(item.inventoryId)" class="market-item-price">
                <div v-if="marketSearchPriceLoading(item.inventoryId)" class="market-item-price-loading">Загрузка цены...</div>
                <div v-else class="market-item-price-card">
                  <ItemCard
                    variant="mini"
                    :name="marketSearchSelected(item.inventoryId).name"
                    :icon-asset-name="marketSearchSelected(item.inventoryId).iconAssetName"
                    :item-type="marketSearchSelected(item.inventoryId).itemType"
                    :tier="marketSearchSelected(item.inventoryId).tier"
                    :rarity-str="marketSearchSelected(item.inventoryId).rarityStr"
                  />
                  <div class="market-item-price-lines">
                    <div
                      v-for="line in marketPriceLines(marketSearchStats(item.inventoryId))"
                      :key="line.label"
                      class="market-item-price-line"
                    >
                      <span class="market-small-label">{{ line.label }}</span>
                      <span class="market-item-price-value">{{ line.value }}</span>
                    </div>
                    <div v-if="!marketPriceLines(marketSearchStats(item.inventoryId)).length" class="market-item-price-empty">нет данных о цене</div>
                  </div>
                  <button type="button" class="market-item-price-clear" title="Очистить" @click="clearMarketSearchItem(item.inventoryId)">✕</button>
                </div>
              </div>
            </div>
            <div class="market-orders">
          <div v-if="marketRows(item.inventoryId).length === 0" class="empty-inv">нет ордеров</div>
          <div v-else-if="marketTileSize(item.inventoryId) === 'small'" class="market-list market-list--small">
            <div v-for="(row, idx) in marketRows(item.inventoryId)" :key="row.rowKey ?? idx" class="market-row market-row--small" :class="marketRowDeviationClass(row.diffPct)">
              <div class="market-icon-wrap">
                <ItemCard
                  variant="mini"
                  :name="row.displayName"
                  :icon-asset-name="row.iconAssetName"
                  :item-type="row.itemTypeValue"
                  :tier="row.tier"
                  :rarity-str="row.rarityStr"
                />
                <span class="market-qty">{{ row.qtyDisplay }}</span>
              </div>
              <div class="market-small-values">
                <span class="market-small-line market-amount--ref"><span class="market-small-label">AVG</span> {{ row.refUnitDisplay }}</span>
                <span class="market-small-line" :class="marketDiffClass(row.diffPct)">
                  {{ row.playerUnitDisplay }}<span v-if="row.diffPct != null" class="market-small-pct"> {{ row.diffPct > 0 ? '+' : '' }}{{ row.diffPct.toFixed(1) }}%</span>
                </span>
              </div>
              <div class="market-lot-block">
                <span v-if="row.location" class="market-lot-location">{{ row.location }}</span>
                <span class="market-lot-total">{{ row.playerLotDisplay }}</span>
              </div>
            </div>
          </div>
          <div v-else class="market-list market-list--large">
            <div v-for="(row, idx) in marketRows(item.inventoryId)" :key="row.rowKey ?? idx" class="market-row market-row--large" :class="marketRowDeviationClass(row.diffPct)">
              <div class="market-icon-wrap">
                <ItemCard
                  variant="mini"
                  :name="row.displayName"
                  :icon-asset-name="row.iconAssetName"
                  :item-type="row.itemTypeValue"
                  :tier="row.tier"
                  :rarity-str="row.rarityStr"
                />
                <span class="market-qty">{{ row.qtyDisplay }}</span>
              </div>
              <div class="market-large-name">{{ row.displayName }}</div>
              <div class="market-large-values">
                <span class="market-large-line market-amount--ref"><span class="market-small-label">AVG</span> {{ row.refUnitDisplay }}</span>
                <span class="market-large-line" :class="marketDiffClass(row.diffPct)">
                  {{ row.playerUnitDisplay }}<span v-if="row.diffPct != null" class="market-small-pct"> {{ row.diffPct > 0 ? '+' : '' }}{{ row.diffPct.toFixed(1) }}%</span>
                </span>
              </div>
              <div class="market-lot-block">
                <span v-if="row.locationFull" class="market-lot-location">{{ row.locationFull }}</span>
                <span class="market-lot-total">{{ row.playerLotDisplay }}</span>
              </div>
            </div>
          </div>
            </div>
          </div>
        </template>
        <template v-else-if="isSkillBlock(item)">
          <SkillProgressCard
            :level-table="levelTable"
            :xp="skillXp(item)"
            :rank="state[item.inventoryId]?.skillRanking?.rank ?? null"
            :total-players="state[item.inventoryId]?.skillRanking?.totalPlayers ?? null"
            :skill-id="item.skillId"
            :skill-title="item.skillTitle"
            :size="skillTileSize(item.inventoryId)"
            :history="skillHistory(item)"
            :tracked-since="skillTrackedSince(item)"
            :chart-tab="item.skillChartTab"
            :pace-period="item.skillPacePeriod"
            :pace-mode="item.skillPaceMode"
            @update-view="updatePinnedBlock(item.inventoryId, $event)"
          />
        </template>
        <template v-else-if="isPlannerBlock(item)">
          <div class="planner-block">
            <div v-if="plannerTarget(item.inventoryId)" class="planner-target planner-target--readonly">
              <div class="planner-target-head">
                <ItemCard
                  variant="mini"
                  :name="plannerItemMeta(plannerTarget(item.inventoryId)).name"
                  :icon-asset-name="plannerItemMeta(plannerTarget(item.inventoryId)).iconAssetName"
                  :item-type="plannerItemMeta(plannerTarget(item.inventoryId)).type"
                  :tier="plannerItemMeta(plannerTarget(item.inventoryId)).tier"
                />
                <div class="planner-target-meta">
                  <span class="planner-target-name">{{ plannerItemMeta(plannerTarget(item.inventoryId)).name }}</span>
                  <span class="planner-target-qty">×{{ plannerQuantity(item.inventoryId) }}</span>
                </div>
              </div>
              <div v-if="plannerActiveRecipe(item.inventoryId)" class="planner-recipe-hint">
                {{ plannerActiveRecipe(item.inventoryId).name }}
                <span v-if="plannerActiveRecipe(item.inventoryId).buildingName"> · {{ plannerActiveRecipe(item.inventoryId).buildingName }}</span>
                <span> · выход ×{{ plannerActiveRecipe(item.inventoryId).outputQuantity }}</span>
              </div>
            </div>
            <div v-if="plannerLoading(item.inventoryId)" class="planner-loading">Считаем материалы...</div>
            <div v-else-if="plannerBlockError(item.inventoryId)" class="planner-error">{{ plannerBlockError(item.inventoryId) }}</div>
            <div v-else-if="plannerTarget(item.inventoryId) && !plannerHasRecipe(item.inventoryId)" class="planner-hint">Нет рецепта крафта — базовый ресурс.</div>
            <div v-else-if="plannerTarget(item.inventoryId) && plannerMaterials(item.inventoryId).length === 0" class="planner-hint">Нет данных о материалах.</div>
            <ul
              v-else-if="plannerMaterialRows(item.inventoryId).length"
              class="planner-materials"
              :class="{ 'planner-materials--tree': plannerTileSize(item.inventoryId) === 'large' }"
            >
              <li
                v-for="(mat, idx) in plannerMaterialRows(item.inventoryId)"
                :key="`${mat.itemType}-${mat.id}-${mat.depth}-${idx}`"
                class="planner-material-row"
                :class="{
                  'planner-material-row--nested': mat.depth > 0,
                  'planner-material-row--leaf': !mat.children?.length && mat.depth > 0,
                }"
                :style="mat.depth > 0 ? { paddingLeft: `${8 + mat.depth * 14}px` } : undefined"
              >
                <span v-if="mat.depth > 0" class="planner-tree-guide" aria-hidden="true">└</span>
                <ItemCard
                  variant="mini"
                  :name="plannerItemMeta(mat).name"
                  :icon-asset-name="plannerItemMeta(mat).iconAssetName"
                  :item-type="plannerItemMeta(mat).type"
                  :tier="plannerItemMeta(mat).tier"
                />
                <span class="planner-material-name">{{ plannerItemMeta(mat).name }}</span>
                <span class="planner-material-qty">{{ formatPlannerQty(mat.quantity) }}</span>
              </li>
            </ul>
            <div v-else class="planner-hint">Нажмите ✎, чтобы настроить планировщик.</div>
          </div>
        </template>
        <template v-else>
          <div v-if="!hasInventoryGrid(item.inventoryId)" class="empty-inv">пусто</div>
          <div v-else class="inventory-board">
            <div class="inventory-grid" :style="inventoryGridStyle(item.inventoryId)">
              <div
                v-for="(slot, i) in displayInventorySlots(item.inventoryId)"
                :key="`${slot.kind}-${i}`"
                class="inventory-slot"
                :class="[`inventory-slot--${slot.kind}`, { 'inventory-slot--empty': !hasContents(slot.pocket) }]"
              >
                <template v-if="hasContents(slot.pocket)">
                  <ItemCard
                    variant="mini"
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
          </div>
        </template>
      </div>
    </div>

    <div v-if="wizard.open" class="modal-backdrop" @click.self="closeAddWizard">
      <div class="modal-card">
        <div class="modal-head">
          <div>
            <h2 class="modal-title">Добавить блок</h2>
            <p class="modal-subtitle">{{ wizardSubtitle }}</p>
          </div>
          <button class="modal-close" @click="closeAddWizard">✕</button>
        </div>

        <div class="wizard-steps">
          <span class="wizard-step" :class="{ active: wizard.step === 'type', done: wizardStepIndex > 0 }">Тип</span>
          <span v-if="wizard.blockType === 'inventory'" class="wizard-step" :class="{ active: wizard.step === 'source', done: wizardStepIndex > 1 }">Источник</span>
          <span class="wizard-step" :class="{ active: wizard.step === 'search', done: wizardStepIndex > 2 }">Поиск</span>
          <span v-if="wizard.blockType === 'planner'" class="wizard-step" :class="{ active: wizard.step === 'planner' }">Планировщик</span>
          <span v-if="wizard.blockType === 'inventory'" class="wizard-step" :class="{ active: wizard.step === 'inventory' }">Инвентарь</span>
          <span v-if="wizard.blockType === 'market'" class="wizard-step" :class="{ active: wizard.step === 'search' }">Игрок</span>
          <span v-if="wizard.blockType === 'character'" class="wizard-step" :class="{ active: wizard.step === 'search' }">Игрок</span>
          <span v-if="wizard.blockType === 'skill'" class="wizard-step" :class="{ active: wizard.step === 'skill' }">Навык</span>
        </div>

        <div v-if="wizard.step === 'type'" class="wizard-panel">
          <button class="choice-card" :class="{ active: wizard.blockType === 'inventory' }" @click="chooseBlockType('inventory')">
            <span class="choice-title">Инвентарь</span>
            <span class="choice-text">Блок с содержимым инвентаря игрока, дома или здания клейма.</span>
          </button>
          <button class="choice-card" :class="{ active: wizard.blockType === 'market' }" @click="chooseBlockType('market')">
            <span class="choice-title">Рынок</span>
            <span class="choice-text">Все ордера игрока на продажу с сравнением цены игрока и средней цены рынка.</span>
          </button>
          <button class="choice-card" :class="{ active: wizard.blockType === 'character' }" @click="chooseBlockType('character')">
            <span class="choice-title">Персонаж</span>
            <span class="choice-text">Показатели персонажа: ХП, Стамина и Насыщенность в виде полосок. Обновляются в реальном времени.</span>
          </button>
          <button class="choice-card" :class="{ active: wizard.blockType === 'skill' }" @click="chooseBlockType('skill')">
            <span class="choice-title">Прокачка профессии</span>
            <span class="choice-text">Прогресс навыка до 100 уровня: текущий уровень, путь по кривой опыта и место среди всех игроков.</span>
          </button>
          <button class="choice-card" :class="{ active: wizard.blockType === 'map' }" @click="chooseBlockType('map')">
            <span class="choice-title">Карта</span>
            <span class="choice-text">Интерактивная карта BitCraft из bitcraftmap.com.</span>
          </button>
          <button class="choice-card" :class="{ active: wizard.blockType === 'planner' }" @click="chooseBlockType('planner')">
            <span class="choice-title">Планировщик</span>
            <span class="choice-text">Выбор предмета и количества — список всех материалов для крафта с разбором по рецептам.</span>
          </button>
        </div>

        <div v-else-if="wizard.step === 'source'" class="wizard-panel">
          <button class="choice-card" :class="{ active: wizard.sourceType === 'player' }" @click="chooseSourceType('player')">
            <span class="choice-title">Игрок</span>
            <span class="choice-text">Найти персонажа и выбрать один из его инвентарей.</span>
          </button>
          <button class="choice-card" :class="{ active: wizard.sourceType === 'claim' }" @click="chooseSourceType('claim')">
            <span class="choice-title">Клейм</span>
            <span class="choice-text">Найти поселение и выбрать инвентарь здания.</span>
          </button>
        </div>

        <div v-else-if="wizard.step === 'search'" class="wizard-panel">
          <div class="wizard-search">
            <input
              v-model="wizard.query"
              class="wizard-input"
              type="text"
              :placeholder="wizard.blockType === 'market' || wizard.blockType === 'character' || wizard.blockType === 'skill' || wizard.sourceType === 'player' ? 'Имя игрока...' : 'Название клейма...'"
            />
            <span v-if="wizard.searching" class="wizard-searching-hint">...</span>
          </div>
          <div v-if="wizard.error" class="wizard-error">{{ wizard.error }}</div>
          <div v-if="wizard.results.length" class="wizard-results">
            <button
              v-for="result in wizard.results"
              :key="result.entityId"
              class="wizard-result"
              @click="selectWizardEntity(result)"
            >
              <span class="result-title">{{ wizardResultTitle(result) }}</span>
              <span class="result-meta">{{ wizardResultMeta(result) }}</span>
            </button>
          </div>
        </div>

        <div v-else-if="wizard.step === 'inventory'" class="wizard-panel">
          <div v-if="wizard.loadingInventories" class="placeholder">Загрузка инвентарей...</div>
          <div v-else-if="wizard.error" class="wizard-error">{{ wizard.error }}</div>
          <div v-else-if="wizard.inventories.length === 0" class="placeholder">Инвентари не найдены</div>
          <div v-else class="wizard-inventory-list">
            <section v-for="group in wizardInventoryGroups" :key="group.id" class="wizard-inventory-group">
              <h3 class="wizard-inventory-group-title">{{ group.label }}</h3>
              <button
                v-for="inv in group.items"
                :key="inv.entityId"
                class="wizard-inventory"
                :disabled="isPinned(inv.entityId)"
                @click="addWizardInventory(inv)"
              >
                <div class="wizard-inventory-head">
                  <span class="result-title">{{ wizardInventoryTitle(inv) }}</span>
                  <span class="result-meta">
                    {{ wizardInventoryMeta(inv) }}
                    <template v-if="isPinned(inv.entityId)"> · уже добавлен</template>
                  </span>
                </div>
                <div v-if="wizardPreviewPockets(inv).length" class="wizard-inventory-preview">
                  <div
                    v-for="(pocket, idx) in wizardPreviewPockets(inv)"
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
                  </div>
                </div>
              </button>
            </section>
          </div>
        </div>

        <div v-else-if="wizard.step === 'skill'" class="wizard-panel">
          <div v-if="wizard.loadingSkills" class="placeholder">Загрузка навыков...</div>
          <div v-else-if="wizard.error" class="wizard-error">{{ wizard.error }}</div>
          <div v-else class="wizard-inventory-list">
            <section v-for="group in wizardSkillGroups" :key="group.id" class="wizard-inventory-group">
              <h3 class="wizard-inventory-group-title">{{ group.label }}</h3>
              <button
                v-for="skill in group.items"
                :key="skill.id"
                class="wizard-inventory wizard-skill"
                :disabled="isPinned(skillBlockId(wizard.selectedEntity?.entityId, skill.id))"
                @click="addWizardSkill(skill)"
              >
                <span class="wizard-skill-glyph">{{ skillGlyph(skill.id) }}</span>
                <span class="wizard-skill-name">
                  <span class="result-title">{{ skill.name }}</span>
                  <span v-if="isPinned(skillBlockId(wizard.selectedEntity?.entityId, skill.id))" class="result-meta">уже добавлен</span>
                </span>
                <span class="wizard-skill-level">ур. {{ skill.level }}</span>
                <span class="wizard-skill-rank">{{ skill.rank ? `#${skill.rank.toLocaleString('ru-RU')}` : '—' }}</span>
              </button>
            </section>
          </div>
        </div>

        <div v-else-if="wizard.step === 'planner'" class="wizard-panel wizard-planner-panel">
          <div v-if="wizard.error" class="wizard-error">{{ wizard.error }}</div>
          <div class="planner-search">
            <input
              :value="wizard.plannerQuery"
              class="planner-search-input"
              type="text"
              placeholder="Предмет для крафта..."
              @input="setWizardPlannerQuery($event.target.value)"
            />
            <span v-if="wizard.plannerSearching" class="planner-searching">...</span>
          </div>
          <ul v-if="wizard.plannerResults.length" class="planner-search-results wizard-planner-results">
            <li v-for="result in wizard.plannerResults" :key="`${result.itemType}-${result.id}`">
              <button
                type="button"
                class="planner-search-result"
                :class="{ 'planner-search-result--active': wizard.plannerTarget?.id === result.id && wizard.plannerTarget?.itemType === result.itemType }"
                @click="selectWizardPlannerTarget(result)"
              >
                <ItemCard
                  variant="mini"
                  :name="plannerItemMeta(result).name"
                  :icon-asset-name="plannerItemMeta(result).iconAssetName"
                  :item-type="plannerItemMeta(result).type"
                  :tier="plannerItemMeta(result).tier"
                />
                <span class="planner-search-result-name">{{ plannerItemMeta(result).name }}</span>
              </button>
            </li>
          </ul>
          <div v-if="wizard.plannerTarget" class="planner-target wizard-planner-target">
            <div class="planner-target-head">
              <ItemCard
                variant="mini"
                :name="plannerItemMeta(wizard.plannerTarget).name"
                :icon-asset-name="plannerItemMeta(wizard.plannerTarget).iconAssetName"
                :item-type="plannerItemMeta(wizard.plannerTarget).type"
                :tier="plannerItemMeta(wizard.plannerTarget).tier"
              />
              <div class="planner-target-meta">
                <span class="planner-target-name">{{ plannerItemMeta(wizard.plannerTarget).name }}</span>
                <label class="planner-qty-label">
                  Кол-во
                  <input
                    class="planner-qty-input"
                    type="number"
                    min="1"
                    step="1"
                    :value="wizard.plannerQuantity"
                    @change="wizard.plannerQuantity = Math.max(1, Math.floor(Number($event.target.value) || 1))"
                  />
                </label>
              </div>
            </div>
            <div v-if="wizard.plannerLoadingRecipes" class="planner-loading">Загрузка рецептов...</div>
            <div v-else-if="wizard.plannerRecipeOptions.length > 1" class="planner-recipes">
              <span class="planner-recipes-label">Рецепт</span>
              <div class="planner-recipes-list">
                <button
                  v-for="recipe in wizard.plannerRecipeOptions"
                  :key="recipe.id"
                  type="button"
                  class="planner-recipe-option"
                  :class="{ 'planner-recipe-option--active': wizard.plannerSelectedRecipeId === String(recipe.id) }"
                  :title="plannerRecipeOptionTitle(recipe)"
                  @click="wizard.plannerSelectedRecipeId = String(recipe.id)"
                >
                  <span class="planner-recipe-option-title">{{ plannerRecipeOptionTitle(recipe) }}</span>
                  <span class="planner-recipe-option-meta">{{ plannerRecipeOptionMeta(recipe) }}</span>
                </button>
              </div>
            </div>
            <div v-else-if="wizard.plannerRecipeOptions.length === 1" class="planner-recipe-hint">
              {{ plannerRecipeOptionTitle(wizard.plannerRecipeOptions[0]) }}
              <span> · {{ plannerRecipeOptionMeta(wizard.plannerRecipeOptions[0]) }}</span>
            </div>
          </div>
        </div>

        <div class="modal-actions">
          <button
            v-if="wizard.step === 'planner'"
            class="add-block-btn"
            type="button"
            :disabled="!wizardPlannerCanConfirm || wizard.plannerLoadingRecipes"
            @click="confirmWizardPlanner"
          >
            {{ wizard.editInventoryId ? 'Сохранить' : 'Добавить блок' }}
          </button>
          <button class="ghost-btn" type="button" @click="wizardBack" :disabled="wizard.step === 'type' && !wizard.editInventoryId">Назад</button>
          <button class="ghost-btn" type="button" @click="closeAddWizard">Отмена</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { computed, reactive, ref, watch, onMounted, onUnmounted } from 'vue'
import { fetchBitjita, fetchXpHistory, postBitjita } from '../api'
import { usePinned } from '../composables/usePinned'
import { useItemCache } from '../composables/useItemCache'
import { useBitjitaLive } from '../composables/useBitjitaLive'
import { useStaminaAlerts } from '../composables/useStaminaAlerts'
import { activeBuffsFromPayload, hasActiveFoodBuff } from '../playerBuffs'
import { characterVitalsFromStats, formatVitalsValue, vitalsBarPercent } from '../playerStats'
import {
  buildCraftTreeFromRecipes,
  collectCraftTreeItemRefs,
  computeCraftMaterialsFromRecipes,
  ensureCraftTreeRecipeCache,
  flattenCraftTree,
  listRecipeOptions,
  resolveRecipeId,
} from '../plannerCrafting'
import { dedupePlannerSearchResults, normalizePlannerSearchEntry } from '../plannerSearch'
import { SKILL_CATEGORIES, levelForXp, parseLevelTable, skillGlyph } from '../skillProgress'
import { mergeHistoryPoints, splitHistorySamples } from '../xpHistory'
import ItemCard from './ItemCard.vue'
import SkillProgressCard from './SkillProgressCard.vue'

const SEARCH_DEBOUNCE_MS = 500

export default {
  name: 'PinnedDashboard',
  components: { ItemCard, SkillProgressCard },
  setup() {
    const { pinned, isPinned, unpin, pinBlock, updatePinnedLayout, updatePinnedBlock } = usePinned()
    const state = reactive({})
    const gridRef = ref(null)
    const dashboardCols = ref(4)
    const dragState = reactive({ inventoryId: null, overCell: null })
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
    // playerId → { trackedSince, lastT, bySkill: { skillId: frozen [[unixSec, xp], ...] } }
    const xpHistory = reactive({})
    const { resolveItem, prefetch, prefetchItems, seedCatalog } = useItemCache()
    const { syncPlayerSubscriptions, syncExperienceSubscriptions, experienceByPlayer, liveSnapshot, staminaByPlayer } = useBitjitaLive()
    const BULK_CHUNK = 100
    const MAP_EMBED_PATH = '/map-embed/'
    const MAP_ROADS_LAYER = 'roadsLayer'
    const mapIframeRefs = {}
    const marketSearchState = reactive({})
    const plannerBlockState = reactive({})
    let wizardPlannerSearchTimer = null
    let wizardPlannerSearchGen = 0
    let wizardSearchTimer = null
    let wizardSearchGen = 0
    let mapUrlSyncTimer = null
    const wizardSteps = computed(() => {
      if (wizard.blockType === 'inventory') return ['type', 'source', 'search', 'inventory']
      if (wizard.blockType === 'planner') return ['type', 'planner']
      if (wizard.blockType === 'skill') return ['type', 'search', 'skill']
      return ['type', 'search']
    })
    const wizardInventoryCategoryOrder = [
      { id: 'inventory', label: 'Инвентарь' },
      { id: 'house', label: 'Дом' },
      { id: 'bank', label: 'Банки' },
      { id: 'recovery', label: 'Восстановление' },
      { id: 'claim', label: 'Клейм' },
      { id: 'trade', label: 'Торговля' },
    ]

    const wizardStepIndex = computed(() => wizardSteps.value.indexOf(wizard.step))
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
    const wizardSubtitle = computed(() => {
      if (wizard.step === 'type') return 'Выбери, какой вид блока добавить на дашборд.'
      if (wizard.step === 'source') return 'Выбери, откуда взять инвентарь.'
      if (wizard.step === 'search') {
        if (wizard.blockType === 'character') return 'Найди игрока, чтобы следить за его показателями.'
        if (wizard.blockType === 'skill') return 'Найди игрока, чью прокачку отслеживать.'
        if (wizard.blockType === 'market' || wizard.sourceType === 'player') return 'Найди игрока по имени.'
        return 'Найди клейм по названию.'
      }
      if (wizard.step === 'skill') return 'Выбери навык — рядом текущий уровень и место в рейтинге.'
      if (wizard.step === 'planner') {
        return wizard.editInventoryId
          ? 'Измените предмет, рецепт и количество для этого блока.'
          : 'Выберите предмет для крафта, рецепт и количество.'
      }
      return 'Выбери конкретный инвентарь для добавления.'
    })
    const dashboardUpdatedLabel = computed(() => {
      const refreshing = pinned.value.some((p) => state[p.inventoryId]?.refreshing)
      const times = pinned.value.map((p) => state[p.inventoryId]?.updatedAt).filter(Boolean)
      if (refreshing && times.length > 0) return `Обновлено: ${times[0]} · обновляем...`
      if (refreshing) return 'Обновляем...'
      if (times.length > 0) return `Обновлено: ${times[0]}`
      return `${pinned.value.length} закреплён${plural(pinned.value.length)}`
    })

    const layoutInfo = computed(() => {
      const occupied = new Map()
      const positions = {}
      let maxY = 0
      pinned.value.forEach((item) => {
        const span = dashboardCardGridSpan(item.inventoryId)
        const saved = item.dashboardLayout ?? {}
        const savedX = Number(saved.x)
        const savedY = Number(saved.y)
        let position = null
        if (
          Number.isInteger(savedX) &&
          Number.isInteger(savedY) &&
          savedX >= 0 &&
          savedY >= 0 &&
          savedX + span <= dashboardCols.value &&
          canOccupy(occupied, savedX, savedY, span)
        ) {
          position = { x: savedX, y: savedY }
        } else {
          position = findFreeCell(occupied, span)
        }
        markOccupied(occupied, position.x, position.y, span, item.inventoryId)
        positions[item.inventoryId] = { ...position, span }
        maxY = Math.max(maxY, position.y)
      })
      return { occupied, positions, rows: Math.max(maxY + 2, 2) }
    })
    const layoutCells = computed(() => {
      const cells = []
      for (let y = 0; y < layoutInfo.value.rows; y += 1) {
        for (let x = 0; x < dashboardCols.value; x += 1) cells.push({ x, y })
      }
      return cells
    })

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
          activeRecipe: null,
          hasRecipe: false,
          materials: [],
          materialTree: null,
          recipeCache: {},
          loading: false,
          error: null,
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
    function plannerActiveRecipe(inventoryId) {
      return ensurePlannerState(inventoryId).activeRecipe
    }
    function plannerRecipeOptionTitle(recipe) {
      if (!recipe) return 'Рецепт'
      if (recipe.buildingName) {
        return recipe.buildingTier ? `${recipe.buildingName} · T${recipe.buildingTier}` : recipe.buildingName
      }
      return recipe.name
    }
    function plannerRecipeOptionMeta(recipe) {
      if (!recipe) return ''
      const parts = [`выход ×${recipe.outputQuantity}`, `${recipe.ingredientCount} ингр.`]
      if (!recipe.buildingName && recipe.name) parts.push(recipe.name)
      return parts.join(' · ')
    }
    function plannerHasRecipe(inventoryId) {
      return ensurePlannerState(inventoryId).hasRecipe
    }
    function plannerMaterials(inventoryId) {
      return ensurePlannerState(inventoryId).materials
    }
    function plannerTileSize(inventoryId) {
      const block = pinned.value.find((p) => p.inventoryId === inventoryId)
      return block?.plannerTileSize === 'large' ? 'large' : 'small'
    }
    function setPlannerTileSize(inventoryId, size) {
      if (size !== 'small' && size !== 'large') return
      updatePinnedBlock(inventoryId, { plannerTileSize: size })
      if (size === 'large') {
        const st = ensurePlannerState(inventoryId)
        if (st.target?.id && !st.materialTree) calculatePlannerMaterials(inventoryId)
      }
    }
    function plannerMaterialRows(inventoryId) {
      const st = ensurePlannerState(inventoryId)
      if (plannerTileSize(inventoryId) === 'large' && st.materialTree?.children?.length) {
        return flattenCraftTree(st.materialTree.children)
      }
      return st.materials.map((mat) => ({ ...mat, depth: 0, children: [] }))
    }
    function plannerLoading(inventoryId) {
      return ensurePlannerState(inventoryId).loading
    }
    function formatPlannerQty(value) {
      const n = Number(value)
      if (!Number.isFinite(n)) return '—'
      return n.toLocaleString('ru-RU')
    }
    async function fetchCraftDetails(itemType, itemId) {
      const id = String(itemId ?? '').trim()
      if (!/^\d+$/.test(id)) {
        throw new Error('Некорректный ID предмета')
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
              // skip candidate
            }
          }
        } catch {
          // search failed — keep original target
        }
      }

      return { target: resolvedTarget, recipes, data }
    }
    async function fetchCraftRecipesSafe(itemType, itemId) {
      const id = String(itemId ?? '').trim()
      if (!/^\d+$/.test(id)) return []
      const paths = itemType === 1
        ? [`/cargo/${id}`, `/items/${id}`]
        : [`/items/${id}`, `/cargo/${id}`]
      for (const path of paths) {
        try {
          const data = await fetchBitjita(path)
          const entity = data.item ?? data.cargo
          if (entity) {
            seedCatalog(path.startsWith('/cargo') ? { cargos: [entity] } : { items: [entity] })
          }
          return data.craftingRecipes ?? []
        } catch {
          // BitJita may reject recipe ingredient IDs (e.g. cargo stacks) — treat as leaf.
        }
      }
      return []
    }
    async function calculatePlannerMaterials(inventoryId, options = {}) {
      const st = ensurePlannerState(inventoryId)
      const target = st.target
      if (!target?.id) return
      st.loading = true
      st.error = null
      try {
        let recipes
        if (options.reuseRecipes && st.craftRecipes?.length) {
          recipes = st.craftRecipes
        } else {
          const resolved = await fetchPlannerTargetDetails(target)
          if (resolved.target.id !== target.id || resolved.target.itemType !== target.itemType) {
            st.target = resolved.target
          }
          recipes = resolved.recipes
          st.craftRecipes = recipes

          const pin = pinned.value.find((p) => p.inventoryId === inventoryId)
          const preferredId = st.selectedRecipeId ?? pin?.plannerRecipeId ?? null
          st.selectedRecipeId = resolveRecipeId(recipes, st.target.id, preferredId)
        }

        const activeTarget = st.target
        const result = computeCraftMaterialsFromRecipes({
          itemId: activeTarget.id,
          itemType: activeTarget.itemType,
          quantity: st.quantity,
          recipes,
          recipeId: st.selectedRecipeId,
        })
        st.activeRecipe = result.recipe
        st.hasRecipe = result.hasRecipe
        st.materials = result.materials

        if (plannerTileSize(inventoryId) === 'large') {
          const recipeCache = { ...st.recipeCache }
          const rootKey = `${activeTarget.itemType}:${activeTarget.id}`
          recipeCache[rootKey] = recipes
          await ensureCraftTreeRecipeCache({
            itemId: activeTarget.id,
            itemType: activeTarget.itemType,
            quantity: st.quantity,
            recipes,
            recipeId: st.selectedRecipeId,
            recipeCache,
            fetchRecipes: fetchCraftRecipesSafe,
          })
          st.recipeCache = recipeCache
          st.materialTree = buildCraftTreeFromRecipes({
            itemId: activeTarget.id,
            itemType: activeTarget.itemType,
            quantity: st.quantity,
            recipes,
            recipeId: st.selectedRecipeId,
            recipeCache,
          })
        } else {
          st.materialTree = null
        }

        const prefetchRefs = [
          { id: activeTarget.id, itemType: activeTarget.itemType },
          ...result.materials,
        ]
        if (st.materialTree?.children?.length) {
          prefetchRefs.push(...collectCraftTreeItemRefs(st.materialTree.children))
        }
        await prefetchItems(prefetchRefs)
      } catch (e) {
        st.error = e.message
        st.materials = []
        st.materialTree = null
        st.activeRecipe = null
        st.hasRecipe = false
      } finally {
        st.loading = false
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
        if (!wizard.plannerResults.length) wizard.error = 'Ничего не найдено'
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
        const data = await fetchCraftDetails(wizard.plannerTarget.itemType, wizard.plannerTarget.id)
        wizard.plannerCraftRecipes = data.craftingRecipes ?? []
        wizard.plannerRecipeOptions = listRecipeOptions(wizard.plannerCraftRecipes, wizard.plannerTarget.id)
        wizard.plannerSelectedRecipeId = resolveRecipeId(
          wizard.plannerCraftRecipes,
          wizard.plannerTarget.id,
          wizard.plannerSelectedRecipeId,
        )
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
        inventoryName: `Планировщик · ${name}`,
      }
      if (wizard.editInventoryId) {
        const inventoryId = wizard.editInventoryId
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
    function ensureMarketSearchState(inventoryId) {
      if (!marketSearchState[inventoryId]) {
        marketSearchState[inventoryId] = {
          query: '',
          searching: false,
          error: null,
          results: [],
          selected: null,
          stats: null,
          priceLoading: false,
        }
      }
      return marketSearchState[inventoryId]
    }
    function marketSearchQuery(inventoryId) {
      return ensureMarketSearchState(inventoryId).query
    }
    function setMarketSearchQuery(inventoryId, value) {
      ensureMarketSearchState(inventoryId).query = value
    }
    function marketSearchSearching(inventoryId) {
      return ensureMarketSearchState(inventoryId).searching
    }
    function marketSearchError(inventoryId) {
      return ensureMarketSearchState(inventoryId).error
    }
    function marketSearchResults(inventoryId) {
      return ensureMarketSearchState(inventoryId).results
    }
    function marketSearchSelected(inventoryId) {
      return ensureMarketSearchState(inventoryId).selected
    }
    function marketSearchStats(inventoryId) {
      return ensureMarketSearchState(inventoryId).stats
    }
    function marketSearchPriceLoading(inventoryId) {
      return ensureMarketSearchState(inventoryId).priceLoading
    }
    function normalizeMarketSearchEntry(raw, itemType) {
      return {
        id: raw.id ?? raw.itemId ?? raw.item_id,
        name: raw.name ?? raw.itemName ?? 'Предмет',
        iconAssetName: raw.iconAssetName ?? raw.icon_asset_name ?? null,
        tier: raw.tier ?? raw.itemTier ?? null,
        rarityStr: raw.rarityStr ?? raw.rarity_str ?? null,
        itemType,
      }
    }
    function formatMarketStat(value) {
      if (value == null || !Number.isFinite(Number(value))) return '—'
      return Number(value).toLocaleString('ru-RU', { maximumFractionDigits: 2 })
    }
    function marketPriceLines(stats) {
      if (!stats || typeof stats !== 'object') return []
      const lines = []
      if (stats.vwap7d != null && Number.isFinite(Number(stats.vwap7d))) {
        lines.push({ label: '7d AVG', value: formatMarketStat(stats.vwap7d) })
      }
      if (stats.vwap24h != null && Number.isFinite(Number(stats.vwap24h))) {
        lines.push({ label: '24h AVG', value: formatMarketStat(stats.vwap24h) })
      }
      if (stats.lowestSellPrice != null && Number.isFinite(Number(stats.lowestSellPrice))) {
        lines.push({ label: 'Мин. ask', value: formatMarketStat(stats.lowestSellPrice) })
      }
      if (stats.highestBuyPrice != null && Number.isFinite(Number(stats.highestBuyPrice))) {
        lines.push({ label: 'Макс. bid', value: formatMarketStat(stats.highestBuyPrice) })
      }
      return lines
    }
    async function searchMarketItems(inventoryId) {
      const st = ensureMarketSearchState(inventoryId)
      const q = st.query.trim()
      if (q.length < 2) return
      st.searching = true
      st.error = null
      st.results = []
      st.selected = null
      st.stats = null
      try {
        const [itemsRes, cargoRes] = await Promise.all([
          fetchBitjita('/items', { q }),
          fetchBitjita('/cargo', { q }),
        ])
        const items = (itemsRes.items ?? []).map((item) => normalizeMarketSearchEntry(item, 0))
        const cargos = (cargoRes.cargos ?? []).map((cargo) => normalizeMarketSearchEntry(cargo, 1))
        st.results = [...items, ...cargos].slice(0, 10)
        if (!st.results.length) st.error = 'Ничего не найдено'
      } catch (e) {
        st.error = e.message
      } finally {
        st.searching = false
      }
    }
    async function selectMarketSearchItem(inventoryId, entry) {
      const st = ensureMarketSearchState(inventoryId)
      st.selected = entry
      st.results = []
      st.error = null
      st.stats = null
      st.priceLoading = true
      seedCatalog(entry.itemType === 1 ? { cargos: [entry] } : { items: [entry] })
      const id = Number(entry.id)
      if (!Number.isFinite(id)) {
        st.priceLoading = false
        st.error = 'Некорректный ID предмета'
        return
      }
      try {
        const bulk = entry.itemType === 1
          ? await postBitjita('/market/prices/bulk', { itemIds: [], cargoIds: [id] })
          : await postBitjita('/market/prices/bulk', { itemIds: [id], cargoIds: [] })
        const maps = bulk?.data ?? {}
        st.stats = entry.itemType === 1 ? maps.cargo?.[String(id)] : maps.items?.[String(id)] ?? null
      } catch (e) {
        st.error = e.message
      } finally {
        st.priceLoading = false
      }
    }
    function clearMarketSearchItem(inventoryId) {
      const st = ensureMarketSearchState(inventoryId)
      st.selected = null
      st.stats = null
      st.error = null
      st.results = []
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
    function ensureMapRoadsLayer(pathWithQueryHash) {
      try {
        const url = new URL(pathWithQueryHash, window.location.origin)
        const layers = url.searchParams.getAll('turnLayerOn')
        if (!layers.includes(MAP_ROADS_LAYER)) {
          url.searchParams.append('turnLayerOn', MAP_ROADS_LAYER)
        }
        return url.pathname + url.search + url.hash
      } catch {
        return pathWithQueryHash
      }
    }
    function mapBlockUrl(pinItem) {
      const embedUrl = new URL(MAP_EMBED_PATH, window.location.origin)
      const raw = pinItem?.mapUrl
      if (raw) {
        try {
          const src = new URL(raw, window.location.origin)
          src.searchParams.delete('bitdashMini')
          embedUrl.search = src.search
          embedUrl.hash = src.hash
        } catch {
          // keep default embed path
        }
      }
      if (mapTileSize(pinItem?.inventoryId) === 'small') {
        embedUrl.searchParams.set('bitdashMini', '1')
      }
      return ensureMapRoadsLayer(embedUrl.pathname + embedUrl.search + embedUrl.hash)
    }
    function setMapIframeRef(inventoryId, el) {
      if (el) mapIframeRefs[inventoryId] = el
      else delete mapIframeRefs[inventoryId]
    }
    function syncMapBlockUrl(inventoryId) {
      const iframe = mapIframeRefs[inventoryId]
      if (!iframe?.contentWindow) return
      try {
        const loc = iframe.contentWindow.location
        if (!loc.pathname.startsWith('/map-embed')) return
        const saved = new URL(`${loc.pathname}${loc.search}${loc.hash}`, window.location.origin)
        saved.searchParams.delete('bitdashMini')
        const next = `${saved.pathname}${saved.search}${saved.hash}`
        const pin = pinned.value.find((p) => p.inventoryId === inventoryId)
        if (pin && pin.mapUrl !== next) {
          updatePinnedBlock(inventoryId, { mapUrl: next })
        }
      } catch {
        // ignore cross-origin until embed is ready
      }
    }
    function syncAllMapBlockUrls() {
      pinned.value.filter(isMapBlock).forEach((p) => syncMapBlockUrl(p.inventoryId))
    }
    function onMapIframeLoad(inventoryId) {
      syncMapBlockUrl(inventoryId)
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
    function parseBigIntSafe(s) {
      try {
        return BigInt(s || '0')
      } catch {
        return 0n
      }
    }
    function orderQtyBn(order) {
      return parseBigIntSafe(order.quantity)
    }
    function unitPriceBn(order) {
      const v = order.price ?? order.priceThreshold
      const bn = parseBigIntSafe(v)
      return bn > 0n ? bn : null
    }
    function lotPriceBn(order) {
      const unit = unitPriceBn(order)
      const qty = orderQtyBn(order)
      if (unit == null || qty <= 0n) return null
      return unit * qty
    }
    function marketReference(stats) {
      if (!stats || typeof stats !== 'object') return { value: null, kind: '' }
      for (const [key, kind] of [['vwap7d', '7d Avg'], ['vwap24h', '24h Avg'], ['lowestSellPrice', 'Low ask']]) {
        const n = Number(stats[key])
        if (Number.isFinite(n) && n > 0) return { value: n, kind }
      }
      return { value: null, kind: '' }
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
    async function loadMarketForPlayer(playerEntityId) {
      const marketPinned = pinned.value.filter((p) => isMarketBlock(p) && String(p.playerEntityId) === String(playerEntityId))
      if (!marketPinned.length) return
      marketPinned.forEach((p) => {
        const st = ensureInventoryState(p.inventoryId)
        st.loading = true
        st.refreshing = false
        st.error = null
      })
      try {
        const data = await fetchBitjita(`/players/${playerEntityId}/market`)
        seedCatalog(data)
        const sellOrders = Array.isArray(data.sellOrders) ? data.sellOrders : (Array.isArray(data.sell_orders) ? data.sell_orders : [])
        await prefetch([{
          pockets: sellOrders.map((order) => ({
            contents: { itemId: orderItemId(order), itemType: orderItemType(order) },
          })),
        }])
        const itemIds = [...new Set(sellOrders.filter((o) => orderItemType(o) === 0).map(orderItemId).filter((id) => id != null).map(Number).filter((n) => Number.isFinite(n)))]
        const cargoIds = [...new Set(sellOrders.filter((o) => orderItemType(o) === 1).map(orderItemId).filter((id) => id != null).map(Number).filter((n) => Number.isFinite(n)))]
        const bulkMaps = await fetchBulkMaps(itemIds, cargoIds)
        marketPinned.forEach((p) => {
          const st = ensureInventoryState(p.inventoryId)
          st.loading = false
          st.refreshing = false
          st.error = null
          st.marketOrders = sellOrders
          st.marketBulkMaps = bulkMaps
          st.updatedAt = new Date().toLocaleTimeString('ru-RU')
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
          invState.error = inv ? null : 'Инвентарь не найден'
          if (inv) {
            invState.inventory = inv
            invState.pockets = inv.pockets ?? []
            invState.updatedAt = new Date().toLocaleTimeString('ru-RU')
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
          invState.error = inv ? null : 'Инвентарь не найден'
          if (inv) {
            invState.inventory = inv
            invState.pockets = inv.pockets ?? []
            invState.updatedAt = new Date().toLocaleTimeString('ru-RU')
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
    async function loadAll() {
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
        ...skillPlayerIds.map(loadSkillProgress),
      ])
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
      if (isPlannerBlock(pinItem)) {
        if (plannerTileSize(inventoryId) === 'large') return 2
        return plannerMaterials(inventoryId).length > 6 ? 2 : 1
      }
      if (isMarketBlock(pinItem)) return marketTileSize(inventoryId) === 'small' ? 1 : 2
      const itemCount = displayItemSlots(inventoryId).length
      const cargoCount = displayCargoSlots(inventoryId).length
      if (cargoCount > 0) return 2
      if (itemCount > 64) return 2
      return 1
    }
    function dashboardCardGridSpan(inventoryId) { return Math.min(dashboardCardSpan(inventoryId), dashboardCols.value) }
    function cellKey(x, y) { return `${x}:${y}` }
    function canOccupy(occupied, x, y, span) {
      if (x < 0 || y < 0 || x + span > dashboardCols.value) return false
      for (let offset = 0; offset < span; offset += 1) {
        if (occupied.get(cellKey(x + offset, y))) return false
      }
      return true
    }
    function markOccupied(occupied, x, y, span, inventoryId = true) {
      for (let offset = 0; offset < span; offset += 1) occupied.set(cellKey(x + offset, y), inventoryId)
    }
    function findFreeCell(occupied, span) {
      for (let y = 0; y < 100; y += 1) {
        for (let x = 0; x <= dashboardCols.value - span; x += 1) {
          if (canOccupy(occupied, x, y, span)) return { x, y }
        }
      }
      return { x: 0, y: 0 }
    }
    function occupiedWithout(inventoryId) {
      const occupied = new Map()
      Object.entries(layoutInfo.value.positions).forEach(([id, pos]) => {
        if (id === String(inventoryId)) return
        markOccupied(occupied, pos.x, pos.y, pos.span, id)
      })
      return occupied
    }
    function canDropAt(inventoryId, cell) {
      if (!inventoryId) return false
      return canOccupy(occupiedWithout(inventoryId), cell.x, cell.y, dashboardCardGridSpan(inventoryId))
    }
    function dashboardCardStyle(inventoryId) {
      const pos = layoutInfo.value.positions[inventoryId]
      if (!pos) return {}
      return { gridColumn: `${pos.x + 1} / span ${pos.span}`, gridRow: `${pos.y + 1} / span 1` }
    }
    function dashboardCellStyle(cell) { return { gridColumn: `${cell.x + 1} / span 1`, gridRow: `${cell.y + 1} / span 1` } }
    function dashboardCellClass(cell) {
      const isOccupied = layoutInfo.value.occupied.has(cellKey(cell.x, cell.y))
      const isOver = dragState.overCell?.x === cell.x && dragState.overCell?.y === cell.y
      return {
        'dashboard-cell--occupied': isOccupied,
        'dashboard-cell--drop-ok': isOver && canDropAt(dragState.inventoryId, cell),
        'dashboard-cell--drop-blocked': isOver && dragState.inventoryId && !canDropAt(dragState.inventoryId, cell),
      }
    }
    function dashboardCardClass(inventoryId) {
      return {
        'inv-card--large': dashboardCardSpan(inventoryId) === 2,
        'inv-card--small': dashboardCardSpan(inventoryId) === 1,
        'inv-card--dragging': dragState.inventoryId === inventoryId,
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
      if (isMarketBlock(item)) return marketRows(item.inventoryId).length === 0
      if (isCharacterBlock(item)) return !state[item.inventoryId]?.characterStats
      if (isSkillBlock(item)) return !state[item.inventoryId]?.skillRanking || !levelTable.value
      return !hasInventoryGrid(item.inventoryId)
    }
    function formatBig(value) {
      return value == null ? '—' : value.toString().replace(/\B(?=(\d{3})+(?!\d))/g, '\u202f')
    }
    function marketRows(inventoryId) {
      const st = state[inventoryId]
      const orders = st?.marketOrders ?? []
      const maps = st?.marketBulkMaps ?? { items: {}, cargo: {} }
      return orders.map((o, idx) => {
        const meta = resolveItem({ itemId: orderItemId(o), itemType: orderItemType(o) })
        const qtyBn = orderQtyBn(o)
        const playerLotBn = lotPriceBn(o)
        const playerUnitBn = unitPriceBn(o)
        const stats = orderItemType(o) === 1 ? maps.cargo[String(orderItemId(o))] : maps.items[String(orderItemId(o))]
        const ref = marketReference(stats)
        const diffPct = ref.value && playerUnitBn != null ? ((Number(playerUnitBn) - ref.value) / ref.value) * 100 : null
        return {
          rowKey: o.entityId ?? o.orderEntityId ?? o.entity_id ?? idx,
          displayName: o.itemName ?? o.item_name ?? o.name ?? meta.name,
          iconAssetName: o.iconAssetName ?? o.icon_asset_name ?? o.itemIconAssetName ?? o.item_icon_asset_name ?? meta.iconAssetName ?? null,
          tier: o.tier ?? o.itemTier ?? o.item_tier ?? meta.tier ?? null,
          rarityStr: o.rarityStr ?? o.itemRarityStr ?? o.item_rarity_str ?? meta.rarityStr ?? null,
          itemTypeValue: orderItemType(o),
          qtyDisplay: formatBig(qtyBn),
          playerUnitDisplay: formatBig(playerUnitBn),
          refUnitDisplay: ref.value != null ? Number(ref.value).toLocaleString('ru-RU', { maximumFractionDigits: 2 }) : '—',
          playerLotDisplay: formatBig(playerLotBn),
          diffPct: Number.isFinite(diffPct) ? diffPct : null,
          location: o.claimName ?? o.claim_name ?? o.marketClaimName ?? null,
          locationFull: [
            o.claimName ?? o.claim_name ?? o.marketClaimName ?? null,
            o.regionName ?? o.region_name ?? null,
          ].filter(Boolean).join(' · ') || null,
        }
      })
    }
    function marketRowDeviationClass(pct) {
      if (pct == null) return null
      if (pct > 15) return 'market-row--dev-above'
      if (pct < -15) return 'market-row--dev-below'
      return null
    }
    function marketDeviationCount(inventoryId, direction) {
      return marketRows(inventoryId).filter((row) => {
        if (row.diffPct == null) return false
        return direction === 'above' ? row.diffPct > 15 : row.diffPct < -15
      }).length
    }
    function marketDiffClass(pct) {
      if (pct == null) return 'market-diff--neutral'
      if (pct > 10) return 'market-diff--above'
      if (pct < -10) return 'market-diff--below'
      return 'market-diff--neutral'
    }
    function marketDiffLabel(pct) {
      if (pct == null) return '—'
      return `${pct > 0 ? '+' : ''}${pct.toFixed(1)}%`
    }
    function itemMeta(contents) { return resolveItem(contents) }
    function plannerItemMeta(itemRef) {
      if (!itemRef?.id) return { name: '?', type: 0 }
      return resolveItem({ itemId: itemRef.id, itemType: itemRef.itemType })
    }
    function quantityDisplay(quantity) { return Number(quantity ?? 0).toLocaleString('ru-RU') }
    function blockOwnerLabel(item) {
      if (isMarketBlock(item)) return item.playerName ? `Игрок: ${item.playerName}` : 'Рынок игрока'
      if (sourceType(item) === 'claim') return item.claimName ? `Клейм: ${item.claimName}` : 'Клейм'
      return item.playerName ?? 'Игрок'
    }
    function plural(n) {
      if (n % 10 === 1 && n % 100 !== 11) return ''
      if (n % 10 >= 2 && n % 10 <= 4 && (n % 100 < 10 || n % 100 >= 20)) return 'о'
      return 'о'
    }
    function startDrag(event, inventoryId) {
      dragState.inventoryId = inventoryId
      dragState.overCell = null
      event.dataTransfer.effectAllowed = 'move'
      event.dataTransfer.setData('text/plain', String(inventoryId))
    }
    function dragOverCell(cell) {
      if (!dragState.inventoryId) return
      dragState.overCell = cell
    }
    function dropOnCell(event, cell) {
      event.preventDefault()
      if (dragState.inventoryId && canDropAt(dragState.inventoryId, cell)) {
        updatePinnedLayout(dragState.inventoryId, { x: cell.x, y: cell.y })
      }
      endDrag()
    }
    function endDrag() {
      dragState.inventoryId = null
      dragState.overCell = null
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
      wizard.step = (type === 'market' || type === 'character' || type === 'skill') ? 'search' : 'source'
    }
    function chooseSourceType(type) {
      wizard.sourceType = type
      wizard.query = ''
      wizard.error = null
      wizard.results = []
      wizard.inventories = []
      wizard.step = 'search'
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
        if (wizard.results.length === 0) wizard.error = searchPlayers ? 'Игрок не найден' : 'Клейм не найден'
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
        inventoryName: `Рынок · ${player.username}`,
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
        inventoryName: `Персонаж · ${player.username}`,
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
        inventoryName: 'Карта',
        mapUrl: ensureMapRoadsLayer(MAP_EMBED_PATH),
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
        if (!stats?.values) throw new Error('нет данных статов')
        const buffs = buffsData?.buffs ?? null
        charPinned.forEach((p) => {
          const st = ensureInventoryState(p.inventoryId)
          st.loading = false
          st.refreshing = false
          st.characterStats = stats
          if (buffs) st.characterBuffs = buffs
          st.updatedAt = new Date().toLocaleTimeString('ru-RU')
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
            if (!table) throw new Error('нет таблицы опыта')
            levelTable.value = table
            return table
          })
          .finally(() => { levelTablePromise = null })
      }
      return levelTablePromise
    }
    async function loadSkillProgress(playerEntityId) {
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
          fetchBitjita(`/players/${playerEntityId}/skill-rankings`),
          ensureLevelTable(),
          loadXpHistory(playerEntityId),
        ])
        const rankings = res?.rankings
        if (!rankings) throw new Error('нет данных навыков')
        skillPinned.forEach((p) => {
          const st = ensureInventoryState(p.inventoryId)
          st.loading = false
          st.refreshing = false
          // Players without XP in a skill are absent from its ranking.
          st.skillRanking = rankings[String(p.skillId)] ?? { xp: 0, rank: null, totalPlayers: null }
          st.updatedAt = new Date().toLocaleTimeString('ru-RU')
        })
      } catch (e) {
        skillPinned.forEach((p) => {
          const st = ensureInventoryState(p.inventoryId)
          st.loading = false
          st.refreshing = false
          st.error = e.message
        })
      }
    }
    async function loadXpHistory(playerEntityId) {
      const id = String(playerEntityId)
      if (!xpHistory[id]) xpHistory[id] = { trackedSince: null, lastT: 0, bySkill: {} }
      const entry = xpHistory[id]
      try {
        const res = await fetchXpHistory(id, entry.lastT)
        entry.trackedSince = res.trackedSince || entry.trackedSince
        Object.entries(splitHistorySamples(res.samples)).forEach(([skill, points]) => {
          // Frozen: thousands of points, no need for deep reactivity.
          entry.bySkill[skill] = Object.freeze(mergeHistoryPoints(entry.bySkill[skill], points))
        })
        ;(res.samples ?? []).forEach((sample) => { entry.lastT = Math.max(entry.lastT, sample.t) })
      } catch {
        // History is optional; the block still works from skill-rankings.
      }
    }
    function skillHistory(pinItem) {
      return xpHistory[String(pinItem.playerEntityId)]?.bySkill[String(pinItem.skillId)] ?? null
    }
    function skillTrackedSince(pinItem) {
      return xpHistory[String(pinItem.playerEntityId)]?.trackedSince ?? null
    }
    /** Freshest XP: live socket, then the recorder, then skill-rankings (cached for a couple of minutes). */
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
        if (!wizard.skillOptions.length) wizard.error = 'Навыки не найдены'
      } catch (e) {
        wizard.error = e.message
      } finally {
        wizard.loadingSkills = false
      }
    }
    function addWizardSkill(skill) {
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
      if (wizard.blockType === 'market' || wizard.blockType === 'character' || wizard.blockType === 'skill' || wizard.sourceType === 'player') return result.signedIn ? 'онлайн' : 'офлайн'
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
      return inv.inventoryName || inv.buildingNickname || inv.buildingName || inv.houseName || 'Инвентарь'
    }
    function wizardPreviewPockets(inv) {
      const pockets = inv.pockets ?? inv.inventory ?? []
      return pockets.map(normalizePocket).filter((p) => hasContents(p))
    }
    function wizardPreviewQty(contents) {
      const n = Number(contents?.quantity ?? 0)
      if (!Number.isFinite(n)) return '0'
      if (n >= 10000) return `${Math.round(n / 1000)}k`
      return n.toLocaleString('ru-RU')
    }
    function wizardInventoryMeta(inv) {
      const parts = []
      if (inv.regionId) parts.push(`R${inv.regionId}`)
      if (inv.claimName) parts.push(inv.claimName)
      if (inv.houseName) parts.push(inv.houseName)
      if (inv.buildingName && inv.buildingName !== inv.inventoryName) parts.push(inv.buildingName)
      parts.push(`${wizardPreviewPockets(inv).length} заполн.`)
      return parts.join(' · ')
    }

    let timer = null
    let resizeObserver = null
    function updateDashboardCols() {
      if (!gridRef.value) return
      const containerWidth = gridRef.value.parentElement?.clientWidth ?? gridRef.value.clientWidth
      const viewportWidth = window.innerWidth - 32
      const width = Math.max(containerWidth, viewportWidth)
      const cell = 380
      const gap = 14
      dashboardCols.value = Math.max(1, Math.floor((width + gap) / (cell + gap)) || 1)
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
    onMounted(() => {
      updateDashboardCols()
      resizeObserver = new ResizeObserver(updateDashboardCols)
      if (gridRef.value) resizeObserver.observe(gridRef.value)
      loadAll()
      timer = setInterval(loadAll, 30000)
      mapUrlSyncTimer = setInterval(syncAllMapBlockUrls, 1000)
      initPlannerBlocks()
    })
    onUnmounted(() => {
      clearInterval(timer)
      clearInterval(mapUrlSyncTimer)
      clearTimeout(wizardSearchTimer)
      clearTimeout(wizardPlannerSearchTimer)
      resizeObserver?.disconnect()
      syncPlayerSubscriptions([])
      syncExperienceSubscriptions([])
      resetStaminaAlerts()
    })

    return {
      pinned, isPinned, unpin, state, dragState, wizard, wizardStepIndex, wizardSubtitle,
      gridRef, dashboardCols, dashboardUpdatedLabel, layoutCells,
      displayInventorySlots, dashboardCardClass, dashboardCardStyle, dashboardCellClass, dashboardCellStyle,
      inventoryGridStyle, hasInventoryGrid, hasContents, itemMeta, quantityDisplay, blockOwnerLabel,
      isCharacterBlock, isMarketBlock, isPlannerBlock, isMapBlock, mapBlockUrl,
      mapTileSize, setMapTileSize, setMapIframeRef, onMapIframeLoad, marketRows, marketTileSize, setMarketTileSize,
      marketDiffClass, marketDiffLabel, marketDeviationCount, marketRowDeviationClass,
      marketSearchQuery, setMarketSearchQuery, searchMarketItems, selectMarketSearchItem, clearMarketSearchItem,
      marketSearchSearching, marketSearchError, marketSearchResults, marketSearchSelected, marketSearchStats,
      marketSearchPriceLoading, marketPriceLines, blockIsEmpty,
      plannerTarget, plannerQuantity, plannerActiveRecipe, plannerBlockError,
      plannerHasRecipe, plannerMaterials, plannerMaterialRows, plannerTileSize, setPlannerTileSize,
      plannerLoading, formatPlannerQty, plannerItemMeta,
      openEditPlannerWizard, wizardPlannerCanConfirm, confirmWizardPlanner,
      setWizardPlannerQuery, selectWizardPlannerTarget, plannerRecipeOptionTitle, plannerRecipeOptionMeta,
      characterBars, characterBuffsLoaded, characterActiveBuffs, characterHasFoodBuff,
      isSkillBlock, skillBlockId, skillTileSize, setSkillTileSize, levelTable, skillGlyph,
      skillXp, skillHistory, skillTrackedSince, updatePinnedBlock,
      wizardSkillGroups, addWizardSkill,
      formatVitalsValue, vitalsBarPercent,
      loadAll, startDrag, dragOverCell, dropOnCell, endDrag,
      openAddWizard, closeAddWizard, chooseBlockType, chooseSourceType, searchWizard,
      selectWizardEntity, addWizardInventory, wizardBack, wizardResultTitle, wizardResultMeta,
      wizardInventoryGroups, wizardInventoryTitle, wizardInventoryMeta, wizardPreviewPockets, wizardPreviewQty,
    }
  },
}
</script>

<style scoped>
.pinned-dashboard {
  --dashboard-cell: 380px;
  --dashboard-row: 446px;
  --dash-slot: 38px;
  --dash-slot-height: 50px;
  --dash-gap: 3px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-width: 0;
}
.empty-state { padding: 40px 24px; text-align: center; }
.empty-title { font-size: 16px; font-weight: 600; color: var(--text); }
.empty-hint { font-size: 14px; color: var(--muted); line-height: 1.5; }
.dash-toolbar { display: flex; justify-content: space-between; align-items: center; padding: 0 2px; }
.dash-hint { font-size: 13px; color: var(--muted); }
.dash-actions { display: flex; gap: 8px; align-items: center; }
.refresh-btn {
  font-size: 14px;
  cursor: pointer;
  color: var(--muted);
  background: none;
  border: none;
  padding: 4px 8px;
  border-radius: 6px;
  transition: color 0.15s;
}
.refresh-btn:hover { color: var(--accent); }
.refresh-btn:disabled { opacity: 0.45; cursor: default; }
.add-block-btn {
  padding: 7px 12px;
  border: 1px solid var(--accent);
  border-radius: 7px;
  background: rgba(138, 164, 255, 0.16);
  color: var(--text);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}
.dashboard-grid {
  display: grid;
  grid-template-columns: repeat(var(--dashboard-cols), var(--dashboard-cell));
  grid-auto-rows: var(--dashboard-row);
  gap: 14px;
  align-items: stretch;
  width: calc((var(--dashboard-cell) * var(--dashboard-cols)) + (14px * (var(--dashboard-cols) - 1)));
  max-width: none;
}
.dashboard-cell {
  min-width: 0;
  min-height: 0;
  border: 1px dashed transparent;
  border-radius: 10px;
  background: transparent;
  transition: border-color 0.15s, background 0.15s, box-shadow 0.15s;
}
.dashboard-grid--dragging .dashboard-cell { border-color: rgba(106, 104, 126, 0.38); background: rgba(37, 36, 55, 0.16); }
.dashboard-cell--occupied { pointer-events: none; opacity: 0; }
.dashboard-cell--drop-ok { border-color: var(--accent); background: rgba(138, 164, 255, 0.16); box-shadow: inset 0 0 0 2px rgba(138, 164, 255, 0.2); }
.dashboard-cell--drop-blocked { border-color: var(--danger); background: rgba(255, 138, 154, 0.12); }
.inv-card {
  z-index: 1;
  width: auto;
  height: 100%;
  min-width: 0;
  max-width: none;
  padding: 14px 18px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  transition: border-color 0.15s, box-shadow 0.15s, opacity 0.15s, transform 0.15s;
}
.inv-card--dragging { opacity: 0.55; transform: scale(0.995); }
.inv-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; cursor: grab; user-select: none; }
.inv-header:active { cursor: grabbing; }
.inv-header-actions { display: flex; align-items: center; gap: 8px; }
.tile-size-switch { display: inline-flex; border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
.tile-size-btn { border: none; background: var(--surface-raised); color: var(--muted); font-size: 11px; font-weight: 600; padding: 3px 7px; cursor: pointer; }
.tile-size-btn.active { background: rgba(138, 164, 255, 0.2); color: var(--accent); }
.inv-meta { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.inv-name { font-size: 14px; font-weight: 600; color: var(--text); }
.inv-region { font-size: 11px; background: rgba(138, 164, 255, 0.18); color: var(--accent); padding: 1px 7px; border-radius: 8px; font-weight: 500; }
.inv-claim { font-size: 11px; background: rgba(117, 224, 167, 0.16); color: var(--success); padding: 1px 7px; border-radius: 8px; }
.inv-player { font-size: 12px; color: var(--muted); font-style: italic; }
.market-dev-badge { font-size: 11px; font-weight: 700; padding: 1px 7px; border-radius: 8px; }
.market-dev-badge--above { background: rgba(255, 138, 154, 0.18); color: var(--danger); }
.market-dev-badge--below { background: rgba(117, 224, 167, 0.16); color: var(--success); }
.unpin-btn { border: none; background: none; font-size: 14px; color: var(--muted); cursor: pointer; padding: 2px 6px; border-radius: 5px; }
.unpin-btn:hover { color: var(--danger); background: rgba(255, 138, 154, 0.12); }
.edit-block-btn {
  border: none;
  background: none;
  font-size: 14px;
  color: var(--muted);
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 5px;
  line-height: 1;
}
.edit-block-btn:hover { color: var(--accent); background: rgba(138, 164, 255, 0.14); }
.inventory-board { flex: 1; min-height: 0; padding: 8px; background: #252437; border: 2px solid #35344a; border-radius: 10px; width: fit-content; max-width: 100%; overflow: hidden; }
.inventory-grid { display: grid; grid-template-columns: repeat(var(--inventory-cols), var(--dash-slot)); gap: var(--dash-gap); justify-content: start; width: max-content; max-width: 100%; }
.inventory-slot { position: relative; height: var(--dash-slot-height); display: flex; align-items: center; justify-content: center; border-radius: 9px; background: linear-gradient(145deg, #4a4960, #2c2b3e); box-shadow: inset 0 0 0 2px #6a687e, inset 0 0 0 4px #2f2e43, 0 1px 2px rgba(0, 0, 0, 0.35); }
.inventory-slot--item { width: var(--dash-slot); }
.inventory-slot--cargo { width: auto; grid-column: span 2; }
.inventory-slot--empty { background: rgba(37, 36, 55, 0.35); border: 3px solid #6a687e; opacity: 1; box-shadow: inset 0 0 0 2px #2f2e43, 0 1px 2px rgba(0, 0, 0, 0.35); }
.slot-qty { position: absolute; right: 5px; bottom: 5px; color: #f8fafc; font-size: 13px; font-weight: 800; line-height: 1; text-shadow: -1px -1px 0 rgba(0,0,0,.8), 1px -1px 0 rgba(0,0,0,.8), -1px 1px 0 rgba(0,0,0,.8), 1px 1px 0 rgba(0,0,0,.8); pointer-events: none; }
.inventory-slot :deep(.item-card--mini .item-card__icon) { width: 34px; height: 44px; flex-basis: 34px; border-radius: 7px; }
.inventory-slot--cargo :deep(.item-card--mini .item-card__icon) { width: 74px; flex-basis: 74px; }
.inventory-slot :deep(.item-card--mini.item-card--rarity .item-card__icon) { border-width: 3px; }
.inventory-slot :deep(.item-card--mini .item-card__fallback) { font-size: 9px; }
.inventory-slot :deep(.item-card--mini .item-card__image) { width: 84%; height: 84%; }
.market-block-body {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.market-orders {
  flex: 1;
  min-height: 0;
  overflow: auto;
}
.market-item-search {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 6px 8px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: rgba(37, 36, 55, 0.45);
}
.market-item-search-form {
  display: flex;
  gap: 6px;
  align-items: center;
}
.market-item-search-input {
  flex: 1;
  min-width: 0;
  padding: 5px 8px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--surface-raised);
  color: var(--text);
  font-size: 12px;
  outline: none;
}
.market-item-search-input:focus { border-color: var(--accent); }
.market-item-search-btn {
  padding: 5px 10px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--surface-raised);
  color: var(--text-soft);
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
}
.market-item-search-btn:disabled { opacity: 0.5; cursor: default; }
.market-item-search-error {
  font-size: 11px;
  color: var(--danger);
}
.market-item-search-results {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 140px;
  overflow: auto;
}
.market-item-search-result {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 6px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--surface-raised);
  color: var(--text);
  cursor: pointer;
  text-align: left;
}
.market-item-search-result:hover { border-color: var(--accent); background: var(--surface-hover); }
.market-item-search-result :deep(.item-card__body) { display: none; }
.market-item-search-result :deep(.item-card--mini .item-card__icon) {
  width: 22px;
  height: 28px;
  flex-basis: 22px;
  border-radius: 5px;
}
.market-item-search-result-name {
  font-size: 12px;
  line-height: 1.25;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.market-item-price-loading,
.market-item-price-empty {
  font-size: 11px;
  color: var(--muted);
  font-style: italic;
}
.market-item-price-card {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 2px;
}
.market-item-price-card :deep(.item-card__body) { display: none; }
.market-item-price-card :deep(.item-card--mini .item-card__icon) {
  width: 28px;
  height: 34px;
  flex-basis: 28px;
  border-radius: 6px;
}
.market-item-price-lines {
  flex: 1;
  min-width: 0;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 2px 10px;
}
.market-item-price-line {
  display: flex;
  align-items: baseline;
  gap: 4px;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.market-item-price-value {
  color: var(--text);
  font-weight: 600;
}
.market-item-price-clear {
  border: none;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
  padding: 2px 5px;
  border-radius: 4px;
  font-size: 12px;
  line-height: 1;
}
.market-item-price-clear:hover { color: var(--danger); background: rgba(255, 138, 154, 0.12); }
.market-list { display: flex; flex-direction: column; gap: 4px; padding: 2px 0; overflow-y: auto; overflow-x: hidden; }
.market-list--small { display: grid; grid-template-columns: 1fr 1fr; gap: 4px; }
.market-row { display: flex; align-items: center; gap: 6px; border-radius: 7px; border: 1px solid var(--border); background: var(--surface-raised); padding: 5px 7px; }
.market-row--dev-above { background: rgba(255, 138, 154, 0.10) !important; border-color: rgba(255, 138, 154, 0.30) !important; }
.market-row--dev-below { background: rgba(117, 224, 167, 0.10) !important; border-color: rgba(117, 224, 167, 0.28) !important; }
.market-row--small :deep(.item-card__body) { display: none; }
.market-row--small { flex-direction: row; align-items: center; gap: 4px; padding: 3px 5px; }
.market-row--small :deep(.item-card--mini .item-card__icon) { width: 24px; height: 30px; flex-basis: 24px; border-radius: 5px; }
.market-row--small :deep(.item-card--mini .item-card__image) { width: 88%; height: 88%; }
.market-list--large { display: grid; grid-template-columns: 1fr; gap: 4px; }
.market-row--large { display: grid; grid-template-columns: 56px 2fr 1fr 1fr; gap: 10px; align-items: center; padding: 7px 10px; }
.market-row--large :deep(.item-card__body) { display: none; }
.market-row--large :deep(.item-card--mini .item-card__icon) { width: 46px; height: 56px; flex-basis: 46px; border-radius: 9px; }
.market-row--large :deep(.item-card--mini .item-card__image) { width: 86%; height: 86%; }
.market-large-name { flex: 1; min-width: 0; font-size: 14px; font-weight: 600; color: var(--text); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.market-large-values { display: flex; flex-direction: column; gap: 3px; min-width: 0; align-self: center; }
.market-large-line { font-size: 13px; font-variant-numeric: tabular-nums; white-space: nowrap; line-height: 1.35; }
.market-list--large .market-small-label { font-size: 12px; }
.market-list--large .market-small-pct { font-size: 11px; }
.market-list--large .market-qty { font-size: 11px; right: 2px; bottom: 2px; }
.market-list--large .market-lot-location { font-size: 11px; max-width: 120px; }
.market-list--large .market-lot-total { font-size: 14px; }
.market-list--large .market-lot-block { min-width: 0; gap: 2px; align-items: flex-start; }
.market-icon-wrap { position: relative; display: inline-flex; flex-shrink: 0; align-items: center; justify-content: center; }
.market-qty {
  position: absolute;
  right: 1px;
  bottom: 1px;
  color: #f8fafc;
  font-size: 9px;
  font-weight: 800;
  line-height: 1;
  text-shadow: -1px -1px 0 rgba(0,0,0,.85), 1px -1px 0 rgba(0,0,0,.85), -1px 1px 0 rgba(0,0,0,.85), 1px 1px 0 rgba(0,0,0,.85);
  pointer-events: none;
}
.market-small-values { display: flex; flex-direction: column; gap: 1px; min-width: 0; flex: 1; overflow: hidden; }
.market-small-line { font-size: 10px; font-variant-numeric: tabular-nums; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; line-height: 1.3; }
.market-small-label { color: var(--muted); font-weight: 500; }
.market-small-pct { font-size: 9px; font-weight: 700; opacity: 0.9; }
.market-lot-block { flex-shrink: 0; display: flex; flex-direction: column; align-items: flex-end; gap: 1px; min-width: 52px; }
.market-lot-location { font-size: 9px; color: var(--muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 90px; line-height: 1.2; }
.market-lot-total { font-size: 10px; font-variant-numeric: tabular-nums; font-weight: 700; color: var(--text); text-align: right; line-height: 1.2; }
.market-amount--ref { color: var(--muted); }
.market-diff--above { color: var(--danger); }
.market-diff--below { color: var(--success); }
.market-diff--neutral { color: var(--muted); }
.planner-block {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  overflow: hidden;
}
.planner-search {
  display: flex;
  gap: 6px;
  align-items: center;
}
.planner-search-input {
  flex: 1;
  min-width: 0;
  padding: 5px 8px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--surface-raised);
  color: var(--text);
  font-size: 12px;
  outline: none;
}
.planner-search-input:focus { border-color: var(--accent); }
.planner-searching {
  flex-shrink: 0;
  font-size: 11px;
  color: var(--muted);
  min-width: 12px;
}
.planner-error { font-size: 11px; color: var(--danger); }
.planner-search-results {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 120px;
  overflow: auto;
}
.planner-search-result {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 6px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--surface-raised);
  color: var(--text);
  cursor: pointer;
  text-align: left;
}
.planner-search-result:hover { border-color: var(--accent); background: var(--surface-hover); }
.planner-search-result :deep(.item-card__body) { display: none; }
.planner-search-result :deep(.item-card--mini .item-card__icon) {
  width: 22px;
  height: 28px;
  flex-basis: 22px;
  border-radius: 5px;
}
.planner-search-result-name {
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.planner-target {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 6px 8px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: rgba(138, 164, 255, 0.08);
}
.planner-target--readonly {
  background: rgba(138, 164, 255, 0.06);
}
.planner-target-qty {
  font-size: 12px;
  font-weight: 700;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}
.wizard-planner-results {
  max-height: 160px;
}
.wizard-planner-target {
  margin-top: 4px;
}
.planner-search-result--active {
  border-color: var(--accent);
  background: rgba(138, 164, 255, 0.12);
}
.modal-actions .add-block-btn { margin-right: auto; }
.planner-target-head {
  display: flex;
  align-items: center;
  gap: 8px;
}
.planner-target-head :deep(.item-card__body) { display: none; }
.planner-target-head :deep(.item-card--mini .item-card__icon) {
  width: 28px;
  height: 34px;
  flex-basis: 28px;
  border-radius: 6px;
}
.planner-target-meta {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.planner-target-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.planner-qty-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--muted);
}
.planner-qty-input {
  width: 72px;
  padding: 3px 6px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--surface-raised);
  color: var(--text);
  font-size: 12px;
}
.planner-target-clear {
  border: none;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
  padding: 2px 5px;
  border-radius: 4px;
}
.planner-target-clear:hover { color: var(--danger); background: rgba(255, 138, 154, 0.12); }
.planner-recipe-pick {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
}
.planner-recipes {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.planner-recipes-label {
  font-size: 10px;
  font-weight: 600;
  color: var(--muted);
  letter-spacing: 0.04em;
  text-transform: uppercase;
}
.planner-recipes-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.planner-recipe-option {
  flex: 1 1 148px;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 6px 8px;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: var(--surface-raised);
  color: var(--text);
  cursor: pointer;
  text-align: left;
  transition: border-color 0.15s, background 0.15s, box-shadow 0.15s;
}
.planner-recipe-option:hover {
  border-color: var(--accent);
  background: var(--surface-hover);
}
.planner-recipe-option--active {
  border-color: var(--accent);
  background: rgba(138, 164, 255, 0.12);
  box-shadow: inset 0 0 0 1px rgba(138, 164, 255, 0.28);
}
.planner-recipe-option-title {
  font-size: 11px;
  font-weight: 600;
  line-height: 1.35;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.planner-recipe-option-meta {
  font-size: 10px;
  color: var(--muted);
  line-height: 1.35;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.planner-recipe-label { color: var(--muted); }
.planner-recipe-select {
  flex: 1;
  min-width: 0;
  padding: 4px 6px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--surface-raised);
  color: var(--text);
  font-size: 11px;
}
.planner-recipe-hint {
  font-size: 11px;
  color: var(--muted);
  line-height: 1.35;
}
.planner-loading,
.planner-hint {
  font-size: 12px;
  color: var(--muted);
  font-style: italic;
}
.planner-materials--tree {
  overflow: auto;
}
.planner-material-row--nested {
  border-color: rgba(138, 164, 255, 0.12);
}
.planner-material-row--leaf .planner-material-name {
  color: var(--muted);
}
.planner-tree-guide {
  flex-shrink: 0;
  width: 12px;
  font-size: 11px;
  line-height: 1;
  color: var(--muted);
  opacity: 0.65;
}
.planner-materials {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
  overflow: auto;
  min-height: 0;
}
.planner-material-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 8px;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: var(--surface-raised);
}
.planner-material-row :deep(.item-card__body) { display: none; }
.planner-material-row :deep(.item-card--mini .item-card__icon) {
  width: 26px;
  height: 32px;
  flex-basis: 26px;
  border-radius: 6px;
}
.planner-material-name {
  flex: 1;
  min-width: 0;
  font-size: 12px;
  color: var(--text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.planner-material-qty {
  font-size: 13px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  color: var(--accent);
}
.empty-inv, .placeholder { font-size: 13px; color: var(--muted); font-style: italic; }
.error-msg, .wizard-error { font-size: 13px; color: var(--danger); }
.modal-backdrop { position: fixed; inset: 0; z-index: 50; display: flex; align-items: center; justify-content: center; padding: 24px; background: rgba(10, 9, 18, 0.72); backdrop-filter: blur(4px); }
.modal-card { width: min(720px, 100%); max-height: min(760px, calc(100vh - 48px)); display: flex; flex-direction: column; padding: 18px; background: var(--surface); border: 1px solid var(--border); border-radius: 14px; box-shadow: 0 24px 80px rgba(0,0,0,.45); }
.modal-head { display: flex; justify-content: space-between; gap: 16px; align-items: flex-start; }
.modal-title { margin: 0; font-size: 18px; color: var(--text); }
.modal-subtitle { margin: 5px 0 0; font-size: 13px; color: var(--muted); }
.modal-close { border: none; background: transparent; color: var(--muted); cursor: pointer; border-radius: 6px; padding: 4px 7px; }
.wizard-steps { display: flex; gap: 8px; margin: 18px 0 14px; flex-wrap: wrap; }
.wizard-step { padding: 4px 9px; border-radius: 999px; background: var(--surface-raised); color: var(--muted); font-size: 12px; font-weight: 600; }
.wizard-step.active { background: rgba(138, 164, 255, 0.2); color: var(--accent); }
.wizard-step.done { color: var(--success); }
.wizard-panel { min-height: 220px; overflow: auto; display: flex; flex-direction: column; gap: 10px; padding-right: 4px; }
.choice-card, .wizard-result, .wizard-inventory { width: 100%; text-align: left; padding: 12px 14px; border: 1px solid var(--border); border-radius: 10px; background: var(--surface-raised); color: var(--text); cursor: pointer; }
.choice-card:hover, .wizard-result:hover, .wizard-inventory:hover:not(:disabled) { border-color: var(--accent); background: var(--surface-hover); }
.choice-card.active { border-color: var(--accent); background: rgba(138, 164, 255, 0.14); }
.choice-title, .result-title { display: block; font-size: 14px; font-weight: 700; color: var(--text); }
.choice-text, .result-meta { display: block; margin-top: 4px; font-size: 12px; color: var(--muted); line-height: 1.4; }
.wizard-search { display: flex; align-items: center; gap: 8px; }
.wizard-search .wizard-input { flex: 1; min-width: 0; }
.wizard-searching-hint {
  flex-shrink: 0;
  font-size: 12px;
  color: var(--muted);
  min-width: 12px;
}
.wizard-input { flex: 1; min-width: 0; padding: 8px 11px; border: 1px solid var(--border); border-radius: 7px; background: var(--surface-raised); color: var(--text); outline: none; }
.wizard-results, .wizard-inventory-list { display: flex; flex-direction: column; gap: 8px; }
.wizard-inventory-group { display: flex; flex-direction: column; gap: 8px; }
.wizard-inventory-group + .wizard-inventory-group { margin-top: 12px; }
.wizard-inventory-group-title {
  margin: 0;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--muted);
}
.wizard-inventory:disabled { opacity: 0.5; cursor: default; }
.wizard-skill { display: grid; grid-template-columns: 24px 1fr auto 64px; align-items: center; gap: 10px; padding: 9px 14px; }
.wizard-skill-glyph { font-size: 16px; text-align: center; }
.wizard-skill-name { min-width: 0; }
.wizard-skill-name .result-meta { margin-top: 1px; }
.wizard-skill-level { font-size: 13px; font-weight: 700; color: var(--text); font-variant-numeric: tabular-nums; }
.wizard-skill-rank { font-size: 12px; color: var(--muted); text-align: right; font-variant-numeric: tabular-nums; }
.wizard-inventory-head { display: flex; flex-direction: column; gap: 2px; }
.wizard-inventory-preview {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
  margin-top: 8px;
  padding-top: 2px;
}
.wizard-preview-slot {
  position: relative;
  flex-shrink: 0;
}
.wizard-preview-slot :deep(.item-card__body) { display: none; }
.wizard-preview-slot :deep(.item-card--mini .item-card__icon) {
  width: 22px;
  height: 28px;
  flex-basis: 22px;
  border-radius: 4px;
}
.wizard-preview-slot :deep(.item-card--mini .item-card__image) { width: 88%; height: 88%; }
.wizard-preview-slot :deep(.item-card--mini.item-card--rarity .item-card__icon) { border-width: 2px; }
.wizard-preview-qty {
  position: absolute;
  right: 0;
  bottom: 0;
  color: #f8fafc;
  font-size: 8px;
  font-weight: 800;
  line-height: 1;
  text-shadow: -1px -1px 0 rgba(0, 0, 0, 0.85), 1px -1px 0 rgba(0, 0, 0, 0.85), -1px 1px 0 rgba(0, 0, 0, 0.85), 1px 1px 0 rgba(0, 0, 0, 0.85);
  pointer-events: none;
}
.modal-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px; padding-top: 14px; border-top: 1px solid var(--border); }
.ghost-btn { padding: 7px 12px; border: 1px solid var(--border); border-radius: 7px; background: var(--surface-raised); color: var(--text-soft); cursor: pointer; }
@media (max-width: 840px) {
  .pinned-dashboard { --dashboard-cell: calc(100vw - 32px); --dashboard-row: 446px; }
  .inv-card--large { grid-column: span 1; }
}

/* ──────────────────────────────────────────
   Map block
   ────────────────────────────────────────── */
.map-block {
  flex: 1;
  min-height: 0;
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid var(--border);
  background: #0c0b14;
}
.map-iframe {
  width: 100%;
  height: 100%;
  min-height: 360px;
  border: none;
  display: block;
}
.map-block--small,
.map-iframe--small {
  min-height: 0;
}
.map-iframe--small {
  min-height: 280px;
}

/* ──────────────────────────────────────────
   Character stats block
   ────────────────────────────────────────── */
.char-stats {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 10px 4px 4px;
  justify-content: center;
  border-radius: 10px;
  transition: background 0.2s, border-color 0.2s;
}
.char-stats--no-food {
  padding: 10px 8px 8px;
  border: 1px solid rgba(248, 113, 113, 0.5);
  background: rgba(248, 113, 113, 0.08);
}
.char-stats--no-food .char-bar-group--satiation .char-bar-track {
  border-color: rgba(248, 113, 113, 0.55);
  box-shadow: inset 0 0 0 1px rgba(248, 113, 113, 0.2);
}
.char-food-warn {
  font-size: 10px;
  font-weight: 700;
  color: #fca5a5;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  margin-right: auto;
  padding: 2px 6px;
  border-radius: 6px;
  background: rgba(248, 113, 113, 0.15);
}
.char-buffs {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-height: 0;
}
.char-buffs-title {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--muted);
}
.char-buffs-empty {
  font-size: 12px;
  color: var(--muted);
  font-style: italic;
}
.char-buff-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 120px;
  overflow-y: auto;
}
.char-buff-item {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 8px;
  padding: 5px 8px;
  border-radius: 7px;
  border: 1px solid var(--border);
  background: var(--surface-raised);
  font-size: 12px;
  line-height: 1.3;
}
.char-buff-item--food {
  border-color: rgba(74, 222, 128, 0.35);
  background: rgba(74, 222, 128, 0.08);
}
.char-buff-item--bad {
  border-color: rgba(248, 113, 113, 0.35);
  background: rgba(248, 113, 113, 0.08);
}
.char-buff-name {
  color: var(--text-soft);
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.char-buff-time {
  flex-shrink: 0;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  color: var(--muted);
  font-weight: 600;
}
.char-bar-group {
  display: flex;
  flex-direction: column;
  gap: 7px;
}
.char-bar-header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 8px;
}
.char-bar-label {
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.01em;
}
.char-bar-label--hp { color: #f87171; }
.char-bar-label--stamina { color: #fbbf24; }
.char-bar-label--satiation { color: #4ade80; }
.char-bar-value {
  font-size: 22px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  color: var(--text);
  line-height: 1;
}
.char-bar-track {
  height: 12px;
  border-radius: 999px;
  background: var(--surface-raised);
  border: 1px solid var(--border);
  overflow: hidden;
}
.char-bar-fill {
  height: 100%;
  border-radius: 999px;
  transition: width 0.5s ease;
}
.char-bar-fill--hp {
  background: linear-gradient(90deg, #ef4444, #f87171);
  box-shadow: 0 0 8px rgba(248, 113, 113, 0.45);
}
.char-bar-fill--stamina {
  background: linear-gradient(90deg, #d97706, #fbbf24);
  box-shadow: 0 0 8px rgba(251, 191, 36, 0.45);
}
.char-bar-fill--satiation {
  background: linear-gradient(90deg, #16a34a, #4ade80);
  box-shadow: 0 0 8px rgba(74, 222, 128, 0.45);
}
.char-meta {
  margin-top: auto;
  display: flex;
  justify-content: flex-end;
}
.char-meta-updated {
  font-size: 11px;
  color: var(--muted);
}
</style>
