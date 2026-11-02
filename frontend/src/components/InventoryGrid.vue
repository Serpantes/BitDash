<template>
  <div
    class="inventory-grid"
    :class="{ 'inventory-grid--arranging': arrangeable, 'inventory-grid--side': sideBySide }"
    :style="{ '--inventory-cols': columns, '--inventory-half-cols': columns * 2 }"
  >
    <div
      v-for="section in shownSections"
      :key="section.kind"
      class="inventory-section"
      :class="`inventory-section--${section.kind}`"
    >
      <div
        v-for="(pocket, i) in section.pockets"
        :key="i"
        class="inventory-slot"
        :class="{
          'inventory-slot--empty': !pocket,
          'inventory-slot--dragged': drag.kind === section.kind && drag.from === i,
          'inventory-slot--target': drag.kind === section.kind && drag.over === i,
        }"
        :style="rowStart(section, i)"
        :draggable="arrangeable && Boolean(pocket)"
        @dragstart="onDragStart(section.kind, i, $event)"
        @dragover="onDragOver(section, i, $event)"
        @drop.prevent="onDrop(section, i)"
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
</template>

<script>
import { computed, reactive } from 'vue'
import { formatNumber } from '../i18n'
import { useItemCache } from '../composables/useItemCache'
import { compactQuantity, GAME_COLS, inventoryColumns, moveInSection } from '../inventoryLayout'
import ItemCard from './ItemCard.vue'

// The pockets of one storage on a fixed grid: item pockets, then cargo pockets below them.
export default {
  name: 'InventoryGrid',
  components: { ItemCard },
  props: {
    // From inventorySections(): { items, cargo }, each an array of pockets and nulls.
    sections: { type: Object, required: true },
    maxCols: { type: Number, default: GAME_COLS },
    // Item and cargo pockets next to each other rather than one above the other.
    sideBySide: { type: Boolean, default: false },
    // Stacks can be dragged to other pockets; emits "arrange" with { kind, moves }.
    arrangeable: { type: Boolean, default: false },
  },
  emits: ['arrange'],
  setup(props, { emit }) {
    const { resolveItem } = useItemCache()
    const columns = computed(() => inventoryColumns(props.maxCols))
    const shownSections = computed(() => [
      { kind: 'item', pockets: props.sections.items },
      { kind: 'cargo', pockets: props.sections.cargo },
    ].filter((section) => section.pockets.length))
    const drag = reactive({ kind: null, from: -1, over: -1 })

    // The grid runs on half-pocket columns so a short last row can sit in the middle, as in
    // game: an item pocket spans two of them, a cargo pocket four.
    function rowStart(section, index) {
      const span = section.kind === 'cargo' ? 4 : 2
      const perRow = Math.max(1, Math.floor((columns.value * 2) / span))
      const count = section.pockets.length
      const last = count % perRow
      if (!last || index !== count - last) return null
      return { gridColumnStart: ((perRow - last) * span) / 2 + 1 }
    }

    function onDragStart(kind, index, event) {
      if (!props.arrangeable) return
      drag.kind = kind
      drag.from = index
      event.dataTransfer.effectAllowed = 'move'
      event.dataTransfer.setData('text/plain', `${kind}:${index}`)
    }
    function movesFor(section, index) {
      if (drag.kind !== section.kind) return null
      return moveInSection(section.pockets, drag.from, index)
    }
    function onDragOver(section, index, event) {
      if (!movesFor(section, index)) {
        if (drag.kind === section.kind) drag.over = -1
        return
      }
      event.preventDefault()
      event.dataTransfer.dropEffect = 'move'
      drag.over = index
    }
    function onDrop(section, index) {
      const moves = movesFor(section, index)
      endDrag()
      if (moves) emit('arrange', { kind: section.kind, moves })
    }
    function endDrag() {
      drag.kind = null
      drag.from = -1
      drag.over = -1
    }

    return { columns, shownSections, rowStart, resolveItem, formatNumber, compactQuantity, drag, onDragStart, onDragOver, onDrop, endDrag }
  },
}
</script>

<style scoped>
.inventory-grid {
  --slot-size: 32px;
  display: flex;
  flex-direction: column;
  align-items: center;
  min-height: 0;
}
.inventory-section {
  display: grid;
  grid-template-columns: repeat(var(--inventory-half-cols), minmax(0, calc((var(--slot-size) + 3px) / 2)));
  row-gap: 3px;
  justify-content: center;
  width: 100%;
}
.inventory-grid--side {
  flex-direction: row;
  align-items: flex-start;
  justify-content: center;
  gap: 14px;
}
.inventory-grid--side .inventory-section { width: auto; flex: 0 1 calc(var(--inventory-cols) * (var(--slot-size) + 3px)); }
.inventory-grid:not(.inventory-grid--side) .inventory-section + .inventory-section--cargo {
  margin-top: 4px;
  padding-top: 4px;
  border-top: 1px solid var(--frame-soft);
}
.inventory-slot {
  position: relative;
  grid-column-end: span 2;
  margin: 0 1.5px;
  aspect-ratio: 1;
  display: flex;
  border-radius: 4px;
}
.inventory-slot--empty {
  border: 1px solid var(--slot-edge);
  background: var(--slot);
  box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.3), inset 0 4px 6px rgba(0, 0, 0, 0.18);
}
.inventory-section--cargo .inventory-slot {
  grid-column-end: span 4;
  aspect-ratio: 67 / 32;
}
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
  font-size: 7px;
  line-height: 1.15;
}
.inventory-grid--arranging .inventory-slot:not(.inventory-slot--empty) { cursor: grab; }
.inventory-grid--arranging .inventory-slot :deep(img) { pointer-events: none; }
.inventory-grid--arranging .inventory-slot--empty { border-color: var(--frame); }
.inventory-slot--dragged { opacity: 0.35; }
.inventory-slot--target { outline: 2px solid var(--cream); outline-offset: 1px; }
.slot-qty {
  position: absolute;
  right: 2px;
  bottom: 1px;
  color: #fff;
  font-size: 10px;
  font-weight: 800;
  line-height: 1;
  font-variant-numeric: tabular-nums;
  text-shadow: -1px -1px 0 #000, 1px -1px 0 #000, -1px 1px 0 #000, 1px 1px 0 #000;
  pointer-events: none;
}
</style>
