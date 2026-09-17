<script setup lang="ts">
import * as tooltip from '@zag-js/tooltip'
import { normalizeProps, useMachine } from '@zag-js/vue'
import { computed, useId } from 'vue'

const props = withDefaults(
  defineProps<{
    /** Tooltip text. Empty or absent renders the trigger bare. */
    label?: string
    placement?: tooltip.PositioningOptions['placement']
  }>(),
  { label: undefined, placement: 'top' },
)

const id = useId()

const service = useMachine(
  tooltip.machine,
  computed(() => ({
    id,
    positioning: { placement: props.placement, gutter: 6 },
  })),
)

const api = computed(() => tooltip.connect(service, normalizeProps))

const enabled = computed(() => !!props.label?.trim())
</script>

<template>
  <!--
    The trigger wraps the caller's element rather than replacing it, so existing
    markup and styling are untouched. Zag opens on hover *and* focus and wires
    aria-describedby — a native `title` does neither, and is unreachable on
    touch.
  -->
  <span v-if="enabled" v-bind="api.getTriggerProps()" class="tooltip-trigger">
    <slot />
  </span>
  <slot v-else />

  <Teleport v-if="enabled" to="body">
    <div
      v-if="api.open"
      v-bind="api.getPositionerProps()"
      class="tooltip-positioner"
    >
      <div v-bind="api.getContentProps()" class="tooltip-content">
        {{ label }}
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
/* Colocated from components.css: used only by this component. The @layer
   wrapper is kept so precedence against Tailwind utilities is unchanged.
   Shared primitives stay in assets/components.css. */
@layer components {
  .tooltip-trigger {
    display: inline-flex;
  }
  .tooltip-positioner {
    isolation: isolate;
  }
  .tooltip-content {
    z-index: var(--z-index-dropdown);
    max-width: 260px;
    padding: 6px 9px;
    border: 1px solid var(--color-line-2);
    border-radius: 7px;
    background: var(--color-surface-4);
    color: var(--color-ink);
    font-size: 12.5px;
    line-height: 1.4;
    box-shadow: 0 10px 28px rgb(0 0 0 / 0.45);
    animation: fb-pop 0.12s ease both;
  }
}
</style>
