const DEFAULT_FADE_SECONDS = 0.1;
const DEFAULT_PARAMETER_RAMP_SECONDS = 0.02;
const FADE_CURVE_POINTS = 64;

export function equalPowerFadeCurves(points = FADE_CURVE_POINTS) {
  if (!Number.isInteger(points) || points < 2) {
    throw new Error("fade curves require at least two points");
  }

  const fadeOut = new Float32Array(points);
  const fadeIn = new Float32Array(points);
  for (let index = 0; index < points; index += 1) {
    const phase = (index / (points - 1)) * (Math.PI / 2);
    fadeOut[index] = Math.cos(phase);
    fadeIn[index] = Math.sin(phase);
  }
  return { fadeOut, fadeIn };
}

export class AuralizationEngine {
  constructor({
    context,
    destination = context?.destination,
    fadeSeconds = DEFAULT_FADE_SECONDS,
    setTimer = (callback, delay) => globalThis.setTimeout(callback, delay),
    clearTimer = (timer) => globalThis.clearTimeout(timer),
    onPlaybackEnded = () => {},
  }) {
    if (!context || !destination) {
      throw new Error("an audio context and destination are required");
    }

    this.context = context;
    this.destination = destination;
    this.fadeSeconds = fadeSeconds;
    this.setTimer = setTimer;
    this.clearTimer = clearTimer;
    this.onPlaybackEnded = onPlaybackEnded;
    this.impulseResponse = null;
    this.source = null;
    this.sourceEnded = false;
    this.playbackStartedAt = 0;
    this.dryGain = null;
    this.wetGain = null;
    this.outputGain = null;
    this.branches = [];
    this.cleanupTimers = new Set();
    this.naturalStopTimer = 0;
  }

  get isPlaying() {
    return this.source !== null;
  }

  get isSourceActive() {
    return this.isPlaying && !this.sourceEnded;
  }

  setImpulseResponse(buffer) {
    if (!buffer) {
      throw new Error("an impulse-response buffer is required");
    }

    this.impulseResponse = buffer;
    if (!this.isSourceActive) {
      return false;
    }

    for (const timer of this.cleanupTimers) {
      this.clearTimer(timer);
    }
    this.cleanupTimers.clear();

    const now = this.context.currentTime;
    const oldBranches = [...this.branches];
    const nextBranch = this.createWetBranch(buffer, 0);
    this.branches.push(nextBranch);

    const { fadeOut, fadeIn } = equalPowerFadeCurves();
    for (const branch of oldBranches) {
      const gainAtStart = branchGainAt(branch, now);
      const curve = fadeOut.map((value) => value * gainAtStart);
      scheduleCurve(branch.gain.gain, curve, now, this.fadeSeconds);
      branch.automation = {
        kind: "out",
        startTime: now,
        duration: this.fadeSeconds,
        startGain: gainAtStart,
      };
    }

    scheduleCurve(nextBranch.gain.gain, fadeIn, now, this.fadeSeconds);
    nextBranch.automation = {
      kind: "in",
      startTime: now,
      duration: this.fadeSeconds,
      startGain: 0,
    };

    const cleanupTimer = this.setTimer(() => {
      this.cleanupTimers.delete(cleanupTimer);
      for (const branch of oldBranches) {
        this.disconnectBranch(branch);
      }
      this.branches = this.branches.filter(
        (branch) => !oldBranches.includes(branch),
      );
      nextBranch.automation = null;
      nextBranch.gain.gain.value = 1;
    }, Math.ceil(this.fadeSeconds * 1000) + 10);
    this.cleanupTimers.add(cleanupTimer);
    this.scheduleNaturalStop();
    return true;
  }

  play(dryBuffer, { wetMix = 0.72, gainLinear = 1 } = {}) {
    if (!dryBuffer) {
      throw new Error("a dry-source buffer is required");
    }
    if (!this.impulseResponse) {
      throw new Error("an impulse-response buffer is required before playback");
    }

    this.stop(false);
    this.source = this.context.createBufferSource();
    this.source.buffer = dryBuffer;
    this.source.loop = false;
    this.sourceEnded = false;
    this.playbackStartedAt = this.context.currentTime;

    this.dryGain = this.context.createGain();
    this.wetGain = this.context.createGain();
    this.outputGain = this.context.createGain();
    this.dryGain.gain.value = Math.sqrt(1 - clampUnit(wetMix));
    this.wetGain.gain.value = Math.sqrt(clampUnit(wetMix));
    this.outputGain.gain.value = Math.max(0, gainLinear);

    this.source.connect(this.dryGain);
    this.dryGain.connect(this.outputGain);
    this.wetGain.connect(this.outputGain);
    this.outputGain.connect(this.destination);

    this.branches = [this.createWetBranch(this.impulseResponse, 1)];
    this.source.onended = () => {
      this.sourceEnded = true;
    };
    this.source.start();
    this.scheduleNaturalStop();
  }

  setMix(wetMix, gainLinear) {
    if (!this.isPlaying) {
      return;
    }

    const now = this.context.currentTime;
    rampParameter(
      this.dryGain.gain,
      Math.sqrt(1 - clampUnit(wetMix)),
      now,
    );
    rampParameter(
      this.wetGain.gain,
      Math.sqrt(clampUnit(wetMix)),
      now,
    );
    rampParameter(this.outputGain.gain, Math.max(0, gainLinear), now);
  }

  stop(notify = true) {
    if (!this.isPlaying) {
      return;
    }

    if (this.naturalStopTimer) {
      this.clearTimer(this.naturalStopTimer);
      this.naturalStopTimer = 0;
    }
    for (const timer of this.cleanupTimers) {
      this.clearTimer(timer);
    }
    this.cleanupTimers.clear();

    this.source.onended = null;
    try {
      this.source.stop();
    } catch (error) {
      void error;
    }

    for (const branch of this.branches) {
      this.disconnectBranch(branch);
    }
    for (const node of [
      this.source,
      this.dryGain,
      this.wetGain,
      this.outputGain,
    ]) {
      disconnectNode(node);
    }

    this.source = null;
    this.sourceEnded = false;
    this.dryGain = null;
    this.wetGain = null;
    this.outputGain = null;
    this.branches = [];
    if (notify) {
      this.onPlaybackEnded();
    }
  }

  createWetBranch(buffer, gainValue) {
    const convolver = this.context.createConvolver();
    convolver.buffer = buffer;
    const gain = this.context.createGain();
    gain.gain.value = gainValue;
    this.source.connect(convolver);
    convolver.connect(gain);
    gain.connect(this.wetGain);
    return { convolver, gain, automation: null };
  }

  disconnectBranch(branch) {
    disconnectNode(this.source, branch.convolver);
    disconnectNode(branch.convolver);
    disconnectNode(branch.gain);
  }

  scheduleNaturalStop() {
    if (!this.isPlaying) {
      return;
    }
    if (this.naturalStopTimer) {
      this.clearTimer(this.naturalStopTimer);
    }

    const dryDuration = this.source.buffer?.duration ?? 0;
    const irDuration = Math.max(
      this.impulseResponse?.duration ?? 0,
      ...this.branches.map((branch) => branch.convolver.buffer?.duration ?? 0),
    );
    const elapsed = Math.max(0, this.context.currentTime - this.playbackStartedAt);
    const remaining = Math.max(
      0,
      dryDuration + irDuration + this.fadeSeconds - elapsed,
    );
    this.naturalStopTimer = this.setTimer(() => {
      this.naturalStopTimer = 0;
      this.stop(true);
    }, Math.ceil(remaining * 1000));
  }
}

function branchGainAt(branch, time) {
  const automation = branch.automation;
  if (!automation) {
    return branch.gain.gain.value;
  }

  const progress = Math.min(
    1,
    Math.max(0, (time - automation.startTime) / automation.duration),
  );
  if (automation.kind === "in") {
    return Math.sin(progress * (Math.PI / 2));
  }
  return automation.startGain * Math.cos(progress * (Math.PI / 2));
}

function scheduleCurve(parameter, curve, startTime, duration) {
  parameter.cancelScheduledValues(startTime);
  parameter.setValueAtTime(curve[0], startTime);
  parameter.setValueCurveAtTime(curve, startTime, duration);
}

function rampParameter(parameter, target, startTime) {
  parameter.cancelScheduledValues(startTime);
  parameter.setValueAtTime(parameter.value, startTime);
  parameter.linearRampToValueAtTime(
    target,
    startTime + DEFAULT_PARAMETER_RAMP_SECONDS,
  );
}

function disconnectNode(node, destination) {
  try {
    if (destination) {
      node?.disconnect?.(destination);
    } else {
      node?.disconnect?.();
    }
  } catch (error) {
    void error;
  }
}

function clampUnit(value) {
  return Math.min(1, Math.max(0, value));
}
