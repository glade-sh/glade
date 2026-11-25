import { readFile, writeFile } from 'node:fs/promises'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { Resvg } from '@resvg/resvg-js'

const publicDir = new URL('../docs-src/public/', import.meta.url)

export async function renderBrandAssets() {
  const [cardSVG, cardMarkSVG, iconSVG] = await Promise.all(
    ['social-card.svg', 'logo-mark.svg', 'logo-mark-topo.svg']
      .map(name => readFile(new URL(name, publicDir)))
  )
  const card = new Resvg(cardSVG)
  const cardMark = new Resvg(cardMarkSVG).render().asPng()
  card.resolveImage('logo-mark.svg', cardMark)

  const appleIcon = renderIcon(iconSVG, 180)
  const favicon = renderIcon(iconSVG, 32)
  return {
    'social-card.png': card.render().asPng(),
    'apple-touch-icon.png': appleIcon,
    'favicon.ico': wrapICO(favicon, 32)
  }
}

function renderIcon(svg, width) {
  return new Resvg(svg, { fitTo: { mode: 'width', value: width } }).render().asPng()
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
    writeFile(fileURLToPath(new URL(name, publicDir)), data)
  ))
}
