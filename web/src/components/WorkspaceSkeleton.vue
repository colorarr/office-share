<script setup>
defineProps({ layout: { type: String, default: 'list' } });
const rows = 6;
</script>

<template>
  <div class="workspace-skeleton" role="status" aria-live="polite">
    <span class="visually-hidden">正在加载共享内容…</span>
    <div v-if="layout === 'grid'" class="items-grid" aria-hidden="true">
      <article v-for="row in rows" :key="row" class="item-card">
        <div class="item-card-preview skeleton-block"></div>
        <div class="item-card-details">
          <span class="skeleton-card-copy">
            <strong class="skeleton-block skeleton-card-title"></strong>
            <small class="skeleton-block skeleton-card-meta"></small>
          </span>
          <span class="icon-button"><i class="skeleton-block skeleton-action"></i></span>
          <span class="icon-button"><i class="skeleton-block skeleton-action"></i></span>
        </div>
      </article>
    </div>
    <div v-else class="table-scroll" aria-hidden="true">
      <table class="file-table">
        <thead>
          <tr>
            <th class="check-cell"><span class="skeleton-block skeleton-check"></span></th>
            <th class="name-column">名称</th>
            <th class="owner-column">共享者</th>
            <th class="time-column">更新时间</th>
            <th class="size-column">大小</th>
            <th class="actions-column"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row">
            <td class="check-cell"><span class="skeleton-block skeleton-check"></span></td>
            <td>
              <div class="file-name-button">
                <span class="file-thumbnail skeleton-block"></span>
                <span class="skeleton-name">
                  <strong class="skeleton-block skeleton-title"></strong>
                  <small class="skeleton-block skeleton-description"></small>
                  <span class="mobile-file-meta">
                    <span class="skeleton-block skeleton-mobile-meta"></span>
                  </span>
                </span>
              </div>
            </td>
            <td class="owner-column"><span class="skeleton-block skeleton-owner"></span></td>
            <td class="time-column"><span class="skeleton-block skeleton-time"></span></td>
            <td class="size-column"><span class="skeleton-block skeleton-size"></span></td>
            <td>
              <div class="row-actions">
                <span class="icon-button star-button">
                  <i class="skeleton-block skeleton-action"></i>
                </span>
                <span class="icon-button"><i class="skeleton-block skeleton-action"></i></span>
                <span class="icon-button"><i class="skeleton-block skeleton-action"></i></span>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
/* Geometry comes from the same table/card classes as the loaded content. */
.skeleton-block {
  display: block;
  position: relative;
  overflow: hidden;
  background: var(--raised);
  border-radius: 4px;
}
.skeleton-block::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(90deg, transparent, rgb(255 255 255 / 65%), transparent);
  transform: translateX(-100%);
  animation: skeleton-shimmer 1.6s ease-in-out infinite;
}
.skeleton-check {
  width: 16px;
  height: 16px;
}
.skeleton-name,
.skeleton-card-copy {
  flex: 1;
  min-width: 0;
}
.skeleton-title {
  width: 62%;
  height: 21px;
}
.skeleton-description {
  width: 38%;
  height: 18px;
}
.skeleton-owner {
  width: 46px;
  height: 16px;
}
.skeleton-time {
  width: 72px;
  height: 16px;
}
.skeleton-size {
  width: 42px;
  height: 16px;
  margin-left: auto;
}
.skeleton-action {
  width: 16px;
  height: 16px;
}
.skeleton-mobile-meta {
  width: 85%;
  height: 17.6px;
}
.skeleton-card-title {
  width: 78%;
  height: 1.5em;
}
.skeleton-card-meta {
  width: 58%;
  height: 1.5em;
}
.item-card-preview.skeleton-block {
  border-radius: 0;
}
@keyframes skeleton-shimmer {
  to {
    transform: translateX(100%);
  }
}
@media (max-width: 640px) {
  .skeleton-description {
    display: none;
  }
}
@media (prefers-reduced-motion: reduce) {
  .skeleton-block::after {
    animation: none;
    background: none;
  }
}
</style>
