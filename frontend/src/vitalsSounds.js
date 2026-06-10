let audioContext = null

function getAudioContext() {
  if (typeof window === 'undefined') return null
  if (!audioContext) audioContext = new AudioContext()
  if (audioContext.state === 'suspended') audioContext.resume().catch(() => {})
  return audioContext
}

function playTone({ frequency = 440, type = 'sine', duration = 0.5, volume = 0.12, delay = 0 }) {
  const ctx = getAudioContext()
  if (!ctx) return
  try {
    const osc = ctx.createOscillator()
    const gain = ctx.createGain()
    osc.connect(gain)
    gain.connect(ctx.destination)
    osc.frequency.value = frequency
    osc.type = type
    const start = ctx.currentTime + delay
    const end = start + duration
    gain.gain.setValueAtTime(volume, start)
    gain.gain.exponentialRampToValueAtTime(0.001, end)
    osc.start(start)
    osc.stop(end)
  } catch {
    // ignore autoplay / audio errors
  }
}

/** Stamina full: original tone twice. */
export function playStaminaFullSound() {
  const tone = { frequency: 660, type: 'sine', duration: 0.45, volume: 0.14 }
  playTone({ ...tone, delay: 0 })
  playTone({ ...tone, delay: 0.25 })
}

/** Low stamina: lower tone, distinct from full. */
export function playStaminaLowSound() {
  playTone({ frequency: 220, type: 'sawtooth', duration: 0.55, volume: 0.1 })
}
