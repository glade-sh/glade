/** Decorative hero backdrop: a contour map drawn on the GPU. The pointer raises a hill that settles
    back after it passes, and the map drifts slowly when left alone. The static contour image stays
    as the first paint and as the fallback; this only takes over once a frame has been drawn. */
const TRAIL = 16
const VERTEX = 'attribute vec2 p; void main() { gl_Position = vec4(p, 0.0, 1.0); }'
const FRAGMENT = `#extension GL_OES_standard_derivatives : enable
precision highp float;
uniform vec2 uRes; uniform float uTime; uniform vec3 uTrail[${TRAIL}];
const float INTERVAL = 0.046; // keep in step with scripts/gen-hero-terrain.mjs
float hill(vec2 q, vec2 at, float r, float a, float i) {
  vec2 d = q - at - 0.05 * vec2(sin(uTime * (0.05 + 0.01 * i) + i * 1.7), cos(uTime * (0.04 + 0.008 * i) + i * 1.7));
  return a * exp(-dot(d, d) / (r * r));
}
float coverage(float f, float from, float to) { return 1.0 - smoothstep(from, to, abs(fract(f - 0.5) - 0.5) / max(fwidth(f), 1e-5)); }
void main() {
  float aspect = uRes.x / uRes.y;
  vec2 p = vec2(gl_FragCoord.x, uRes.y - gl_FragCoord.y) / uRes.y;
  // q is measured from the map centre in hero heights, the same frame scripts/gen-hero-terrain.mjs draws the still in.
  vec2 q = p - vec2(0.66 * aspect, 0.42);
  float v = 0.10 * sin(q.x * 2.1 + q.y * 1.3 + uTime * 0.035) + 0.18 * q.x;
  v += hill(q, vec2(0.0), 0.30, 0.55, 0.0) + hill(q, vec2(0.35, -0.20), 0.22, 0.45, 1.0) + hill(q, vec2(-0.30, 0.25), 0.26, 0.40, 2.0)
     + hill(q, vec2(0.50, 0.30), 0.20, 0.50, 3.0) + hill(q, vec2(-0.55, -0.15), 0.24, 0.35, 4.0) + hill(q, vec2(0.15, 0.45), 0.18, 0.40, 5.0)
     + hill(q, vec2(-0.15, -0.40), 0.20, 0.30, 6.0) + hill(q, vec2(0.75, -0.05), 0.28, 0.35, 7.0);
  float lift = 0.0;
  for (int i = 0; i < ${TRAIL}; i++) { vec2 d = p - uTrail[i].xy; lift += uTrail[i].z * exp(-dot(d, d) / 0.03); }
  lift = 0.09 * (1.0 - exp(-1.6 * lift));
  float f = (v + lift) / INTERVAL;
  // Every fourth level is an index contour, drawn heavier, as on a printed map.
  float line = max(0.62 * coverage(f, 0.15, 0.95), coverage(f * 0.25, 0.45, 1.4));
  // A faint phosphor bloom around each line and soft scanlines, as on a tube display.
  float bloom = 0.028 * coverage(f, 0.0, 4.5) + 0.028 * coverage(f * 0.25, 0.0, 5.5);
  float scan = mix(0.82, 1.0, smoothstep(0.0, 1.0, abs(mod(gl_FragCoord.y, 3.0) - 1.5)));
  float away = length(q);
  vec3 ink = mix(mix(vec3(0.56, 0.918, 0.95), vec3(0.50, 0.66, 1.0), smoothstep(0.0, 0.45, away)), vec3(0.60, 0.52, 1.0), smoothstep(0.45, 0.93, away));
  // The map fades out toward the copy; a raised hill shows faintly wherever the pointer is.
  float reach = max(1.0 - smoothstep(0.24, 0.75, away), min(0.13, lift * 2.2));
  // On start one soft scan passes down the screen: the canvas is revealed behind it (see home.css) and the lines lift slightly under it.
  float pass = exp(-pow((p.y - (uTime / 2.4 * 1.24 - 0.12)) * 7.0, 2.0)) * step(uTime, 2.6);
  float alpha = min(1.0, (line * 0.24 + bloom) * scan * reach * (1.0 + 0.45 * pass));
  gl_FragColor = vec4(ink * alpha, alpha);
}`

export function startHeroTerrain(hero: HTMLElement, canvas: HTMLCanvasElement): () => void {
  const fine = window.matchMedia('(hover: hover) and (pointer: fine)')
  const still = window.matchMedia('(prefers-reduced-motion: reduce)')
  if (!fine.matches || still.matches) return () => {}
  const gl = canvas.getContext('webgl', { alpha: true, antialias: false, depth: false, stencil: false, powerPreference: 'low-power' })
  if (!gl || !gl.getExtension('OES_standard_derivatives')) return () => {}

  const program = gl.createProgram()!
  for (const [type, source] of [[gl.VERTEX_SHADER, VERTEX], [gl.FRAGMENT_SHADER, FRAGMENT]] as const) {
    const shader = gl.createShader(type)!
    gl.shaderSource(shader, source); gl.compileShader(shader); gl.attachShader(program, shader)
  }
  gl.linkProgram(program)
  if (!gl.getProgramParameter(program, gl.LINK_STATUS)) return () => {}
  gl.useProgram(program)
  // One triangle that covers the viewport.
  gl.bindBuffer(gl.ARRAY_BUFFER, gl.createBuffer())
  gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 3, -1, -1, 3]), gl.STATIC_DRAW)
  gl.enableVertexAttribArray(0); gl.bindAttribLocation(program, 0, 'p'); gl.vertexAttribPointer(gl.getAttribLocation(program, 'p'), 2, gl.FLOAT, false, 0, 0)
  const uRes = gl.getUniformLocation(program, 'uRes'), uTime = gl.getUniformLocation(program, 'uTime'), uTrail = gl.getUniformLocation(program, 'uTrail')

  // trail[0] is the hill under the pointer; the rest are hills it left behind, each settling on its own.
  const trail = new Float32Array(TRAIL * 3)
  let width = 0, height = 0, next = 1
  let frame = 0, running = false, visible = true, shown = false, lost = false
  let px = -1, py = -1, last = 0, clock = 0, idle = false

  function layout() {
    const box = hero.getBoundingClientRect()
    width = box.width; height = box.height
    // The lines are soft by design, so a backing store below device resolution is enough.
    const scale = Math.max(0.75, Math.min(window.devicePixelRatio || 1, 2) * 0.6)
    canvas.width = Math.max(1, Math.round(width * scale)); canvas.height = Math.max(1, Math.round(height * scale))
    gl!.viewport(0, 0, canvas.width, canvas.height); gl!.uniform2f(uRes, canvas.width, canvas.height)
  }
  function draw(now: number) {
    frame = 0
    if (!running) return
    const dt = Math.min(64, last ? now - last : 16); last = now
    // With nothing but the slow drift to show, every other frame is plenty.
    let live = px >= 0
    for (let i = 5; i < trail.length; i += 3) if (trail[i] > 0.004) live = true
    idle = !live && trail[2] < 0.004 && !idle
    if (idle) { frame = requestAnimationFrame(draw); return }
    clock += dt / 1000
    const settle = Math.exp(-dt / 520)
    for (let i = 5; i < trail.length; i += 3) trail[i] *= settle
    if (px >= 0) {
      const x = px / height, y = py / height, follow = 1 - Math.exp(-dt / 70)
      if (trail[2] < 0.01) { trail[0] = x; trail[1] = y }
      trail[0] += (x - trail[0]) * follow; trail[1] += (y - trail[1]) * follow
      trail[2] += (1 - trail[2]) * (1 - Math.exp(-dt / 160))
      // Leave a hill behind once the pointer has moved on from the last one.
      const at = (next === 1 ? TRAIL - 1 : next - 1) * 3
      if (Math.hypot(trail[0] - trail[at], trail[1] - trail[at + 1]) > 0.075) {
        trail.set([trail[0], trail[1], trail[2] * 0.6], next * 3); trail[2] *= 0.55
        next = next === TRAIL - 1 ? 1 : next + 1
      }
    } else trail[2] *= settle
    gl!.uniform1f(uTime, clock); gl!.uniform3fv(uTrail, trail)
    gl!.drawArrays(gl!.TRIANGLES, 0, 3)
    if (!shown) { shown = true; canvas.dataset.live = ''; hero.dataset.terrain = '' }
    frame = requestAnimationFrame(draw)
  }
  function sync() {
    const want = !lost && visible && !document.hidden && fine.matches && !still.matches && window.innerWidth > 760
    // Narrow, coarse-pointer, reduced-motion and lost-context states go back to the static image. This runs
    // before the early return: the map may already be paused offscreen when one of them begins.
    if (!want && (lost || !fine.matches || still.matches || window.innerWidth <= 760)) { shown = false; delete canvas.dataset.live; delete hero.dataset.terrain }
    if (want === running) return
    running = want
    if (running) { last = 0; layout(); frame = requestAnimationFrame(draw) } else { cancelAnimationFrame(frame); frame = 0 }
  }
  function move(event: PointerEvent) { const box = hero.getBoundingClientRect(); px = event.clientX - box.left; py = event.clientY - box.top }
  function leave() { px = py = -1 }
  function drop(event: Event) { event.preventDefault(); lost = true; sync() }

  const seen = new IntersectionObserver((entries) => { visible = entries[entries.length - 1].isIntersecting; sync() })
  const sized = new ResizeObserver(() => { if (running) layout(); else sync() })
  seen.observe(hero); sized.observe(hero)
  hero.addEventListener('pointermove', move, { passive: true }); hero.addEventListener('pointerleave', leave)
  canvas.addEventListener('webglcontextlost', drop)
  document.addEventListener('visibilitychange', sync); fine.addEventListener('change', sync); still.addEventListener('change', sync)
  return () => {
    running = false; cancelAnimationFrame(frame)
    seen.disconnect(); sized.disconnect()
    hero.removeEventListener('pointermove', move); hero.removeEventListener('pointerleave', leave)
    canvas.removeEventListener('webglcontextlost', drop)
    document.removeEventListener('visibilitychange', sync); fine.removeEventListener('change', sync); still.removeEventListener('change', sync)
    delete canvas.dataset.live; delete hero.dataset.terrain
  }
}
