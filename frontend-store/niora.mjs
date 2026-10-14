import { chromium } from 'playwright'
const b = await chromium.launch({ args: ['--enable-unsafe-swiftshader'] })
const p = await b.newPage({ viewport: { width: 1440, height: 900 } })
const reqs = []
p.on('request', (r) => reqs.push(r.url()))
p.on('response', (r) => { if (r.status() >= 400) reqs.push(`!!${r.status()} ${r.url()}`) })
await p.goto('https://niora.ru/calculator/metal', { waitUntil: 'networkidle', timeout: 60000 }).catch(e => console.log('goto:', e.message.slice(0, 90)))
await p.waitForTimeout(6000)
const gl = await p.evaluate(() => {
  const c = document.querySelector('canvas')
  if (!c) return { canvas: false }
  const ctx = c.getContext('webgl2') || c.getContext('webgl')
  if (!ctx) return { canvas: true, gl: false }
  const d = ctx.getExtension('WEBGL_debug_renderer_info')
  return {
    canvas: true, w: c.width, h: c.height,
    vendor: d ? ctx.getParameter(d.UNMASKED_VENDOR_WEBGL) : ctx.getParameter(ctx.VENDOR),
    renderer: d ? ctx.getParameter(d.UNMASKED_RENDERER_WEBGL) : ctx.getParameter(ctx.RENDERER),
    version: ctx.getParameter(ctx.VERSION),
    maxTex: ctx.getParameter(ctx.MAX_TEXTURE_SIZE),
    ext: ctx.getSupportedExtensions().filter(e => /float|color_buffer|anisotropic|compressed|s3tc|astc|etc/i.test(e)).slice(0, 10),
  }
})
console.log('GL:', JSON.stringify(gl, null, 1))
const assets = [...new Set(reqs)].filter(u => /\.(hdr|exr|ktx2?|basis|glb|gltf|fbx|obj|bin|draco|wasm|json|webp|jpg|png|avif)(\?|$)/i.test(u))
console.log('ассеты сцены:', assets.length)
for (const a of assets.slice(0, 25)) console.log('  ', a.replace(/^https?:\/\/[^/]+/, '').slice(0, 110))
const scripts = [...new Set(reqs)].filter(u => /\.(m?js)(\?|$)/i.test(u))
console.log('скриптов:', scripts.length)
for (const s of scripts.slice(0, 12)) console.log('  ', s.replace(/^https?:\/\/[^/]+/, '').slice(0, 100))
await b.close()
