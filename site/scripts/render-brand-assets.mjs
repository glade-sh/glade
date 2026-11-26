import { readFile, writeFile } from 'node:fs/promises'
import { pathToFileURL } from 'node:url'
import { Resvg, initWasm } from '@resvg/resvg-wasm'
import { decompress } from 'wawoff2'

const publicDir = new URL('../docs-src/public/', import.meta.url)
const cardFonts = [
  '@fontsource-variable/inter/files/inter-latin-wght-normal.woff2',
  '@fontsource/ibm-plex-mono/files/ibm-plex-mono-latin-400-normal.woff2'
]
await initWasm(readFile(new URL(import.meta.resolve('@resvg/resvg-wasm/index_bg.wasm'))))

// Copy each Latin font out of the decoder's memory before decoding the next.
const fontBuffers = []
for (const name of cardFonts) {
  const font = await readFile(new URL(import.meta.resolve(name)))
  fontBuffers.push(Buffer.from(await decompress(font)))
}

export async function renderBrandAssets() {
  const [cardSVG, cardMarkSVG, iconSVG] = await Promise.all(
    ['social-card.svg', 'logo-mark.svg', 'logo-mark-topo.svg']
      .map(name => readFile(new URL(name, publicDir)))
  )
  // Resolve the source SVG's relative logo reference for the raster renderer.
  const cardMark = renderIcon(cardMarkSVG, 138).toString('base64')
  const card = cardSVG.toString('utf8').replace(
    'href="logo-mark.svg"', `href="data:image/png;base64,${cardMark}"`
  )

  return {
    'social-card.png': renderPNG(card, {
      font: { fontBuffers, sansSerifFamily: 'Inter', monospaceFamily: 'IBM Plex Mono' }
    }),
    'apple-touch-icon.png': renderIcon(iconSVG, 180),
    'favicon.ico': wrapICO(renderIcon(iconSVG, 32), 32)
  }
}

function renderIcon(svg, width) {
  return renderPNG(svg, { fitTo: { mode: 'width', value: width } })
}

function renderPNG(svg, options) {
  const renderer = new Resvg(svg, options)
  const image = renderer.render()
  const png = Buffer.from(image.asPng())
  image.free()
  renderer.free()
  return png
}

function wrapICO(png, size) {
  // ICO header and one directory entry pointing to the embedded PNG.
  const header = Buffer.alloc(22)
  header.writeUInt16LE(1, 2) // ICO file type
  header.writeUInt16LE(1, 4) // One image
  header[6] = size
  header[7] = size
  header.writeUInt16LE(1, 10) // One color plane
  header.writeUInt16LE(32, 12) // Bits per pixel
  header.writeUInt32LE(png.length, 14)
  header.writeUInt32LE(header.length, 18)
  return Buffer.concat([header, png])
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const assets = await renderBrandAssets()
  await Promise.all(Object.entries(assets).map(([name, data]) =>
    writeFile(new URL(name, publicDir), data)
  ))
}
