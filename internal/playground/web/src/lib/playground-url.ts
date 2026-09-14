export const playgroundSurfaces = ["apex", "visualforce", "lwc"] as const

export type PlaygroundSurface = (typeof playgroundSurfaces)[number]

export type PlaygroundURLState = {
  surface: PlaygroundSurface
  example?: string
  file?: string
}
function surfaceValue(value: string | null): PlaygroundSurface {
  return playgroundSurfaces.includes(value as PlaygroundSurface) ? (value as PlaygroundSurface) : "apex"
}

export function parsePlaygroundURL(input: string): PlaygroundURLState {
  const url = new URL(input, "http://localhost")
  const hash = new URLSearchParams(url.hash.replace(/^#/, ""))
  const legacyExample = hash.get("example") || (url.hash.startsWith("#example=") ? url.hash.slice("#example=".length) : "")
  const example = url.searchParams.get("example") || legacyExample
  const file = url.searchParams.get("file")
  return {
    surface: surfaceValue(url.searchParams.get("surface")),
    ...(example ? { example } : {}),
    ...(file ? { file } : {}),
  }
}

export function canonicalPlaygroundURL(input: string, state: PlaygroundURLState) {
  const url = new URL(input, "http://localhost")
  url.search = ""
  url.hash = ""
  url.searchParams.set("surface", state.surface)
  if (state.example) url.searchParams.set("example", state.example)
  if (state.file) url.searchParams.set("file", state.file)
  return url
}

export function replacePlaygroundURL(state: PlaygroundURLState) {
  if (typeof window === "undefined") return
  const url = canonicalPlaygroundURL(window.location.href, state)
  window.history.replaceState({}, "", `${url.pathname}${url.search}`)
}
