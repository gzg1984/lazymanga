<template>
  <section class="collapsible-section">
    <button type="button" class="collapsible-header" @click="toggleCollapsed">
      <span class="collapsible-title-row">
        <span class="collapsible-title">{{ title }}</span>
        <span
          :class="[
            'collapsible-indicator',
            collapsed ? 'collapsible-indicator-collapsed' : 'collapsible-indicator-expanded'
          ]"
          aria-hidden="true"
        />
      </span>
    </button>

    <div v-if="!collapsed" class="collapsible-body">
      <slot />
    </div>
  </section>
</template>

<script setup>
const props = defineProps({
  title: {
    type: String,
    required: true
  },
  collapsed: {
    type: Boolean,
    default: true
  }
})

const emit = defineEmits(['update:collapsed'])

function toggleCollapsed() {
  emit('update:collapsed', !props.collapsed)
}
</script>

<style scoped>
.collapsible-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.collapsible-header {
  display: inline-flex;
  align-items: center;
  justify-content: flex-start;
  width: fit-content;
  max-width: 100%;
  padding: 0;
  border: 0;
  background: transparent;
  cursor: pointer;
}

.collapsible-title-row {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.collapsible-title {
  color: #334155;
  font-size: 14px;
  font-weight: 700;
  line-height: 1.2;
}

.collapsible-indicator {
  position: relative;
  display: inline-block;
  flex: 0 0 auto;
}

.collapsible-indicator-collapsed {
  width: 0;
  height: 0;
  border-top: 6px solid transparent;
  border-bottom: 6px solid transparent;
  border-left: 9px solid #334155;
}

.collapsible-indicator-expanded {
  width: 12px;
  height: 10px;
}

.collapsible-indicator-expanded::before,
.collapsible-indicator-expanded::after {
  content: '';
  position: absolute;
  left: 50%;
  transform: translateX(-50%);
  width: 0;
  height: 0;
}

.collapsible-indicator-expanded::before {
  top: 0;
  border-left: 6px solid transparent;
  border-right: 6px solid transparent;
  border-top: 8px solid #334155;
}

.collapsible-indicator-expanded::after {
  top: 1px;
  border-left: 4px solid transparent;
  border-right: 4px solid transparent;
  border-top: 6px solid #ffffff;
}

.collapsible-body {
  min-width: 0;
}
</style>