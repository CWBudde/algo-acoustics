import test from "node:test";
import assert from "node:assert/strict";

import {
  AuralizationEngine,
  equalPowerFadeCurves,
} from "./auralization-engine.mjs";

class FakeAudioParam {
  constructor() {
    this.value = 1;
    this.events = [];
  }

  cancelScheduledValues(time) {
    this.events.push({ type: "cancel", time });
  }

  setValueAtTime(value, time) {
    this.value = value;
    this.events.push({ type: "set", value, time });
  }

  setValueCurveAtTime(curve, time, duration) {
    this.events.push({ type: "curve", curve, time, duration });
  }

  linearRampToValueAtTime(value, time) {
    this.value = value;
    this.events.push({ type: "linear", value, time });
  }
}

class FakeNode {
  constructor() {
    this.connections = [];
    this.disconnections = [];
  }

  connect(destination) {
    this.connections.push(destination);
  }

  disconnect(destination) {
    this.disconnections.push(destination ?? null);
  }
}

class FakeSource extends FakeNode {
  constructor() {
    super();
    this.buffer = null;
    this.loop = false;
    this.started = false;
    this.stopped = false;
    this.onended = null;
  }

  start() {
    this.started = true;
  }

  stop() {
    this.stopped = true;
  }

  end() {
    this.onended?.();
  }
}

class FakeGain extends FakeNode {
  constructor() {
    super();
    this.gain = new FakeAudioParam();
  }
}

class FakeConvolver extends FakeNode {
  constructor() {
    super();
    this.buffer = null;
    this.normalize = true;
  }
}

class FakeContext {
  constructor() {
    this.currentTime = 0;
    this.destination = new FakeNode();
    this.sources = [];
    this.convolvers = [];
  }

  createBufferSource() {
    const source = new FakeSource();
    this.sources.push(source);
    return source;
  }

  createGain() {
    return new FakeGain();
  }

  createConvolver() {
    const convolver = new FakeConvolver();
    this.convolvers.push(convolver);
    return convolver;
  }
}

function createTimers() {
  let nextID = 1;
  const timers = new Map();
  return {
    timers,
    setTimer(callback, delay) {
      const id = nextID;
      nextID += 1;
      timers.set(id, { callback, delay, active: true });
      return id;
    },
    clearTimer(id) {
      const timer = timers.get(id);
      if (timer) {
        timer.active = false;
      }
    },
    runDelay(delay) {
      for (const timer of timers.values()) {
        if (timer.active && timer.delay === delay) {
          timer.active = false;
          timer.callback();
        }
      }
    },
  };
}

function createEngine(onPlaybackEnded = () => {}) {
  const context = new FakeContext();
  const timers = createTimers();
  const engine = new AuralizationEngine({
    context,
    setTimer: timers.setTimer,
    clearTimer: timers.clearTimer,
    onPlaybackEnded,
  });
  return { context, timers, engine };
}

test("equal-power curves preserve power across the fade", () => {
  const { fadeOut, fadeIn } = equalPowerFadeCurves(17);
  assert.equal(fadeOut[0], 1);
  assert.ok(Math.abs(fadeOut.at(-1)) < 1e-6);
  assert.equal(fadeIn[0], 0);
  assert.equal(fadeIn.at(-1), 1);
  for (let index = 0; index < fadeOut.length; index += 1) {
    const power = fadeOut[index] ** 2 + fadeIn[index] ** 2;
    assert.ok(Math.abs(power - 1) < 1e-6);
  }
});

test("an active IR replacement crossfades and retires the old convolver", () => {
  const { engine, timers } = createEngine();
  const firstIR = { duration: 1.2 };
  const nextIR = { duration: 1.8 };
  engine.setImpulseResponse(firstIR);
  engine.play({ duration: 3 }, { wetMix: 0.75, gainLinear: 0.8 });
  const oldBranch = engine.branches[0];

  assert.equal(engine.setImpulseResponse(nextIR), true);
  assert.equal(engine.branches.length, 2);
  const nextBranch = engine.branches[1];
  const oldCurve = oldBranch.gain.gain.events.find(
    (event) => event.type === "curve",
  ).curve;
  const nextCurve = nextBranch.gain.gain.events.find(
    (event) => event.type === "curve",
  ).curve;
  assert.equal(oldCurve[0], 1);
  assert.ok(Math.abs(oldCurve.at(-1)) < 1e-6);
  assert.equal(nextCurve[0], 0);
  assert.equal(nextCurve.at(-1), 1);

  timers.runDelay(110);
  assert.deepEqual(engine.branches, [nextBranch]);
  assert.ok(oldBranch.convolver.disconnections.length > 0);
});

test("rapid updates keep only the newest convolver", () => {
  const { context, engine, timers } = createEngine();
  engine.setImpulseResponse({ duration: 1 });
  engine.play({ duration: 4 });
  engine.setImpulseResponse({ duration: 1.1 });
  context.currentTime = 0.05;
  const newestIR = { duration: 1.2 };
  engine.setImpulseResponse(newestIR);

  timers.runDelay(110);
  assert.equal(engine.branches.length, 1);
  assert.equal(engine.branches[0].convolver.buffer, newestIR);
});

test("an update during the reverb tail is reserved for next playback", () => {
  const { engine } = createEngine();
  const firstIR = { duration: 2 };
  const nextIR = { duration: 3 };
  engine.setImpulseResponse(firstIR);
  engine.play({ duration: 1 });
  engine.source.end();

  assert.equal(engine.isPlaying, true);
  assert.equal(engine.isSourceActive, false);
  assert.equal(engine.setImpulseResponse(nextIR), false);
  assert.equal(engine.branches.length, 1);
  assert.equal(engine.impulseResponse, nextIR);
});

test("natural cleanup includes the IR tail and disconnects the graph", () => {
  let ended = 0;
  const { engine, timers } = createEngine(() => {
    ended += 1;
  });
  engine.setImpulseResponse({ duration: 1.5 });
  engine.play({ duration: 2 });
  engine.source.end();

  assert.equal(engine.isPlaying, true);
  timers.runDelay(3600);
  assert.equal(engine.isPlaying, false);
  assert.equal(ended, 1);
});

test("mix changes ramp live parameters and stop cancels playback", () => {
  const { engine } = createEngine();
  engine.setImpulseResponse({ duration: 1 });
  engine.play({ duration: 2 });
  const source = engine.source;
  const dryGain = engine.dryGain;
  const wetGain = engine.wetGain;
  const outputGain = engine.outputGain;

  engine.setMix(0.25, 0.5);
  assert.equal(dryGain.gain.events.at(-1).type, "linear");
  assert.equal(wetGain.gain.events.at(-1).type, "linear");
  assert.equal(outputGain.gain.events.at(-1).value, 0.5);

  engine.stop();
  assert.equal(source.stopped, true);
  assert.equal(engine.isPlaying, false);
});
