<template>
  <div class="inventory-grid" :style="{ '--inventory-cols': columns }">
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
        :class="{ 'inventory-slot--empty': !pocket }"
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
import { computed } from 'vue'
import { formatNumber } from '../i18n'
import { useItemCache } from '../composables/useItemCache'
import { compactQuantity, inventoryColumns } from '../inventoryLayout'
import ItemCard from './ItemCard.vue'

// The pockets of one storage on a fixed grid: item pockets, then cargo pockets below them.
export default {
  name: 'InventoryGrid',
  components: { ItemCard },
  props: {
    // From inventorySections(): { items, cargo }, each an array of pockets and nulls.
    sections: { type: Object, required: true },
    maxCols: { type: Number, default: 10 },
    maxHeight: { type: Number, default: Infinity },
  },
  setup(props) {
    const { resolveItem } = useItemCache()
    const columns = computed(() => inventoryColumns(props.sections, props.maxCols, props.maxHeight))
    const shownSections = computed(() => [
      { kind: 'item', pockets: props.sections.items },
      { kind: 'cargo', pockets: props.sections.cargo },
    ].filter((section) => section.pockets.length))
    return { columns, shownSections, resolveItem, formatNumber, compactQuantity }
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
  grid-template-columns: repeat(var(--inventory-cols), minmax(0, var(--slot-size)));
  gap: 3px;
  justify-content: center;
  width: 100%;
}
.inventory-section + .inventory-section--cargo {
  margin-top: 4px;
  padding-top: 4px;
  border-top: 1px solid var(--frame-soft);
}
.inventory-slot {
  position: relative;
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
  grid-column: span 2;
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
