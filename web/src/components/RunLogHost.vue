<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { runLog, runLogState } from '@/lib/runLog'

const PANEL_W = 360
const PANEL_H = 280
const PANEL_H_MIN = 44
const MARGIN = 16

const listEl = ref(null)
const dragging = ref(false)
const pos = ref(null)

let dragOffsetX = 0
let dragOffsetY = 0
let moved = false

function clamp(n, min, max) {
  return Math.min(max, Math.max(min, n))
}

function panelHeight() {
  return runLogState.minimized ? PANEL_H_MIN : PANEL_H
}

function defaultPos() {
  const w = window.innerWidth || 1280
  const h = window.innerHeight || 720
  return {
    x: Math.max(MARGIN, w - PANEL_W - MARGIN),
    y: Math.max(MARGIN, h - panelHeight() - MARGIN),
  }
}

function ensurePos() {
  if (!pos.value) pos.value = defaultPos()
  const h = panelHeight()
  pos.value = {
    x: clamp(pos.value.x, MARGIN, Math.max(MARGIN, window.innerWidth - PANEL_W - MARGIN)),
    y: clamp(pos.value.y, MARGIN, Math.max(MARGIN, window.innerHeight - h - MARGIN)),
  }
}

const panelStyle = computed(() => {
  const p = pos.value || defaultPos()
  return {
    position: 'fixed',
    left: `${p.x}px`,
    top: `${p.y}px`,
    right: 'auto',
    bottom: 'auto',
    width: `${PANEL_W}px`,
    height: `${panelHeight()}px`,
    zIndex: 9999,
    margin: 0,
  }
})

watch(
  () => runLogState.open,
  (open) => {
    if (open) ensurePos()
  },
)

watch(
  () => runLogState.minimized,
  () => {
    if (runLogState.open) ensurePos()
  },
)

watch(
  () => runLogState.lines.length,
  async () => {
    if (runLogState.minimized) return
    await nextTick()
    const el = listEl.value
    if (el) el.scrollTop = el.scrollHeight
  },
)

function onPointerMove(e) {
  if (!dragging.value) return
  moved = true
  const h = panelHeight()
  pos.value = {
    x: clamp(e.clientX - dragOffsetX, MARGIN, Math.max(MARGIN, window.innerWidth - PANEL_W - MARGIN)),
    y: clamp(e.clientY - dragOffsetY, MARGIN, Math.max(MARGIN, window.innerHeight - h - MARGIN)),
  }
}

function stopDrag() {
  if (!dragging.value) return
  dragging.value = false
  window.removeEventListener('pointermove', onPointerMove)
  window.removeEventListener('pointerup', stopDrag)
  window.removeEventListener('pointercancel', stopDrag)
}

function startDrag(e) {
  if (e.button != null && e.button !== 0) return
  if (e.target?.closest?.('.run-log__actions')) return
  ensurePos()
  dragging.value = true
  moved = false
  dragOffsetX = e.clientX - pos.value.x
  dragOffsetY = e.clientY - pos.value.y
  window.addEventListener('pointermove', onPointerMove)
  window.addEventListener('pointerup', stopDrag)
  window.addEventListener('pointercancel', stopDrag)
  e.preventDefault()
}

function onTitleClick() {
  if (moved) return
  runLog.restore()
}

function onResize() {
  if (runLogState.open) ensurePos()
}

window.addEventListener('resize', onResize)
onBeforeUnmount(() => {
  stopDrag()
  window.removeEventListener('resize', onResize)
})

const toneClass = {
  success: 'run-log__line--success',
  error: 'run-log__line--error',
  warning: 'run-log__line--warning',
  info: 'run-log__line--info',
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="runLogState.open"
      class="run-log"
      :class="{ 'run-log--min': runLogState.minimized, 'run-log--dragging': dragging }"
      :style="panelStyle"
    >
      <header class="run-log__head" @pointerdown="startDrag">
        <button type="button" class="run-log__title" @click="onTitleClick">
          运行日志
          <span v-if="runLogState.minimized && runLogState.lines.length" class="run-log__badge">
            {{ runLogState.lines.length }}
          </span>
        </button>
        <div class="run-log__actions">
          <button type="button" title="清空" @click="runLog.clear()">清空</button>
          <button
            type="button"
            :title="runLogState.minimized ? '展开' : '收起'"
            @click="runLogState.minimized ? runLog.restore() : runLog.minimize()"
          >
            {{ runLogState.minimized ? '展开' : '收起' }}
          </button>
          <button type="button" title="关闭" @click="runLog.close()">关闭</button>
        </div>
      </header>
      <div v-show="!runLogState.minimized" ref="listEl" class="run-log__body" aria-live="polite">
        <p v-if="!runLogState.lines.length" class="run-log__empty">还没有动静，领积分或跑批量时会往这儿写。</p>
        <div
          v-for="line in runLogState.lines"
          :key="line.id"
          class="run-log__line"
          :class="toneClass[line.tone] || toneClass.info"
        >
          <span class="run-log__time">{{ line.at }}</span>
          <span class="run-log__text">{{ line.text }}</span>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.run-log {
  display: flex;
  flex-direction: column;
  border: 1px solid hsl(var(--border));
  border-radius: 0.9rem;
  background: hsl(var(--card) / 0.96);
  color: hsl(var(--card-foreground));
  box-shadow: var(--shadow-toast, 0 12px 30px rgba(0, 0, 0, 0.18));
  backdrop-filter: blur(8px);
  overflow: hidden;
  user-select: none;
}

.run-log--dragging {
  opacity: 0.96;
  box-shadow: 0 16px 36px rgba(0, 0, 0, 0.22);
}

.run-log__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  flex: none;
  height: 44px;
  padding: 0 0.7rem;
  border-bottom: 1px solid hsl(var(--border) / 0.8);
  background: hsl(var(--muted) / 0.35);
  cursor: move;
  touch-action: none;
}

.run-log--min .run-log__head {
  border-bottom: 0;
}

.run-log__title {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  border: 0;
  background: transparent;
  color: inherit;
  font-size: 0.8rem;
  font-weight: 600;
  cursor: move;
  padding: 0;
}

.run-log__badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 1.25rem;
  height: 1.25rem;
  padding: 0 0.3rem;
  border-radius: 999px;
  background: hsl(var(--primary));
  color: hsl(var(--primary-foreground));
  font-size: 0.65rem;
  font-weight: 600;
}

.run-log__actions {
  display: flex;
  gap: 0.25rem;
}

.run-log__actions button {
  border: 0;
  background: transparent;
  color: hsl(var(--muted-foreground));
  font-size: 0.7rem;
  cursor: pointer;
  padding: 0.15rem 0.35rem;
  border-radius: 0.4rem;
}

.run-log__actions button:hover {
  background: hsl(var(--muted));
  color: hsl(var(--foreground));
}

.run-log__body {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 0.55rem 0.7rem 0.7rem;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  user-select: text;
}

.run-log__empty {
  margin: 0;
  font-size: 0.75rem;
  color: hsl(var(--muted-foreground));
}

.run-log__line {
  display: grid;
  grid-template-columns: 3.4rem 1fr;
  gap: 0.45rem;
  font-size: 0.75rem;
  line-height: 1.45;
}

.run-log__time {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  color: hsl(var(--muted-foreground));
  font-size: 0.68rem;
  padding-top: 0.1rem;
}

.run-log__text {
  word-break: break-word;
}

.run-log__line--success .run-log__text {
  color: hsl(var(--tone-success-strong, 152 76% 36%));
}

.run-log__line--error .run-log__text {
  color: hsl(var(--tone-error-strong, 350 90% 42%));
}

.run-log__line--warning .run-log__text {
  color: hsl(var(--tone-warning-strong, 32 90% 38%));
}

.run-log__line--info .run-log__text {
  color: hsl(var(--foreground));
}
</style>
