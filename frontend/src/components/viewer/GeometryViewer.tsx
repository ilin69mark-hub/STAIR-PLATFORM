// 3D-вьювер геометрии (FE-0017, ENG-GEO-0008): отображение preview mesh
// из снапшота. Вращение — ЛКМ, панорама — ПКМ/средняя, зум — колесо.

import { useEffect, useRef } from 'react'
import * as THREE from 'three'
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js'
import type { Mesh as ApiMesh } from '../../api/types'

interface Props {
  mesh: ApiMesh
}

export function GeometryViewer({ mesh }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const container = containerRef.current
    if (!container || mesh.Vertices.length === 0) return

    const width = container.clientWidth || 600
    const height = container.clientHeight || 380

    const scene = new THREE.Scene()
    scene.background = new THREE.Color('#f7f9fc')

    const camera = new THREE.PerspectiveCamera(45, width / height, 1, 100000)
    const renderer = new THREE.WebGLRenderer({ antialias: true })
    renderer.setSize(width, height)
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))
    container.appendChild(renderer.domElement)

    const controls = new OrbitControls(camera, renderer.domElement)
    controls.enableDamping = true
    controls.dampingFactor = 0.08

    scene.add(new THREE.HemisphereLight(0xffffff, 0xbfc8d8, 1))
    const dir = new THREE.DirectionalLight(0xffffff, 1.4)
    dir.position.set(2000, 4000, 3000)
    scene.add(dir)
    const dir2 = new THREE.DirectionalLight(0xffffff, 0.5)
    dir2.position.set(-2000, -1000, -3000)
    scene.add(dir2)

    const positions = new Float32Array(mesh.Vertices.length * 3)
    mesh.Vertices.forEach((v, i) => {
      positions[i * 3] = v.X
      positions[i * 3 + 1] = v.Y
      positions[i * 3 + 2] = v.Z
    })
    const indices = new Uint32Array(mesh.Triangles.length * 3)
    mesh.Triangles.forEach((t, i) => {
      indices[i * 3] = t[0]
      indices[i * 3 + 1] = t[1]
      indices[i * 3 + 2] = t[2]
    })

    const geo = new THREE.BufferGeometry()
    geo.setAttribute('position', new THREE.BufferAttribute(positions, 3))
    geo.setIndex(new THREE.BufferAttribute(indices, 1))
    geo.computeVertexNormals()
    geo.computeBoundingBox()

    const box = geo.boundingBox ?? new THREE.Box3(new THREE.Vector3(), new THREE.Vector3(1, 1, 1))
    const center = new THREE.Vector3()
    box.getCenter(center)
    const radius = Math.max(box.getBoundingSphere(new THREE.Sphere()).radius, 1000)

    const mat = new THREE.MeshStandardMaterial({
      color: 0x4f8df7,
      roughness: 0.55,
      metalness: 0.12,
      side: THREE.DoubleSide,
    })
    const stair = new THREE.Mesh(geo, mat)
    scene.add(stair)

    const edges = new THREE.LineSegments(
      new THREE.EdgesGeometry(geo),
      new THREE.LineBasicMaterial({ color: 0x1d4ed8 }),
    )
    scene.add(edges)

    const grid = new THREE.GridHelper(radius * 2.6, 24, 0x94a3b8, 0xcdd6e0)
    grid.position.y = box.min.y
    scene.add(grid)

    camera.position.copy(center).add(new THREE.Vector3(radius * 1.4, radius * 1.2, radius * 1.6))
    controls.target.copy(center)
    controls.update()

    renderer.setAnimationLoop(() => {
      controls.update()
      renderer.render(scene, camera)
    })

    const ro = new ResizeObserver(() => {
      const w = container.clientWidth || 600
      const h = container.clientHeight || 380
      camera.aspect = w / h
      camera.updateProjectionMatrix()
      renderer.setSize(w, h)
    })
    ro.observe(container)

    return () => {
      renderer.setAnimationLoop(null)
      ro.disconnect()
      controls.dispose()
      edges.geometry.dispose()
      ;(edges.material as THREE.Material).dispose()
      mat.dispose()
      geo.dispose()
      renderer.dispose()
      if (renderer.domElement.parentElement === container) {
        container.removeChild(renderer.domElement)
      }
    }
  }, [mesh])

  return (
    <div className="viewer">
      <div className="viewer__stage" ref={containerRef} />
      <p className="viewer__hint">Вращение — ЛКМ · панорама — ПКМ/средняя · зум — колесо</p>
    </div>
  )
}
