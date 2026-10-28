<script setup>
import { computed } from 'vue';
import {
  PhSquaresFour,
  PhFolderSimple,
  PhClock,
  PhStar,
  PhTrash,
  PhUsers,
  PhGearSix,
  PhMagnifyingGlass,
  PhPlus,
  PhUploadSimple,
  PhCaretDown,
  PhCaretRight,
  PhArrowLeft,
  PhArrowUpRight,
  PhDownloadSimple,
  PhDotsThree,
  PhFileText,
  PhImage,
  PhTextT,
  PhList,
  PhGridFour,
  PhX,
  PhCheck,
  PhCopy,
  PhLink,
  PhArrowCounterClockwise,
  PhSignOut,
  PhLockKey,
  PhArrowRight,
  PhInfo,
  PhCloudCheck,
  PhWarningCircle,
  PhPencilSimple,
  PhCheckCircle,
  PhPaperPlaneTilt,
} from '@phosphor-icons/vue';

const props = defineProps({
  name: { type: String, default: 'file' },
  size: { type: Number, default: 20 },
  weight: { type: String, default: 'regular' },
});
const icons = {
  squares: PhSquaresFour,
  folder: PhFolderSimple,
  clock: PhClock,
  star: PhStar,
  trash: PhTrash,
  users: PhUsers,
  settings: PhGearSix,
  search: PhMagnifyingGlass,
  plus: PhPlus,
  upload: PhUploadSimple,
  down: PhCaretDown,
  right: PhCaretRight,
  back: PhArrowLeft,
  external: PhArrowUpRight,
  download: PhDownloadSimple,
  more: PhDotsThree,
  file: PhFileText,
  image: PhImage,
  text: PhTextT,
  list: PhList,
  grid: PhGridFour,
  close: PhX,
  check: PhCheck,
  copy: PhCopy,
  link: PhLink,
  restore: PhArrowCounterClockwise,
  logout: PhSignOut,
  lock: PhLockKey,
  arrow: PhArrowRight,
  info: PhInfo,
  cloud: PhCloudCheck,
  warning: PhWarningCircle,
  edit: PhPencilSimple,
  success: PhCheckCircle,
  send: PhPaperPlaneTilt,
};
const approvedNames = [
  'logo',
  'folder',
  'person',
  'team',
  'share',
  'clock',
  'star',
  'trash',
  'search',
  'upload',
  'download',
  'file',
  'image',
  'sheet',
  'text',
  'settings',
  'lock',
  'chevron',
  'close',
  'plus',
  'grid',
  'list',
  'more',
  'check',
  'copy',
  'menu',
  'restore',
  'link',
];
const symbol = computed(
  () =>
    ({ users: 'team', squares: 'logo', right: 'chevron', down: 'chevron' })[props.name] ||
    props.name,
);
</script>

<template>
  <svg
    v-if="approvedNames.includes(symbol)"
    viewBox="0 0 24 24"
    :width="size"
    :height="size"
    aria-hidden="true"
    :class="{ 'icon-down': name === 'down' }"
  >
    <g v-if="symbol === 'logo'">
      <path d="M3 5h8v5H3zm10 0h8v5h-8zM3 12h18v2H3zm1 3h2v5H4zm14 0h2v5h-2z" />
    </g>
    <g v-if="symbol === 'folder'">
      <path d="M3 5a2 2 0 0 1 2-2h5l2 3h7a2 2 0 0 1 2 2v1H3zM2 10h20l-2 10H4z" />
    </g>
    <g v-if="symbol === 'person'">
      <path d="M12 3a4 4 0 1 1 0 8 4 4 0 0 1 0-8zM4 21v-3a8 6 0 0 1 16 0v3z" />
    </g>
    <g v-if="symbol === 'team'">
      <path
        d="M9 3a4 4 0 1 1 0 8 4 4 0 0 1 0-8zm9 1a3 3 0 1 1 0 6 3 3 0 0 1 0-6zM1 21v-3a8 6 0 0 1 16 0v3zm17-8a6 5 0 0 1 5 5v3h-4v-3a8 8 0 0 0-1-5z"
      />
    </g>
    <g v-if="symbol === 'share'">
      <path
        d="M18 2a4 4 0 1 1-3.5 6L9 11a4 4 0 0 1 0 2l5.5 3a4 4 0 1 1-1 2L8 15a4 4 0 1 1 0-6l5.5-3A4 4 0 0 1 18 2z"
      />
    </g>
    <g v-if="symbol === 'clock'">
      <path
        fill-rule="evenodd"
        d="M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zm0 2a8 8 0 1 1 0 16 8 8 0 0 1 0-16zm-1 3h2v5l4 2-1 2-5-3z"
      />
    </g>
    <g v-if="symbol === 'star'">
      <path d="m12 2 3 6.4 7 1-5 5 1.2 7L12 18l-6.2 3.4 1.2-7-5-5 7-1z" />
    </g>
    <g v-if="symbol === 'trash'">
      <path d="M8 2h8l1 3h4v2H3V5h4zm-3 7h14l-1 13H6zm4 2v8h2v-8zm4 0v8h2v-8z" />
    </g>
    <g v-if="symbol === 'search'">
      <path
        fill-rule="evenodd"
        d="M10 2a8 8 0 1 0 4.9 14.3l5.4 5.4 1.4-1.4-5.4-5.4A8 8 0 0 0 10 2zm0 2a6 6 0 1 1 0 12 6 6 0 0 1 0-12z"
      />
    </g>
    <g v-if="symbol === 'upload'">
      <path d="m12 2 6 6-1.4 1.4L13 5.8V16h-2V5.8L7.4 9.4 6 8zM3 15h2v5h14v-5h2v7H3z" />
    </g>
    <g v-if="symbol === 'download'">
      <path d="M11 2h2v10.2l3.6-3.6L18 10l-6 6-6-6 1.4-1.4 3.6 3.6zM3 16h2v4h14v-4h2v6H3z" />
    </g>
    <g v-if="symbol === 'file'">
      <path d="M5 2h9v6h6v14H5zm11 0 4 4h-4zM8 12v2h9v-2zm0 4v2h7v-2z" />
    </g>
    <g v-if="symbol === 'image'">
      <path
        fill-rule="evenodd"
        d="M2 3h20v18H2zm2 2v12l5-5 4 4 3-3 4 4V5zm12 2a2 2 0 1 0 0 4 2 2 0 0 0 0-4z"
      />
    </g>
    <g v-if="symbol === 'sheet'">
      <path
        fill-rule="evenodd"
        d="M3 2h18v20H3zm2 4v3h6V6zm8 0v3h6V6zm-8 5v3h6v-3zm8 0v3h6v-3zm-8 5v3h6v-3zm8 0v3h6v-3z"
      />
    </g>
    <g v-if="symbol === 'text'">
      <path d="M3 3h18v5h-2V5h-6v14h3v2H8v-2h3V5H5v3H3z" />
    </g>
    <g v-if="symbol === 'settings'">
      <path
        fill-rule="evenodd"
        d="m10 2 4 0 .6 3 2 .9 2.7-1 2 3.5-2.2 2v2.3l2.2 2-2 3.5-2.7-1-2 .9-.6 3h-4l-.6-3-2-.9-2.7 1-2-3.5 2.2-2v-2.3l-2.2-2 2-3.5 2.7 1 2-.9zM12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8z"
      />
    </g>
    <g v-if="symbol === 'lock'">
      <path
        fill-rule="evenodd"
        d="M6 10V7a6 6 0 0 1 12 0v3h2v12H4V10zm2 0h8V7a4 4 0 0 0-8 0zm3 4v4h2v-4z"
      />
    </g>
    <g v-if="symbol === 'chevron'">
      <path d="m9 4 8 8-8 8-1.5-1.5L14 12 7.5 5.5z" />
    </g>
    <g v-if="symbol === 'close'">
      <path d="m5 3 7 7 7-7 2 2-7 7 7 7-2 2-7-7-7 7-2-2 7-7-7-7z" />
    </g>
    <g v-if="symbol === 'plus'">
      <path d="M11 3h2v8h8v2h-8v8h-2v-8H3v-2h8z" />
    </g>
    <g v-if="symbol === 'grid'">
      <path d="M3 3h7v7H3zm11 0h7v7h-7zM3 14h7v7H3zm11 0h7v7h-7z" />
    </g>
    <g v-if="symbol === 'list'">
      <path d="M3 4h3v3H3zm5 0h13v3H8zM3 10h3v3H3zm5 0h13v3H8zM3 16h3v3H3zm5 0h13v3H8z" />
    </g>
    <g v-if="symbol === 'more'">
      <circle cx="5" cy="12" r="2" />
      <circle cx="12" cy="12" r="2" />
      <circle cx="19" cy="12" r="2" />
    </g>
    <g v-if="symbol === 'check'">
      <path d="m3 12 5 5L21 4l2 2L8 21l-7-7z" />
    </g>
    <g v-if="symbol === 'copy'">
      <path fill-rule="evenodd" d="M8 1h14v17h-4v5H2V6h6zm2 5h8v10h2V3H10zM4 8v13h12V8z" />
    </g>
    <g v-if="symbol === 'menu'">
      <path d="M3 5h18v2H3zm0 6h18v2H3zm0 6h18v2H3z" />
    </g>
    <g v-if="symbol === 'restore'">
      <path d="M12 3a9 9 0 1 1-8.5 12H6a7 7 0 1 0 1.1-7.9L10 10H2V2l3.6 3.6A9 9 0 0 1 12 3z" />
    </g>
    <g v-if="symbol === 'link'">
      <path
        d="m15 3 2-1a5 5 0 0 1 7 7l-6 6a5 5 0 0 1-7-1l2-2a2.5 2.5 0 0 0 3 .5l6-6a2.5 2.5 0 0 0-3.5-3.5l-2 2zM9 21l-2 1a5 5 0 0 1-7-7l6-6a5 5 0 0 1 7 1l-2 2a2.5 2.5 0 0 0-3-.5l-6 6A2.5 2.5 0 0 0 5.5 21l2-2z"
      />
    </g>
  </svg>
  <component
    v-else
    :is="icons[name] || icons.file"
    :size="size"
    :weight="weight"
    aria-hidden="true"
  />
</template>
