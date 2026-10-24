import assert from 'node:assert/strict';
import test from 'node:test';
import { effectScope, ref } from 'vue';
import { useDelayedIndicator } from './useDelayedIndicator.js';

function setup() {
  let now = 0;
  let sequence = 0;
  const timers = new Map();
  const clock = {
    setTimeout(callback, delay) {
      const id = ++sequence;
      timers.set(id, { callback, time: now + delay });
      return id;
    },
    clearTimeout: (id) => timers.delete(id),
    advance(ms) {
      now += ms;
      for (const [id, task] of timers) {
        if (task.time <= now) {
          timers.delete(id);
          task.callback();
        }
      }
    },
  };
  const scope = effectScope();
  const busy = ref(false);
  const visible = scope.run(() => useDelayedIndicator(busy, { clock }));
  return { busy, visible, clock, scope, timers };
}

test('fast operations never flash a late indicator', () => {
  const state = setup();
  state.busy.value = true;
  state.clock.advance(100);
  assert.equal(state.visible.value, false);
  state.busy.value = false;
  state.clock.advance(200);
  assert.equal(state.visible.value, false);
  assert.equal(state.timers.size, 0);
  state.scope.stop();
});

test('slow operations reveal after 180ms and hide immediately on completion', () => {
  const state = setup();
  state.busy.value = true;
  state.clock.advance(179);
  assert.equal(state.visible.value, false);
  state.clock.advance(1);
  assert.equal(state.visible.value, true);
  state.busy.value = false;
  assert.equal(state.visible.value, false);
  state.scope.stop();
});

test('a new attempt gets a fresh delay and unmount clears pending timers', () => {
  const state = setup();
  state.busy.value = true;
  state.clock.advance(100);
  state.busy.value = false;
  state.busy.value = true;
  state.clock.advance(100);
  assert.equal(state.visible.value, false);
  state.scope.stop();
  assert.equal(state.timers.size, 0);
  state.clock.advance(1000);
  assert.equal(state.visible.value, false);
});
