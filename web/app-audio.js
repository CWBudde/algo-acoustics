import { clamp, cssVar, dbToLinear } from "./app-utils.js";

export function waveformPalette(context, width) {
  const gradient = context.createLinearGradient(0, 0, width, 0);
  gradient.addColorStop(0, cssVar("--accent-2") || "#0f9d92");
  gradient.addColorStop(1, cssVar("--accent") || "#ff6b4a");
  return {
    background: cssVar("--wave-fill") || "#0d141b",
    grid: cssVar("--wave-grid") || "#f8fafc",
    empty: cssVar("--wave-empty") || "#ffffff",
    divider: cssVar("--wave-divider") || "#ffffff",
    trace: gradient,
  };
}

export function buildDrySourceBuffer(context, preset) {
  const sampleRate = context.sampleRate;
  const durationSeconds = preset === "music" ? 3.5 : preset === "speech" ? 2.4 : 1.4;
  const buffer = context.createBuffer(1, Math.ceil(durationSeconds * sampleRate), sampleRate);
  const data = buffer.getChannelData(0);

  if (preset === "clap") {
    for (let index = 0; index < data.length; index += 1) {
      const t = index / sampleRate;
      const burst = Math.exp(-t * 16);
      const noise = Math.sin(index * 12.9898) * 43758.5453;
      const n = (noise - Math.floor(noise)) * 2 - 1;
      data[index] = burst * n * (t < 0.12 ? 1 : 0.15);
    }
    return buffer;
  }

  if (preset === "music") {
    const notes = [261.63, 329.63, 392, 523.25];
    for (let index = 0; index < data.length; index += 1) {
      const t = index / sampleRate;
      const note = notes[Math.floor(t * 1.5) % notes.length];
      const env = Math.exp(-t * 0.8) * (0.92 + 0.08 * Math.sin(t * 2 * Math.PI * 0.5));
      data[index] =
        env *
        (0.42 * Math.sin(2 * Math.PI * note * t) +
          0.18 * Math.sin(2 * Math.PI * note * 2 * t) +
          0.08 * Math.sin(2 * Math.PI * note * 3 * t));
    }
    return buffer;
  }

  const phonemes = [
    { freq: 140, v1: 730, v2: 1090, amp: 1.0, dur: 0.32 },
    { freq: 155, v1: 530, v2: 1840, amp: 0.92, dur: 0.36 },
    { freq: 170, v1: 300, v2: 2240, amp: 0.86, dur: 0.34 },
    { freq: 150, v1: 600, v2: 870, amp: 0.88, dur: 0.36 },
  ];

  let cursor = 0;
  for (const phoneme of phonemes) {
    const start = cursor;
    const end = Math.min(data.length, start + Math.floor(phoneme.dur * sampleRate));
    for (let index = start; index < end; index += 1) {
      const t = (index - start) / sampleRate;
      const env = Math.exp(-t * 5.5);
      const base = Math.sin(2 * Math.PI * phoneme.freq * (index / sampleRate));
      const formant =
        0.55 * Math.sin(2 * Math.PI * phoneme.v1 * (index / sampleRate)) +
        0.35 * Math.sin(2 * Math.PI * phoneme.v2 * (index / sampleRate)) +
        0.1 * Math.sin(2 * Math.PI * phoneme.v2 * 1.5 * (index / sampleRate));
      data[index] += phoneme.amp * env * (0.45 * base + 0.55 * formant);
    }
    cursor = end + Math.floor(0.05 * sampleRate);
  }

  return buffer;
}

export function encodeAudioBufferToWav(audioBuffer) {
  const channels = audioBuffer.numberOfChannels;
  const sampleRate = audioBuffer.sampleRate;
  const frameCount = audioBuffer.length;
  const bytesPerSample = 2;
  const blockAlign = channels * bytesPerSample;
  const byteRate = sampleRate * blockAlign;
  const dataSize = frameCount * blockAlign;
  const buffer = new ArrayBuffer(44 + dataSize);
  const view = new DataView(buffer);
  let offset = 0;

  const writeString = (text) => {
    for (let index = 0; index < text.length; index += 1) {
      view.setUint8(offset + index, text.charCodeAt(index));
    }
    offset += text.length;
  };

  writeString("RIFF");
  view.setUint32(offset, 36 + dataSize, true);
  offset += 4;
  writeString("WAVE");
  writeString("fmt ");
  view.setUint32(offset, 16, true);
  offset += 4;
  view.setUint16(offset, 1, true);
  offset += 2;
  view.setUint16(offset, channels, true);
  offset += 2;
  view.setUint32(offset, sampleRate, true);
  offset += 4;
  view.setUint32(offset, byteRate, true);
  offset += 4;
  view.setUint16(offset, blockAlign, true);
  offset += 2;
  view.setUint16(offset, 16, true);
  offset += 2;
  writeString("data");
  view.setUint32(offset, dataSize, true);
  offset += 4;

  const channelData = [];
  for (let channel = 0; channel < channels; channel += 1) {
    channelData.push(audioBuffer.getChannelData(channel));
  }

  for (let frame = 0; frame < frameCount; frame += 1) {
    for (let channel = 0; channel < channels; channel += 1) {
      const sample = clamp(channelData[channel][frame], -1, 1);
      view.setInt16(offset, Math.round(sample * 0x7fff), true);
      offset += 2;
    }
  }

  return new Uint8Array(buffer);
}
