<template>
  <div
    class="inventory-grid"
    :class="{ 'inventory-grid--arranging': arrangeable, 'inventory-grid--fill': fill }"
    :style="{ '--slot-size': `${slotPx}px`, '--inventory-half-cols': columns * 2 }"
  >
    <div ref="tray" class="inventory-tray">
      <div class="inventory-panels">
        <div v-for="(panel, p) in panels" :key="p" class="inventory-panel">
          <div
            v-for="part in panel.parts"
            :key="`${part.kind}-${part.offset}`"
            class="inventory-section"
            :class="`inventory-section--${part.kind}`"
          >
            <div
              v-for="(pocket, i) in part.pockets"
              :key="part.offset + i"
              class="inventory-slot"
              :class="{
                'inventory-slot--empty': !pocket,
                'inventory-slot--dragged': drag.kind === part.kind && drag.from === part.offset + i,
                'inventory-slot--target': drag.kind === part.kind && drag.over === part.offset + i,
              }"
              :style="rowStart(part, i)"
              :draggable="arrangeable && Boolean(pocket)"
              @dragstart="onDragStart(part.kind, part.offset + i, $event)"
              @dragover="onDragOver(part.kind, part.offset + i, $event)"
              @drop.prevent="onDrop(part.kind, part.offset + i)"
              @dragend="endDrag"
            >
              <template v-if="pocket">
                <ItemCard
                  variant="mini"
                  badge
                  :name="resolveItem(pocket.contents).name"
                  :icon-asset-name="resolveItem(pocket.contents).iconAssetName"
                  :item-type="resolveItem(pocket.contents).type"
                  :tier="resolveItem(pocket.contents).tier"
                  :rarity-str="resolveItem(pocket.contents).rarityStr"
                  :hint="`× ${formatNumber(pocket.contents.quantity ?? 0)}`"
                />
                <span v-if="pocket.contents.quantity > 1" class="slot-qty">{{ compactQuantity(pocket.contents.quantity) }}</span>
              </template>
            </div>
          </div>
        </div>
      </div>
    </div>
    <div v-if="stats.length" class="inventory-foot">
      <span v-for="stat in stats" :key="stat.kind" class="inventory-stat">
        <span class="inventory-stat__label">{{ stat.label }}</span>
        <span class="inventory-stat__bar"><span :style="{ width: `${stat.share * 100}%` }"></span></span>
        <span class="inventory-stat__count">{{ stat.used }}<span class="inventory-stat__of">/{{ stat.total }}</span></span>
      </span>
      <span v-if="live" class="inventory-live" :title="t('inventoryGrid.liveTitle')">{{ t('inventoryGrid.live') }}</span>
    </div>
  </div>
</template>

<script>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { formatNumber, t } from '../i18n'
import { useItemCache } from '../composables/useItemCache'
import { compactQuantity, GAME_COLS, inventoryLayout, inventoryPanels, moveInSection } from '../inventoryLayout'
import ItemCard from './ItemCard.vue'

const FIXED_SLOT_PX = 34

// The pockets of one storage in whole game rows (ten pockets; five or twenty when that fills
// the space better): item pockets, then cargo pockets.
// With `fill` the pockets grow to use the space the grid is given.
export default {
  name: 'InventoryGrid',
  components: { ItemCard },
  props: {
    // From inventorySections(): { items, cargo }, each an array of pockets and nulls.
    sections: { type: Object, required: true },
    // Items and cargo in panels side by side (a lone section split in two) instead of stacked.
    split: { type: Boolean, default: false },
    fill: { type: Boolean, default: true },
    // Stacks can be dragged to other pockets; emits "arrange" with { kind, moves }.
    arrangeable: { type: Boolean, default: false },
    // The pockets come straight from the game's database replica.
    live: { type: Boolean, default: false },
  },
  emits: ['arrange'],
  setup(props, { emit }) {
    const { resolveItem } = useItemCache()
    const tray = ref(null)
    const box = reactive({ width: 0, height: 0 })
    const layout = computed(() => {
      if (props.fill && box.width && box.height) return inventoryLayout(props.sections, props.split, box.width, box.height)
      return { cols: GAME_COLS, panels: inventoryPanels(props.sections, props.split), slot: FIXED_SLOT_PX }
    })
    const columns = computed(() => layout.value.cols)
    const panels = computed(() => layout.value.panels)
    const slotPx = computed(() => Math.floor(layout.value.slot * 10) / 10)
    const stats = computed(() => [
      { kind: 'item', pockets: props.sections.items },
      { kind: 'cargo', pockets: props.sections.cargo },
    ].filter(({ pockets }) => pockets.length).map(({ kind, pockets }) => {
      const used = pockets.filter(Boolean).length
      return { kind, label: t(`inventoryGrid.${kind}`), used, total: pockets.length, share: used / pockets.length }
    }))

    let observer = null
    onMounted(() => {
      if (!props.fill || !tray.value) return
      observer = new ResizeObserver(([entry]) => {
        box.width = entry.contentRect.width
        box.height = entry.contentRect.height
      })
      observer.observe(tray.value)
    })
    onBeforeUnmount(() => observer?.disconnect())

    // The grid runs on half-pocket columns so a short last row can sit in the middle, as in
    // game: an item pocket spans two of them, a cargo pocket four.
    function rowStart(part, index) {
      const span = part.kind === 'cargo' ? 4 : 2
      const perRow = Math.max(1, Math.floor((columns.value * 2) / span))
      const count = part.pockets.length
      const last = count % perRow
      if (!last || index !== count - last) return null
      return { gridColumnStart: ((perRow - last) * span) / 2 + 1 }
    }

    const drag = reactive({ kind: null, from: -1, over: -1 })
    function sectionPockets(kind) {
      return kind === 'cargo' ? props.sections.cargo : props.sections.items
    }
    function movesFor(kind, index) {
      if (drag.kind !== kind) return null
      return moveInSection(sectionPockets(kind), drag.from, index)
    }
    function onDragStart(kind, index, event) {
      if (!props.arrangeable) return
      drag.kind = kind
      drag.from = index
      event.dataTransfer.effectAllowed = 'move'
      event.dataTransfer.setData('text/plain', `${kind}:${index}`)
    }
    function onDragOver(kind, index, event) {
      if (!movesFor(kind, index)) {
        if (drag.kind === kind) drag.over = -1
        return
      }
      event.preventDefault()
      event.dataTransfer.dropEffect = 'move'
      drag.over = index
    }
    function onDrop(kind, index) {
      const moves = movesFor(kind, index)
      endDrag()
      if (moves) emit('arrange', { kind, moves })
    }
    function endDrag() {
      drag.kind = null
      drag.from = -1
      drag.over = -1
    }

    return {
      tray, columns, panels, slotPx, stats, t, rowStart, resolveItem, formatNumber, compactQuantity,
      drag, onDragStart, onDragOver, onDrop, endDrag,
    }
  },
}
</script>

<style scoped>
.inventory-grid {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
  min-height: 0;
}
.inventory-grid--fill {
  flex: 1;
  width: 100%;
}
/* A recessed board like the game's storage window: a soft glow from the top and a faint
   honeycomb that fades towards the edges, so the room around the pockets reads as part of it. */
.inventory-tray {
  position: relative;
  display: flex;
  min-height: 0;
  padding: 8px 6px;
  border: 1px solid rgba(233, 223, 196, 0.1);
  border-radius: 4px;
  background:
    radial-gradient(120% 70% at 50% 0%, rgba(74, 128, 186, 0.16), transparent 70%),
    linear-gradient(180deg, rgba(14, 13, 22, 0.35), rgba(14, 13, 22, 0.55));
  box-shadow: inset 0 1px 6px rgba(0, 0, 0, 0.45);
  overflow: hidden;
}
.inventory-tray::before {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='30' height='52'%3E%3Cpath d='M0 8.66L15 0l15 8.66v17.32L15 34.64 0 25.98zM15 34.64V52' fill='none' stroke='%23e9dfc4' stroke-opacity='0.07'/%3E%3C/svg%3E");
  background-size: 30px 52px;
  background-position: center;
  -webkit-mask-image: radial-gradient(closest-side, transparent 35%, #000 100%);
  mask-image: radial-gradient(closest-side, transparent 35%, #000 100%);
}
.inventory-grid--fill .inventory-tray { flex: 1; }
.inventory-panels {
  position: relative;
  flex: 1;
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  align-content: center;
  align-items: flex-start;
  gap: 14px;
  min-width: 0;
}
.inventory-foot {
  display: flex;
  align-items: center;
  gap: 14px;
  height: 16px;
  padding: 0 2px;
  font-family: var(--font-title);
  font-size: 10px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--muted);
}
.inventory-stat { display: inline-flex; align-items: center; gap: 6px; min-width: 0; }
.inventory-stat__bar {
  position: relative;
  width: 44px;
  height: 3px;
  border-radius: 2px;
  background: rgba(233, 223, 196, 0.12);
  overflow: hidden;
}
.inventory-stat__bar > span { position: absolute; inset: 0 auto 0 0; border-radius: 2px; background: var(--cream); opacity: 0.75; }
.inventory-stat__count { color: var(--text-soft); font-variant-numeric: tabular-nums; letter-spacing: 0.04em; }
.inventory-stat__of { color: var(--muted); }
.inventory-live {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: var(--success);
}
.inventory-live::before {
  content: '';
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
  box-shadow: 0 0 6px currentColor;
}
.inventory-panel {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.inventory-section {
  display: grid;
  grid-template-columns: repeat(var(--inventory-half-cols), calc((var(--slot-size) + 3px) / 2));
  row-gap: 3px;
}
.inventory-section + .inventory-section {
  padding-top: 4px;
  border-top: 1px solid var(--frame-soft);
}
.inventory-slot {
  position: relative;
  grid-column-end: span 2;
  margin: 0 1.5px;
  height: calc(var(--slot-size) * 64 / 52);
  display: flex;
  border-radius: 4px;
}
.inventory-slot--empty {
  border: 1px solid var(--slot-edge);
  background: var(--slot);
  box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.3), inset 0 4px 6px rgba(0, 0, 0, 0.18);
}
.inventory-section--cargo .inventory-slot { grid-column-end: span 4; }
.inventory-section--cargo .inventory-slot--empty { border-style: dashed; }
.inventory-slot :deep(.item-card--mini) { width: 100%; height: 100%; }
.inventory-slot :deep(.item-card--mini .item-card__icon),
.inventory-slot :deep(.item-card--mini.item-card--cargo .item-card__icon) {
  width: 100%;
  height: 100%;
  flex-basis: 100%;
  border-width: 1px;
  border-radius: 4px;
}
.inventory-slot :deep(.item-card__badge) {
  min-width: 0;
  padding: 0 2px 0 1px;
  border-radius: 0 0 3px 0;
  font-size: clamp(7px, calc(var(--slot-size) * 0.22), 10px);
  line-height: 1.15;
}
.inventory-grid--arranging .inventory-slot:not(.inventory-slot--empty) { cursor: grab; }
.inventory-grid--arranging .inventory-slot :deep(img) { pointer-events: none; }
.inventory-grid--arranging .inventory-slot--empty { border-color: var(--frame); }
.inventory-slot--dragged { opacity: 0.35; }
.inventory-slot--target { outline: 2px solid var(--cream); outline-offset: 1px; }
.slot-qty {
  position: absolute;
  right: 3px;
  bottom: 2px;
  color: #fff;
  font-size: clamp(10px, calc(var(--slot-size) * 0.3), 13px);
  font-weight: 800;
  line-height: 1;
  font-variant-numeric: tabular-nums;
  text-shadow: -1px -1px 0 #000, 1px -1px 0 #000, -1px 1px 0 #000, 1px 1px 0 #000;
  pointer-events: none;
}
</style>
