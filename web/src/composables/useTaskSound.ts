import { shallowRef } from "vue";
import { readStoredString, writeStoredString } from "../lib/persistence";

type SoundKind = "success" | "error" | "cancelled";

const storageKey = "aicli.workflow.sound.enabled";
const enabled = shallowRef(readStoredString(storageKey, "true") !== "false");
const available = shallowRef(typeof window !== "undefined" && typeof window.AudioContext !== "undefined");
let audioContext: AudioContext | null = null;

export function useTaskSound() {
  function setEnabled(value: boolean) {
    enabled.value = value;
    writeStoredString(storageKey, value ? "true" : "false");
    if (value) void unlockSound();
  }

  async function unlockSound() {
    if (!available.value) return;
    const ctx = context();
    if (!ctx) return;
    if (ctx.state === "suspended") {
      await ctx.resume().catch(() => undefined);
    }
  }

  async function play(kind: SoundKind) {
    if (!enabled.value || !available.value) return;
    const ctx = context();
    if (!ctx) return;
    if (ctx.state === "suspended") {
      await ctx.resume().catch(() => undefined);
    }
    if (ctx.state !== "running") return;
    playPattern(ctx, kind);
  }

  async function testSound() {
    await unlockSound();
    await play("success");
  }

  return {
    available,
    enabled,
    setEnabled,
    unlockSound,
    play,
    testSound,
  };
}

function context(): AudioContext | null {
  if (audioContext) return audioContext;
  try {
    audioContext = new window.AudioContext();
    return audioContext;
  } catch {
    available.value = false;
    return null;
  }
}

function playPattern(ctx: AudioContext, kind: SoundKind) {
  const tones =
    kind === "success"
      ? [
          { frequency: 660, offset: 0, duration: 0.12 },
          { frequency: 880, offset: 0.14, duration: 0.16 },
        ]
      : kind === "cancelled"
        ? [{ frequency: 440, offset: 0, duration: 0.18 }]
        : [
            { frequency: 220, offset: 0, duration: 0.16 },
            { frequency: 180, offset: 0.18, duration: 0.2 },
          ];
  for (const tone of tones) {
    playTone(ctx, tone.frequency, tone.offset, tone.duration);
  }
}

function playTone(ctx: AudioContext, frequency: number, offset: number, duration: number) {
  const oscillator = ctx.createOscillator();
  const gain = ctx.createGain();
  const start = ctx.currentTime + offset;
  const end = start + duration;

  oscillator.type = "sine";
  oscillator.frequency.setValueAtTime(frequency, start);
  gain.gain.setValueAtTime(0.0001, start);
  gain.gain.exponentialRampToValueAtTime(0.16, start + 0.02);
  gain.gain.exponentialRampToValueAtTime(0.0001, end);
  oscillator.connect(gain);
  gain.connect(ctx.destination);
  oscillator.start(start);
  oscillator.stop(end + 0.03);
}
