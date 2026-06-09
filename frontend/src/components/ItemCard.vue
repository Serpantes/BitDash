<template>
  <span
    class="item-card"
    :class="[
      `item-card--${variant}`,
      `item-card--${typeKind}`,
      {
        'item-card--tiered': normalizedTier,
        'item-card--rarity': rarityColorStyle,
      },
    ]"
    :style="{ ...cardStyle, ...rarityColorStyle }"
    :title="title"
  >
    <span class="item-card__icon">
      <img
        v-if="resolvedIconUrl && !iconFailed"
        class="item-card__image"
        :src="resolvedIconUrl"
        :alt="displayName"
        loading="lazy"
        @error="hideBrokenIcon"
      />
      <span v-else class="item-card__fallback" aria-hidden="true"></span>
    </span>
    <span v-if="variant !== 'mini'" class="item-card__body">
      <span class="item-card__name">{{ displayName }}</span>
      <span v-if="tierLabel" class="item-card__tier">{{ tierLabel }}</span>
      <span v-if="typeKind === 'cargo'" class="item-card__type">cargo</span>
    </span>
  </span>
</template>

<script>
import { computed, ref, watch } from 'vue'
import { iconUrl, itemTypeKind, rarityStyle, tierNumber, tierStyle } from '../itemVisuals'

export default {
  name: 'ItemCard',
  props: {
    name: { type: String, default: '' },
    iconAssetName: { type: String, default: null },
    itemType: { type: [String, Number], default: 0 },
    tier: { type: [String, Number], default: null },
    rarityStr: { type: String, default: null },
    variant: { type: String, default: 'default' },
  },
  setup(props) {
    const displayName = computed(() => props.name || '?')
    const typeKind = computed(() => itemTypeKind(props.itemType))
    const normalizedTier = computed(() => tierNumber(props.tier))
    const cardStyle = computed(() => tierStyle(props.tier) ?? null)
    const rarityColorStyle = computed(() => rarityStyle(props.rarityStr))
    const resolvedIconUrl = computed(() => iconUrl(props.iconAssetName))
    const iconFailed = ref(false)
    const tierLabel = computed(() => normalizedTier.value ? `T${normalizedTier.value}` : '')
    const title = computed(() => {
      const parts = [displayName.value]
      if (tierLabel.value) parts.push(tierLabel.value)
      if (props.rarityStr) parts.push(props.rarityStr)
      if (typeKind.value === 'cargo') parts.push('cargo')
      return parts.join(' · ')
    })

    watch(resolvedIconUrl, () => {
      iconFailed.value = false
    })

    function hideBrokenIcon() {
      iconFailed.value = true
    }

    return {
      displayName,
      typeKind,
      normalizedTier,
      cardStyle,
      rarityColorStyle,
      resolvedIconUrl,
      iconFailed,
      tierLabel,
      title,
      hideBrokenIcon,
    }
  },
}
</script>

<style scoped>
.item-card {
  --tier-color: #37324f;
  --tier-bg: rgba(55, 50, 79, 0.18);
  --tier-fg: #171625;
  --rarity-color: #d1d5db;
  display: inline-flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  color: var(--text-soft, #f4f2ff);
  vertical-align: middle;
}
.item-card--default { min-width: 220px; }
.item-card--mini { gap: 0; }
.item-card__icon {
  width: 34px;
  height: 34px;
  flex: 0 0 34px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: 8px;
  border: 1px solid var(--border, #45435c);
}
.item-card--mini .item-card__icon {
  width: 52px;
  height: 64px;
  flex-basis: 52px;
  border: 0;
  border-radius: 9px;
}
.item-card--mini.item-card--cargo .item-card__icon {
  width: 116px;
  flex-basis: 116px;
}
.item-card--item .item-card__icon { background: rgba(138, 164, 255, 0.18); }
.item-card--cargo .item-card__icon { background: rgba(248, 198, 109, 0.18); }
.item-card--mini .item-card__icon,
.item-card--mini.item-card--tiered .item-card__icon {
  background:
    linear-gradient(145deg, rgba(255, 255, 255, 0.24), rgba(255, 255, 255, 0) 44%),
    var(--tier-color);
}
.item-card--tiered .item-card__icon {
  border-color: var(--tier-color);
  box-shadow: inset 3px 0 0 var(--tier-color);
}
.item-card--rarity .item-card__icon { border-color: var(--rarity-color); }
.item-card--mini.item-card--tiered .item-card__icon {
  box-shadow: inset 0 0 0 2px rgba(15, 23, 42, 0.38);
}
.item-card--mini.item-card--rarity .item-card__icon {
  border: 4px solid transparent;
  background:
    linear-gradient(var(--tier-color), var(--tier-color)) padding-box,
    linear-gradient(
      225deg,
      color-mix(in srgb, var(--rarity-color) 70%, white) 0%,
      var(--rarity-color) 48%,
      color-mix(in srgb, var(--rarity-color) 60%, black) 100%
    ) border-box;
}
.item-card__image {
  width: 100%;
  height: 100%;
  object-fit: contain;
}
.item-card--mini .item-card__image {
  width: 86%;
  height: 86%;
}
.item-card__fallback {
  color: var(--muted, #a7a1bd);
  font-size: 11px;
  font-weight: 800;
}
.item-card__body { min-width: 0; }
.item-card__name { font-weight: 500; }
.item-card__tier,
.item-card__type {
  display: inline-block;
  margin-left: 6px;
  font-size: 10px;
  font-weight: 700;
  padding: 1px 5px;
  border-radius: 5px;
  vertical-align: middle;
}
.item-card__tier {
  color: var(--tier-fg);
  background: var(--tier-color);
}
.item-card__type {
  background: rgba(248, 198, 109, 0.18);
  color: var(--warning, #f8c66d);
}
</style>
