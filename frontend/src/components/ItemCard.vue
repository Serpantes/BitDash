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
      <span v-if="badge && tierRoman" class="item-card__badge">{{ tierRoman }}</span>
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

const TIER_ROMAN = ['', 'I', 'II', 'III', 'IV', 'V', 'VI', 'VII', 'VIII', 'IX', 'X']

export default {
  name: 'ItemCard',
  props: {
    name: { type: String, default: '' },
    iconAssetName: { type: String, default: null },
    itemType: { type: [String, Number], default: 0 },
    tier: { type: [String, Number], default: null },
    rarityStr: { type: String, default: null },
    variant: { type: String, default: 'default' },
    badge: { type: Boolean, default: false },
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
    const tierRoman = computed(() => TIER_ROMAN[normalizedTier.value] ?? '')
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
      tierRoman,
      title,
      hideBrokenIcon,
    }
  },
}
</script>

<style scoped>
.item-card {
  --tier-color: #4a4858;
  --tier-bg: rgba(74, 72, 88, 0.18);
  --tier-fg: #171625;
  --rarity-color: transparent;
  display: inline-flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  color: var(--text-soft);
  vertical-align: middle;
}
.item-card--default { min-width: 220px; }
.item-card--mini { gap: 0; }
.item-card__icon {
  position: relative;
  width: 34px;
  height: 34px;
  flex: 0 0 34px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: 5px;
  border: 1px solid var(--border);
}
.item-card--mini .item-card__icon {
  width: 52px;
  height: 64px;
  flex-basis: 52px;
  border: 2px solid color-mix(in srgb, var(--tier-color) 55%, #e9dfc4);
  border-radius: 6px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.16), rgba(255, 255, 255, 0) 50%),
    var(--tier-color);
  box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.35);
}
.item-card--mini.item-card--cargo .item-card__icon {
  width: 116px;
  flex-basis: 116px;
}
.item-card--mini.item-card--rarity .item-card__icon {
  border-color: color-mix(in srgb, var(--rarity-color) 70%, #e9dfc4);
}
.item-card--tiered:not(.item-card--mini) .item-card__icon {
  border-color: var(--tier-color);
  background: var(--tier-bg);
}
.item-card__image {
  width: 100%;
  height: 100%;
  object-fit: contain;
}
.item-card--mini .item-card__image {
  width: 86%;
  height: 86%;
  filter: drop-shadow(0 1px 1px rgba(0, 0, 0, 0.35));
}
.item-card__fallback {
  color: var(--muted);
  font-size: 11px;
  font-weight: 800;
}
.item-card__badge {
  position: absolute;
  top: 0;
  left: 0;
  min-width: 13px;
  padding: 1px 3px 1px 2px;
  border-radius: 0 0 4px 0;
  background: var(--cream);
  color: var(--on-cream);
  font-family: var(--font-title);
  font-size: 8px;
  font-weight: 600;
  line-height: 1.1;
  letter-spacing: 0.02em;
  text-align: center;
  pointer-events: none;
}
.item-card__body { min-width: 0; }
.item-card__name { font-weight: 600; }
.item-card__tier,
.item-card__type {
  display: inline-block;
  margin-left: 6px;
  font-size: 10px;
  font-weight: 700;
  padding: 1px 5px;
  border-radius: 3px;
  vertical-align: middle;
}
.item-card__tier {
  color: var(--on-cream);
  background: var(--cream);
}
.item-card__type {
  background: rgba(217, 180, 92, 0.18);
  color: var(--gold);
}
</style>
