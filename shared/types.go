package shared

import "math"

// Vec2 represents a 2D vector
type Vec2 struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
}

func (v Vec2) DistanceTo(other Vec2) float32 {
	dx := v.X - other.X
	dy := v.Y - other.Y
	return float32(math.Sqrt(float64(dx*dx + dy*dy)))
}

type TileType byte

const (
	TileTypeBlocked    TileType = 0
	TileTypeWalkable   TileType = 1
	TileTypeFlyable    TileType = 2
	TileTypeJumpable   TileType = 3
	TileTypeCrouchable TileType = 4
	TileTypeSwimable   TileType = 5
	TileTypeLava       TileType = 6
)

func (t TileType) IsPassable() bool {
	switch t {
	case TileTypeWalkable, TileTypeFlyable, TileTypeJumpable, TileTypeCrouchable, TileTypeSwimable:
		return true
	default:
		return false
	}
}

type CollisionLayer struct {
	Width  uint32 `json:"width"`
	Height uint32 `json:"height"`
	Tiles  []byte `json:"tiles"` // Base64 encoded or byte array in JSON
}

func NewCollisionLayer(width, height uint32) CollisionLayer {
	tiles := make([]byte, width*height)
	for i := range tiles {
		tiles[i] = byte(TileTypeWalkable)
	}
	return CollisionLayer{Width: width, Height: height, Tiles: tiles}
}

func (c *CollisionLayer) Get(x, y uint32) TileType {
	if x >= c.Width || y >= c.Height {
		return TileTypeBlocked
	}
	return TileType(c.Tiles[y*c.Width+x])
}

func (c *CollisionLayer) Set(x, y uint32, tileType TileType) {
	if x < c.Width && y < c.Height {
		c.Tiles[y*c.Width+x] = byte(tileType)
	}
}

func (c *CollisionLayer) IsBlocked(x, y uint32) bool {
	return !c.Get(x, y).IsPassable()
}

type VisualLayer struct {
	Name   string `json:"name"`
	ZMin   uint32 `json:"z_min"`
	ZMax   uint32 `json:"z_max"`
	Width  uint32 `json:"width"`
	Height uint32 `json:"height"`
	Tiles  []uint16 `json:"tiles"`
}

type CollisionType int

const (
	CollisionTypeSolid       CollisionType = 0
	CollisionTypePassthrough CollisionType = 1
	CollisionTypePlatform    CollisionType = 2
)

type GameObject struct {
	ID            uint64            `json:"id"`
	Kind          string            `json:"kind"`
	X             float32           `json:"x"`
	Y             float32           `json:"y"`
	Z             uint32            `json:"z"`
	CollisionType CollisionType     `json:"collision_type"`
	Properties    map[string]string `json:"properties"`
}

type Level struct {
	Name          string         `json:"name"`
	Width         uint32         `json:"width"`
	Height        uint32         `json:"height"`
	Tileset       string         `json:"tileset"`
	Collision     CollisionLayer `json:"collision"`
	VisualLayers  []VisualLayer  `json:"visual_layers"`
	Objects       []GameObject   `json:"objects"`
}

const (
	TileSize               float32 = 16.0
	MaxSpeed               float32 = 5.0
	SpeedTolerance         float32 = 2.0
	NetworkTickRate        uint32  = 50 // ms (20Hz) - client send interval, server broadcast interval, and client interpolation window all derive from this
	SuspicionSpeedHack     float32 = 0.5
	SuspicionWallPhase     float32 = 2.0
	SuspicionPlayerReport  float32 = 0.5
	SuspicionCommunityFlag float32 = 1.0
	SuspicionEnableLogging float32 = 5.0
	SuspicionFlagReview    float32 = 8.0
	SuspicionAutoBan       float32 = 10.0
	ReportThresholdLog     uint32  = 5
)
