import * as THREE from "three";
import { OrbitControls } from "three/addons/controls/OrbitControls.js";

import { MATERIALS } from "./app-presets.js";
import {
  average,
  clamp,
  cssNumber,
  cssVar,
  isWithin,
} from "./app-utils.js";
import { getReflectionPreviewConfig } from "./reflection-preview.mjs";

let sceneState = null;
let sceneView = null;
let sceneChangeHandler = null;
let sceneRaycaster = null;
let scenePointer = null;
let sceneDragPlane = null;
let dragState = null;

export function setSceneContext(state, view = null) {
  sceneState = state;
  if (view) {
    sceneView = view;
  }
}

export function setSceneView(view) {
  sceneView = view;
}

export function setSceneChangeHandler(handler) {
  sceneChangeHandler = handler;
}

export function scenePalette() {
  return {
    gridMajor: cssVar("--scene-grid-major") || "#2f3d4a",
    gridMinor: cssVar("--scene-grid-minor") || "#1e2934",
    edge: cssVar("--scene-edge") || "#f3f5f8",
    ray: cssVar("--scene-ray") || "#f8fafc",
    path: cssVar("--scene-path") || "#f2efe7",
    key: cssVar("--scene-key") || "#fff2dc",
    fill: cssVar("--scene-fill") || "#7dd3c7",
    ambient: cssNumber("--scene-ambient", 0.8),
    keyIntensity: cssNumber("--scene-key-intensity", 1.3),
    fillIntensity: cssNumber("--scene-fill-intensity", 0.55),
  };
}

export function applySceneTheme() {
  if (!sceneView) {
    return;
  }

  const palette = scenePalette();
  sceneView.ambientLight.intensity = palette.ambient;
  sceneView.keyLight.color.set(palette.key);
  sceneView.keyLight.intensity = palette.keyIntensity;
  sceneView.fillLight.color.set(palette.fill);
  sceneView.fillLight.intensity = palette.fillIntensity;

  const mat = sceneView.grid.material;
  if (Array.isArray(mat)) {
    mat[0].color.set(palette.gridMajor);
    mat[1].color.set(palette.gridMinor);
  } else {
    mat.color.set(palette.gridMajor);
  }
}

export function createSceneView(canvas) {
  const renderer = new THREE.WebGLRenderer({
    canvas,
    antialias: true,
    alpha: true,
  });
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
  const clock = new THREE.Clock();
  const animatedPathMaterials = [];

  const scene = new THREE.Scene();
  const camera = new THREE.PerspectiveCamera(36, 1, 0.1, 100);
  camera.position.set(8.5, 6.5, 8.5);

  const controls = new OrbitControls(camera, canvas);
  controls.enableDamping = true;
  controls.autoRotate = true;
  controls.autoRotateSpeed = 0.4;
  if (sceneState) {
    controls.target.set(
      sceneState.room.width / 2,
      sceneState.room.height / 2,
      sceneState.room.depth / 2,
    );
  }

  const ambientLight = new THREE.AmbientLight(0xffffff, 0.8);
  scene.add(ambientLight);
  const keyLight = new THREE.DirectionalLight(0xfff2dc, 1.3);
  keyLight.position.set(8, 10, 6);
  scene.add(keyLight);
  const fillLight = new THREE.DirectionalLight(0x7dd3c7, 0.55);
  fillLight.position.set(-6, 4, -8);
  scene.add(fillLight);
  const grid = new THREE.GridHelper(30, 24, 0x2f3d4a, 0x1e2934);
  scene.add(grid);

  const roomGroup = new THREE.Group();
  scene.add(roomGroup);
  const pathGroup = new THREE.Group();
  scene.add(pathGroup);

  function resize() {
    const width = canvas.clientWidth;
    const height = canvas.clientHeight;
    if (!width || !height) {
      return;
    }
    renderer.setSize(width, height, false);
    camera.aspect = width / height;
    camera.updateProjectionMatrix();
  }

  function animate() {
    resize();
    const delta = clock.getDelta();
    for (const material of animatedPathMaterials) {
      if (material && typeof material.dashOffset === "number") {
        material.dashOffset -= delta * 0.8;
      }
    }
    if (sceneState) {
      controls.target.set(
        sceneState.room.width / 2,
        sceneState.room.height / 2,
        sceneState.room.depth / 2,
      );
    }
    controls.update();
    renderer.render(scene, camera);
    requestAnimationFrame(animate);
  }
  animate();

  return {
    scene,
    camera,
    controls,
    renderer,
    roomGroup,
    pathGroup,
    roomSurfaces: [],
    animatedPathMaterials,
    ambientLight,
    keyLight,
    fillLight,
    grid,
    resize,
    sourceMarker: null,
    receiverMarker: null,
    interactiveObjects: [],
  };
}

export function bindSceneInteractions(canvas) {
  sceneRaycaster = new THREE.Raycaster();
  scenePointer = new THREE.Vector2();
  sceneDragPlane = new THREE.Plane(new THREE.Vector3(0, 1, 0), 0);

  canvas.style.touchAction = "none";
  canvas.addEventListener("pointerdown", onScenePointerDown);
  canvas.addEventListener("pointermove", onScenePointerMove);
  canvas.addEventListener("pointerup", onScenePointerUp);
  canvas.addEventListener("pointercancel", onScenePointerUp);
  canvas.addEventListener("lostpointercapture", onScenePointerUp);
}

export function updateSceneView() {
  if (!sceneView || !sceneState) {
    return;
  }

  const { roomGroup, pathGroup } = sceneView;
  roomGroup.clear();
  pathGroup.clear();
  sceneView.roomSurfaces = [];
  sceneView.animatedPathMaterials.length = 0;

  const width = sceneState.room.width;
  const depth = sceneState.room.depth;
  const height = sceneState.room.height;
  const palette = scenePalette();

  if (sceneState.room.kind === "mesh" && sceneState.room.mesh?.triangles?.length) {
    const meshPreview = createMeshRoomPreview(sceneState.room.mesh, palette);
    roomGroup.add(meshPreview.mesh);
    roomGroup.add(meshPreview.edges);
    sceneView.roomSurfaces = [meshPreview.mesh];
  } else {
    const walls = [
      createWallMesh("west", depth, height, sceneState.materials.west, (mesh) => {
        mesh.rotation.y = Math.PI / 2;
        mesh.position.set(0, height / 2, depth / 2);
      }),
      createWallMesh("east", depth, height, sceneState.materials.east, (mesh) => {
        mesh.rotation.y = -Math.PI / 2;
        mesh.position.set(width, height / 2, depth / 2);
      }),
      createWallMesh("south", width, height, sceneState.materials.south, (mesh) => {
        mesh.position.set(width / 2, height / 2, 0);
      }),
      createWallMesh("north", width, height, sceneState.materials.north, (mesh) => {
        mesh.rotation.y = Math.PI;
        mesh.position.set(width / 2, height / 2, depth);
      }),
      createWallMesh("floor", width, depth, sceneState.materials.floor, (mesh) => {
        mesh.rotation.x = -Math.PI / 2;
        mesh.position.set(width / 2, 0, depth / 2);
      }),
      createWallMesh("ceiling", width, depth, sceneState.materials.ceiling, (mesh) => {
        mesh.rotation.x = Math.PI / 2;
        mesh.position.set(width / 2, height, depth / 2);
      }),
    ];

    walls.forEach((wall) => roomGroup.add(wall));
    sceneView.roomSurfaces = walls;

    const edgeBox = new THREE.LineSegments(
      new THREE.EdgesGeometry(new THREE.BoxGeometry(width, height, depth)),
      new THREE.LineBasicMaterial({
        color: palette.edge,
        transparent: true,
        opacity: 1,
      }),
    );
    edgeBox.position.set(width / 2, height / 2, depth / 2);
    roomGroup.add(edgeBox);
  }

  const reflections = getReflectionPreviewConfig(sceneState.reflections);
  const sourceMarker = createMarkerSphere(sceneState.source, 0xff6b4a, 0.12, "source");
  const receiverMarker = createMarkerSphere(sceneState.receiver, 0x0f9d92, 0.14, "receiver");
  const directPath = createDirectPathLine(sceneState, palette);
  directPath.visible = reflections.showDirectPath;
  pathGroup.add(directPath);
  sceneView.animatedPathMaterials.push(directPath.material);

  const rayPaths = createRayPathOverlay(sceneState, sceneView, palette);
  rayPaths.forEach((pathLine) => {
    pathGroup.add(pathLine);
    sceneView.animatedPathMaterials.push(pathLine.material);
  });

  roomGroup.add(sourceMarker);
  roomGroup.add(receiverMarker);

  if (sceneState.source.directivity === "cardioid") {
    roomGroup.add(createDirectivityCone(sceneState, palette));
  }

  sceneView.sourceMarker = sourceMarker;
  sceneView.receiverMarker = receiverMarker;
  sceneView.interactiveObjects = [sourceMarker, receiverMarker];
  sceneView.directPathLine = directPath;
}

function onScenePointerDown(event) {
  if (!sceneView || !sceneState) {
    return;
  }

  const target = pickDragTarget(event);
  if (!target) {
    return;
  }

  event.preventDefault();

  dragState = {
    target,
    pointerId: event.pointerId,
    planeY: target === "source" ? sceneState.source.z : sceneState.receiver.z,
  };

  sceneView.controls.enabled = false;
  canvasForScene().setPointerCapture(event.pointerId);
  canvasForScene().classList.add("is-dragging");
}

function onScenePointerMove(event) {
  if (!dragState || !sceneView || !sceneState) {
    return;
  }

  if (event.pointerId !== dragState.pointerId) {
    return;
  }

  event.preventDefault();

  const point = projectScenePointerToPlane(event, dragState.planeY);
  if (!point) {
    return;
  }

  if (dragState.target === "source") {
    sceneState.source.x = clamp(point.x, 0.15, sceneState.room.width - 0.15);
    sceneState.source.y = clamp(point.z, 0.15, sceneState.room.depth - 0.15);
  } else if (dragState.target === "receiver") {
    sceneState.receiver.x = clamp(point.x, 0.15, sceneState.room.width - 0.15);
    sceneState.receiver.y = clamp(point.z, 0.15, sceneState.room.depth - 0.15);
  }

  sceneChangeHandler?.();
}

function onScenePointerUp(event) {
  if (!dragState || event.pointerId !== dragState.pointerId) {
    return;
  }

  try {
    canvasForScene().releasePointerCapture(event.pointerId);
  } catch (error) {
    void error;
  }

  dragState = null;
  if (sceneView) {
    sceneView.controls.enabled = true;
  }
  canvasForScene().classList.remove("is-dragging");
}

function pickDragTarget(event) {
  if (!sceneView?.interactiveObjects?.length) {
    return null;
  }

  const rect = canvasForScene().getBoundingClientRect();
  scenePointer.x = ((event.clientX - rect.left) / rect.width) * 2 - 1;
  scenePointer.y = -(((event.clientY - rect.top) / rect.height) * 2 - 1);
  sceneRaycaster.setFromCamera(scenePointer, sceneView.camera);
  const hits = sceneRaycaster.intersectObjects(sceneView.interactiveObjects, false);
  if (!hits.length) {
    return null;
  }

  return hits[0].object.userData.dragTarget ?? null;
}

function projectScenePointerToPlane(event, planeY) {
  const rect = canvasForScene().getBoundingClientRect();
  scenePointer.x = ((event.clientX - rect.left) / rect.width) * 2 - 1;
  scenePointer.y = -(((event.clientY - rect.top) / rect.height) * 2 - 1);
  sceneRaycaster.setFromCamera(scenePointer, sceneView.camera);
  const plane = sceneDragPlane ?? new THREE.Plane(new THREE.Vector3(0, 1, 0), -planeY);
  plane.constant = -planeY;
  const point = new THREE.Vector3();
  const hit = sceneRaycaster.ray.intersectPlane(plane, point);
  return hit ? point : null;
}

function canvasForScene() {
  return sceneView?.renderer?.domElement ?? sceneView?.renderer?.canvas ?? null;
}

function createWallMesh(name, width, height, materialKey, configure) {
  const material = MATERIALS[materialKey];
  const averageAbsorption = average(material.absorption);
  const mesh = new THREE.Mesh(
    new THREE.PlaneGeometry(width, height),
    new THREE.MeshPhysicalMaterial({
      color: material.color,
      transparent: true,
      opacity: 0.16 + averageAbsorption * 0.26,
      roughness: 0.42,
      metalness: 0.05,
      side: THREE.DoubleSide,
      transmission: name === "east" ? 0.15 : 0,
    }),
  );
  configure(mesh);
  return mesh;
}

function createMeshRoomPreview(meshSpec, palette) {
  const geometry = new THREE.BufferGeometry();
  const positions = [];
  const triangles = meshSpec?.triangles ?? [];

  for (const tri of triangles) {
    for (const vertex of [tri.v0, tri.v1, tri.v2]) {
      positions.push(vertex.x, vertex.z, vertex.y);
    }
  }

  geometry.setAttribute("position", new THREE.Float32BufferAttribute(positions, 3));
  geometry.computeVertexNormals();

  const material = new THREE.MeshPhysicalMaterial({
    color: MATERIALS.perforatedWood.color,
    transparent: true,
    opacity: 0.22,
    roughness: 0.5,
    metalness: 0.02,
    side: THREE.DoubleSide,
  });

  const mesh = new THREE.Mesh(geometry, material);
  const edges = new THREE.LineSegments(
    new THREE.EdgesGeometry(geometry),
    new THREE.LineBasicMaterial({
      color: palette.edge,
      transparent: true,
      opacity: 1,
    }),
  );

  return { mesh, edges };
}

function createMarkerSphere(position, color, radius, target) {
  const marker = new THREE.Mesh(
    new THREE.SphereGeometry(radius, 28, 28),
    new THREE.MeshStandardMaterial({
      color,
      emissive: color,
      emissiveIntensity: 0.35,
    }),
  );
  marker.position.set(position.x, position.z, position.y);
  marker.userData.dragTarget = target;
  marker.userData.dragHeight = position.z;
  return marker;
}

function createDirectPathLine(state, palette) {
  const geometry = new THREE.BufferGeometry().setFromPoints([
    new THREE.Vector3(state.source.x, state.source.z, state.source.y),
    new THREE.Vector3(state.receiver.x, state.receiver.z, state.receiver.y),
  ]);
  const line = new THREE.Line(
    geometry,
    new THREE.LineDashedMaterial({
      color: palette.path,
      dashSize: 0.22,
      gapSize: 0.16,
      opacity: 1,
      transparent: true,
    }),
  );
  line.computeLineDistances();
  return line;
}

function createRayPathOverlay(state, view, palette) {
  const reflections = getReflectionPreviewConfig(state.reflections);
  if (!reflections.reflectionOrders.length) {
    return [];
  }

  if (state.room.kind === "shoebox") {
    return createShoeboxReflectionPaths(state, reflections.reflectionOrders, palette);
  }

  if (view?.roomSurfaces?.length) {
    return reflections.reflectionOrders.flatMap((order) => {
      const path = traceProbeRayPath(
        state,
        view.roomSurfaces,
        order,
        createProbeTargetPoint(state),
      );
      if (path.length >= 2) {
        return [makePathLine(path, palette.ray, order === 1 ? 0.72 : 0.58)];
      }
      return [];
    });
  }

  return [];
}

function createShoeboxReflectionPaths(state, reflectionOrders, palette) {
  const walls = buildShoeboxWalls(state);
  const paths = [];

  for (const order of reflectionOrders) {
    const path = pickBestReflectionPath(state, walls, order);
    if (path) {
      paths.push(makePathLine(path, palette.ray, order === 1 ? 0.8 : 0.62));
    }
  }

  return paths;
}

function pickBestReflectionPath(state, walls, order) {
  if (order <= 0) {
    return null;
  }

  const indices = walls.map((_, index) => index);
  let bestPath = null;
  let bestLength = Infinity;

  const visit = (sequence, depth) => {
    if (depth === order) {
      const path = buildReflectionPath(state, sequence, walls);
      if (!path || path.length < 2) {
        return;
      }

      const length = totalPathLength(path);
      if (length < bestLength) {
        bestLength = length;
        bestPath = path;
      }
      return;
    }

    for (const wallIndex of indices) {
      sequence.push(wallIndex);
      visit(sequence, depth + 1);
      sequence.pop();
    }
  };

  visit([], 0);
  return bestPath;
}

function buildReflectionPath(state, sequence, walls) {
  if (!sequence.length) {
    return null;
  }

  const source = toAcousticPoint(state.source);
  const receiver = toAcousticPoint(state.receiver);
  const images = [source];
  for (const wallIndex of sequence) {
    images.push(reflectAcousticPointAcrossWall(images[images.length - 1], walls[wallIndex]));
  }

  const hitsReverse = [];
  let currentReceiver = receiver;
  for (let index = sequence.length - 1; index >= 0; index -= 1) {
    const wall = walls[sequence[index]];
    const hit = intersectAcousticSegmentWithWall(currentReceiver, images[index + 1], wall);
    if (!hit) {
      return null;
    }

    hitsReverse.push(hit);
    currentReceiver = reflectAcousticPointAcrossWall(currentReceiver, wall);
  }

  const points = [toScenePoint(source)];
  for (let index = hitsReverse.length - 1; index >= 0; index -= 1) {
    points.push(toScenePoint(hitsReverse[index]));
  }
  points.push(toScenePoint(receiver));
  return points;
}

function buildShoeboxWalls(state) {
  return [
    { axis: "x", coord: 0, minA: 0, maxA: state.room.depth, minB: 0, maxB: state.room.height },
    { axis: "x", coord: state.room.width, minA: 0, maxA: state.room.depth, minB: 0, maxB: state.room.height },
    { axis: "y", coord: 0, minA: 0, maxA: state.room.width, minB: 0, maxB: state.room.height },
    { axis: "y", coord: state.room.depth, minA: 0, maxA: state.room.width, minB: 0, maxB: state.room.height },
    { axis: "z", coord: 0, minA: 0, maxA: state.room.width, minB: 0, maxB: state.room.depth },
    { axis: "z", coord: state.room.height, minA: 0, maxA: state.room.width, minB: 0, maxB: state.room.depth },
  ];
}

function totalPathLength(points) {
  let length = 0;
  for (let index = 1; index < points.length; index += 1) {
    length += points[index - 1].distanceTo(points[index]);
  }
  return length;
}

function toAcousticPoint(point) {
  return { x: point.x, y: point.y, z: point.z };
}

function toScenePoint(point) {
  return new THREE.Vector3(point.x, point.z, point.y);
}

function reflectAcousticPointAcrossWall(point, wall) {
  if (wall.axis === "x") {
    return { x: 2 * wall.coord - point.x, y: point.y, z: point.z };
  }
  if (wall.axis === "y") {
    return { x: point.x, y: 2 * wall.coord - point.y, z: point.z };
  }
  return { x: point.x, y: point.y, z: 2 * wall.coord - point.z };
}

function intersectAcousticSegmentWithWall(start, end, wall) {
  const direction = {
    x: end.x - start.x,
    y: end.y - start.y,
    z: end.z - start.z,
  };

  const denom = direction[wall.axis];
  if (Math.abs(denom) < 1e-9) {
    return null;
  }

  const t = (wall.coord - start[wall.axis]) / denom;
  if (t <= 0 || t >= 1) {
    return null;
  }

  const hit = {
    x: start.x + direction.x * t,
    y: start.y + direction.y * t,
    z: start.z + direction.z * t,
  };

  if (wall.axis === "x") {
    if (!isWithin(hit.y, wall.minA, wall.maxA) || !isWithin(hit.z, wall.minB, wall.maxB)) {
      return null;
    }
  } else if (wall.axis === "y") {
    if (!isWithin(hit.x, wall.minA, wall.maxA) || !isWithin(hit.z, wall.minB, wall.maxB)) {
      return null;
    }
  } else if (wall.axis === "z") {
    if (!isWithin(hit.x, wall.minA, wall.maxA) || !isWithin(hit.y, wall.minB, wall.maxB)) {
      return null;
    }
  }

  return hit;
}

function makePathLine(points, color, opacity) {
  const geometry = new THREE.BufferGeometry().setFromPoints(points);
  const material = new THREE.LineDashedMaterial({
    color,
    dashSize: 0.18,
    gapSize: 0.12,
    opacity,
    transparent: true,
  });
  const line = new THREE.Line(geometry, material);
  line.computeLineDistances();
  return line;
}

function createProbeTargetPoint(state) {
  const delta = new THREE.Vector3(
    state.receiver.x - state.source.x,
    state.receiver.z - state.source.z,
    state.receiver.y - state.source.y,
  );
  if (delta.lengthSq() === 0) {
    delta.set(1, 0, 0);
  }

  return new THREE.Vector3(
    state.receiver.x + delta.x,
    state.receiver.z + delta.y,
    state.receiver.y + delta.z,
  );
}

function traceProbeRayPath(state, roomSurfaces, maxBounces, targetPoint) {
  const points = [new THREE.Vector3(state.source.x, state.source.z, state.source.y)];
  if (!roomSurfaces?.length) {
    return points;
  }

  if (maxBounces <= 0) {
    return points;
  }

  const raycaster = new THREE.Raycaster();
  let origin = points[0].clone();
  const target = targetPoint ?? new THREE.Vector3(state.receiver.x, state.receiver.z, state.receiver.y);
  let direction = target.clone().sub(origin);
  if (direction.lengthSq() === 0) {
    direction.set(1, 0, 0);
  }
  direction.normalize();

  for (let bounce = 0; bounce < maxBounces; bounce += 1) {
    raycaster.set(origin, direction);
    const hits = raycaster.intersectObjects(roomSurfaces, false);
    const hit = hits.find((candidate) => candidate.distance > 1e-4);
    if (!hit) {
      break;
    }

    points.push(hit.point.clone());
    const normal = getWorldFaceNormal(hit);
    if (!normal) {
      break;
    }

    direction = reflectDirection(direction, normal).normalize();
    origin = hit.point.clone().add(direction.clone().multiplyScalar(1e-3));
  }

  return points;
}

function getWorldFaceNormal(hit) {
  if (!hit?.face) {
    return null;
  }

  const normal = hit.face.normal.clone();
  normal.transformDirection(hit.object.matrixWorld);
  return normal.normalize();
}

function reflectDirection(direction, normal) {
  const n = normal.clone().normalize();
  return direction.clone().sub(n.multiplyScalar(2 * direction.dot(n)));
}

function createDirectivityCone(state, palette) {
  const cone = new THREE.Mesh(
    new THREE.ConeGeometry(0.16, 0.42, 24, 1, true),
    new THREE.MeshStandardMaterial({
      color: palette.path,
      transparent: true,
      opacity: 0.32,
    }),
  );
  const azimuthRadians = (state.source.azimuthDegrees * Math.PI) / 180;
  const direction = new THREE.Vector3(
    Math.cos(azimuthRadians),
    0,
    Math.sin(azimuthRadians),
  );
  cone.position.set(state.source.x, state.source.z, state.source.y);
  cone.quaternion.setFromUnitVectors(
    new THREE.Vector3(0, 1, 0),
    direction.normalize(),
  );
  cone.position.add(direction.multiplyScalar(0.24));
  return cone;
}
