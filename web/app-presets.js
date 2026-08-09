export const MATERIALS = {
  defaultMaterial: {
    label: "Default Material",
    color: "#7385a3",
    absorption: [0.12, 0.12, 0.18, 0.24, 0.28, 0.32],
  },
  smoothConcrete: {
    label: "Smooth Concrete",
    color: "#a0a9b8",
    absorption: [0.02, 0.02, 0.03, 0.04, 0.05, 0.07],
  },
  plywoodPanels: {
    label: "Plywood Panels",
    color: "#c88c52",
    absorption: [0.16, 0.14, 0.12, 0.1, 0.09, 0.08],
  },
  glassWindow: {
    label: "Glass Window",
    color: "#6fb6d6",
    absorption: [0.18, 0.08, 0.05, 0.03, 0.02, 0.02],
  },
  pileCarpet: {
    label: "Pile Carpet",
    color: "#d05b55",
    absorption: [0.08, 0.14, 0.32, 0.58, 0.7, 0.72],
  },
  thinCarpet: {
    label: "Thin Carpet",
    color: "#f0a55f",
    absorption: [0.03, 0.05, 0.14, 0.24, 0.36, 0.39],
  },
  heavyCurtain: {
    label: "Heavy Curtain",
    color: "#6d8c72",
    absorption: [0.06, 0.1, 0.22, 0.48, 0.66, 0.7],
  },
  perforatedWood: {
    label: "Perforated Wood",
    color: "#9e6b43",
    absorption: [0.14, 0.22, 0.36, 0.42, 0.39, 0.3],
  },
};

// Transmission presets are kept separate from room-surface absorption
// materials so an open doorway cannot accidentally be selected as a wall.
export const PORTAL_MATERIALS = {
  concretePartition: {
    label: "Concrete partition",
    soundReductionIndex: [50, 50, 50, 50, 50, 50],
  },
  plasterboard: {
    label: "Plasterboard",
    soundReductionIndex: [35, 35, 35, 35, 35, 35],
  },
  woodenDoor: {
    label: "Wooden door",
    soundReductionIndex: [25, 25, 25, 25, 25, 25],
  },
  glassPartition: {
    label: "Glass partition",
    soundReductionIndex: [30, 30, 30, 30, 30, 30],
  },
  openDoorway: {
    label: "Open doorway",
    soundReductionIndex: [0, 0, 0, 0, 0, 0],
  },
};

export const ROOM_PRESETS = {
  custom: {
    label: "Custom",
    kind: "shoebox",
  },
  shoebox: makeShoeboxPreset(
    "Shoebox",
    { width: 6.4, depth: 4.8, height: 2.9 },
    {
      x: 1.4,
      y: 1.9,
      z: 1.25,
      gainDb: 0,
      directivity: "omni",
      azimuthDegrees: 18,
      cardioidOrder: 1.15,
    },
    { x: 4.85, y: 2.9, z: 1.2 },
    {
      materialPreset: "concertHall",
      renderMode: "hybrid",
    },
  ),
  classroom: makeShoeboxPreset(
    "Classroom",
    { width: 8.8, depth: 6.4, height: 3.1 },
    {
      x: 1.8,
      y: 1.9,
      z: 1.35,
      gainDb: 0,
      directivity: "omni",
      azimuthDegrees: 0,
      cardioidOrder: 1.1,
    },
    { x: 6.7, y: 3.7, z: 1.35 },
    {
      materialPreset: "studio",
      renderMode: "hybrid",
    },
  ),
  lectureHall: makeShoeboxPreset(
    "Lecture Hall",
    { width: 14.2, depth: 9.4, height: 5.1 },
    {
      x: 2.1,
      y: 3.0,
      z: 1.55,
      gainDb: -1,
      directivity: "omni",
      azimuthDegrees: 0,
      cardioidOrder: 1.1,
    },
    { x: 10.6, y: 4.7, z: 1.45 },
    {
      materialPreset: "concertHall",
      renderMode: "hybrid",
    },
  ),
  rehearsalRoom: makeShoeboxPreset(
    "Rehearsal Room",
    { width: 8.2, depth: 5.8, height: 3.0 },
    {
      x: 2.4,
      y: 2.2,
      z: 1.32,
      gainDb: -1,
      directivity: "cardioid",
      azimuthDegrees: 20,
      cardioidOrder: 1.2,
    },
    { x: 6.1, y: 3.4, z: 1.28 },
    {
      materialPreset: "studio",
      renderMode: "hybrid",
    },
  ),
  library: makeShoeboxPreset(
    "Library",
    { width: 11.2, depth: 7.0, height: 3.7 },
    {
      x: 2.3,
      y: 2.0,
      z: 1.28,
      gainDb: -2,
      directivity: "omni",
      azimuthDegrees: 0,
      cardioidOrder: 1.1,
    },
    { x: 8.1, y: 4.2, z: 1.22 },
    {
      materials: {
        west: "perforatedWood",
        east: "perforatedWood",
        south: "thinCarpet",
        north: "thinCarpet",
        floor: "pileCarpet",
        ceiling: "heavyCurtain",
      },
      renderMode: "hybrid",
    },
  ),
  chapel: makeShoeboxPreset(
    "Chapel",
    { width: 12.4, depth: 6.8, height: 7.2 },
    {
      x: 3.0,
      y: 2.0,
      z: 1.55,
      gainDb: -1,
      directivity: "cardioid",
      azimuthDegrees: 8,
      cardioidOrder: 1.4,
    },
    { x: 8.9, y: 3.5, z: 1.4 },
    {
      materialPreset: "concertHall",
      renderMode: "hybrid",
    },
  ),
  podcastBooth: makeShoeboxPreset(
    "Podcast Booth",
    { width: 3.8, depth: 2.8, height: 2.4 },
    {
      x: 1.1,
      y: 1.2,
      z: 1.2,
      gainDb: 0,
      directivity: "cardioid",
      azimuthDegrees: 15,
      cardioidOrder: 1.6,
    },
    { x: 2.9, y: 1.6, z: 1.18 },
    {
      materialPreset: "studio",
      renderMode: "early",
    },
  ),
  twoRoom: makeShoeboxPreset(
    "Two-room portal",
    { width: 5.625, depth: 4, height: 4 },
    {
      x: 1.5,
      y: 2,
      z: 1.4,
      gainDb: 0,
      directivity: "omni",
      azimuthDegrees: 0,
      cardioidOrder: 1.1,
    },
    // Receiver coordinates are local to the adjacent room. The browser
    // preview offsets that room along +X; the WASM adapter should do likewise.
    { x: 4.1, y: 2, z: 1.4 },
    {
      materialPreset: "studio",
      renderMode: "hybrid",
      portal: {
        enabled: true,
        aperture: 0,
        rootOrder: 2,
        material: "woodenDoor",
        receiverRoom: { width: 5.625, depth: 4, height: 4 },
        opening: { width: 1.2, height: 2.1, bottom: 0 },
      },
    },
  ),
  loft: makeMeshPreset(
    "Loft Atrium",
    { width: 8.6, depth: 6.4, height: 3.2 },
    makeTriangularPrismMesh(8.6, 6.4, 3.2),
    {
      x: 2.1,
      y: 1.8,
      z: 1.35,
      gainDb: -1,
      directivity: "cardioid",
      azimuthDegrees: 18,
      cardioidOrder: 1.35,
    },
    { x: 5.8, y: 3.7, z: 1.25 },
    {
      materialPreset: "concertHall",
      renderMode: "late",
    },
  ),
  wedgeHall: makeMeshPreset(
    "Wedge Hall",
    { width: 12.0, depth: 7.2, height: 5.4 },
    makeSlopedRoofMesh(12.0, 7.2, 3.0, 5.4),
    {
      x: 2.7,
      y: 2.2,
      z: 1.48,
      gainDb: -1,
      directivity: "omni",
      azimuthDegrees: 0,
      cardioidOrder: 1.1,
    },
    { x: 8.8, y: 4.1, z: 1.4 },
    {
      materialPreset: "concertHall",
      renderMode: "late",
    },
  ),
  cornerGallery: makeMeshPreset(
    "Corner Gallery",
    { width: 9.4, depth: 6.0, height: 4.1 },
    makeSlopedRoofMesh(9.4, 6.0, 2.8, 4.1),
    {
      x: 1.9,
      y: 1.7,
      z: 1.34,
      gainDb: -2,
      directivity: "cardioid",
      azimuthDegrees: 12,
      cardioidOrder: 1.2,
    },
    { x: 6.8, y: 3.3, z: 1.28 },
    {
      materials: {
        west: "smoothConcrete",
        east: "glassWindow",
        south: "perforatedWood",
        north: "perforatedWood",
        floor: "thinCarpet",
        ceiling: "heavyCurtain",
      },
      renderMode: "late",
    },
  ),
};

export const ROOM_PRESET_GROUPS = [
  {
    label: "Compact Rooms",
    presets: ["shoebox", "podcastBooth", "rehearsalRoom"],
  },
  {
    label: "Medium Rooms",
    presets: ["classroom", "library"],
  },
  {
    label: "Connected Rooms",
    presets: ["twoRoom"],
  },
  {
    label: "Large Rooms",
    presets: ["lectureHall", "chapel"],
  },
  {
    label: "Non-Rectangular",
    presets: ["loft", "wedgeHall", "cornerGallery"],
  },
];

export const MATERIAL_PRESETS = {
  custom: {
    label: "Custom",
  },
  concertHall: {
    label: "Concert Hall",
    materials: {
      west: "perforatedWood",
      east: "perforatedWood",
      south: "heavyCurtain",
      north: "heavyCurtain",
      floor: "plywoodPanels",
      ceiling: "heavyCurtain",
    },
  },
  studio: {
    label: "Studio",
    materials: {
      west: "thinCarpet",
      east: "thinCarpet",
      south: "plywoodPanels",
      north: "defaultMaterial",
      floor: "pileCarpet",
      ceiling: "heavyCurtain",
    },
  },
  bathroom: {
    label: "Bathroom",
    materials: {
      west: "glassWindow",
      east: "glassWindow",
      south: "smoothConcrete",
      north: "smoothConcrete",
      floor: "smoothConcrete",
      ceiling: "smoothConcrete",
    },
  },
};

export function makeShoeboxPreset(label, room, source, receiver, options = {}) {
  return {
    label,
    kind: "shoebox",
    room,
    source,
    receiver,
    ...options,
  };
}

export function makeMeshPreset(
  label,
  room,
  mesh,
  source,
  receiver,
  options = {},
) {
  return {
    label,
    kind: "mesh",
    room,
    mesh,
    source,
    receiver,
    ...options,
  };
}

// Room meshes are wound so that every triangle normal (v1-v0)×(v2-v0) points
// *into* the room. The Go mesh image-source solver depends on this: it only
// mirrors the source across planes it lies in front of (ism/mesh_image.go),
// so an outward-facing face contributes no specular reflections at all.
export function makeTriangularPrismMesh(width, depth, height) {
  return {
    triangles: [
      {
        v0: { x: 0, y: 0, z: 0 },
        v1: { x: width, y: 0, z: 0 },
        v2: { x: width / 2, y: depth, z: 0 },
      },
      {
        v0: { x: 0, y: 0, z: height },
        v1: { x: width / 2, y: depth, z: height },
        v2: { x: width, y: 0, z: height },
      },
      {
        v0: { x: 0, y: 0, z: 0 },
        v1: { x: width, y: 0, z: height },
        v2: { x: width, y: 0, z: 0 },
      },
      {
        v0: { x: 0, y: 0, z: 0 },
        v1: { x: 0, y: 0, z: height },
        v2: { x: width, y: 0, z: height },
      },
      {
        v0: { x: width, y: 0, z: 0 },
        v1: { x: width / 2, y: depth, z: height },
        v2: { x: width / 2, y: depth, z: 0 },
      },
      {
        v0: { x: width, y: 0, z: 0 },
        v1: { x: width, y: 0, z: height },
        v2: { x: width / 2, y: depth, z: height },
      },
      {
        v0: { x: width / 2, y: depth, z: 0 },
        v1: { x: 0, y: 0, z: height },
        v2: { x: 0, y: 0, z: 0 },
      },
      {
        v0: { x: width / 2, y: depth, z: 0 },
        v1: { x: width / 2, y: depth, z: height },
        v2: { x: 0, y: 0, z: height },
      },
    ],
  };
}

export function makeSlopedRoofMesh(width, depth, lowHeight, highHeight) {
  return {
    triangles: [
      {
        v0: { x: 0, y: 0, z: 0 },
        v1: { x: width, y: 0, z: 0 },
        v2: { x: width, y: depth, z: 0 },
      },
      {
        v0: { x: 0, y: 0, z: 0 },
        v1: { x: width, y: depth, z: 0 },
        v2: { x: 0, y: depth, z: 0 },
      },
      {
        v0: { x: 0, y: 0, z: lowHeight },
        v1: { x: width, y: depth, z: highHeight },
        v2: { x: width, y: 0, z: lowHeight },
      },
      {
        v0: { x: 0, y: 0, z: lowHeight },
        v1: { x: 0, y: depth, z: highHeight },
        v2: { x: width, y: depth, z: highHeight },
      },
      {
        v0: { x: 0, y: 0, z: 0 },
        v1: { x: width, y: 0, z: lowHeight },
        v2: { x: width, y: 0, z: 0 },
      },
      {
        v0: { x: 0, y: 0, z: 0 },
        v1: { x: 0, y: 0, z: lowHeight },
        v2: { x: width, y: 0, z: lowHeight },
      },
      {
        v0: { x: width, y: depth, z: 0 },
        v1: { x: width, y: 0, z: 0 },
        v2: { x: width, y: 0, z: lowHeight },
      },
      {
        v0: { x: width, y: depth, z: 0 },
        v1: { x: width, y: 0, z: lowHeight },
        v2: { x: width, y: depth, z: highHeight },
      },
      {
        v0: { x: 0, y: depth, z: 0 },
        v1: { x: 0, y: 0, z: lowHeight },
        v2: { x: 0, y: 0, z: 0 },
      },
      {
        v0: { x: 0, y: depth, z: 0 },
        v1: { x: 0, y: depth, z: highHeight },
        v2: { x: 0, y: 0, z: lowHeight },
      },
      {
        v0: { x: 0, y: depth, z: 0 },
        v1: { x: width, y: depth, z: 0 },
        v2: { x: width, y: depth, z: highHeight },
      },
      {
        v0: { x: 0, y: depth, z: 0 },
        v1: { x: width, y: depth, z: highHeight },
        v2: { x: 0, y: depth, z: highHeight },
      },
    ],
  };
}
