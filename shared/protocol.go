package shared

// Common message wrapper for JSON parsing
type MessageWrapper struct {
	Type    string `json:"type"`
	Payload []byte `json:"payload"`
}

// Client Messages

type ClientMoveMsg struct {
	PlayerID  uint64 `json:"player_id"`
	Position  Vec2   `json:"position"`
	TimeMs    uint32 `json:"time_ms"`
	Direction uint8  `json:"direction"`
	ColorR    uint8  `json:"color_r"`
	ColorG    uint8  `json:"color_g"`
	ColorB    uint8  `json:"color_b"`
}

type ClientAttackMsg struct {
	Direction uint8 `json:"direction"`
	Hit       bool  `json:"hit"`
}

type ClientInteractMsg struct {
	TargetID uint64 `json:"target_id"`
}

type ClientLoadLevelMsg struct {
	LevelName string `json:"level_name"`
}

type ClientChatMsg struct {
	Message string `json:"message"`
}

type ClientReportPlayerMsg struct {
	ReportedID uint64 `json:"reported_id"`
	Reason     string `json:"reason"`
}

// Server Messages

type ServerLevelLoadedMsg struct {
	Level Level `json:"level"`
}

type PlayerState struct {
	PlayerID  uint64 `json:"player_id"`
	Position  Vec2   `json:"position"`
	Animation uint8  `json:"animation"`
	Direction uint8  `json:"direction"`
	ColorR    uint8  `json:"color_r"`
	ColorG    uint8  `json:"color_g"`
	ColorB    uint8  `json:"color_b"`
}

type ServerPlayerStateMsg struct {
	PlayerState
}

type ServerPlayerStatesMsg struct {
	States []PlayerState `json:"states"`
}

type ServerAttackResultMsg struct {
	AttackerID uint64 `json:"attacker_id"`
	TargetID   uint64 `json:"target_id"` // 0 if none
	Damage     uint16 `json:"damage"`
}

type ServerChatMsg struct {
	PlayerID uint64 `json:"player_id"`
	Message  string `json:"message"`
}

type ServerMovementRejectedMsg struct {
	Reason string `json:"reason"`
}

type ServerBannedMsg struct {
	Reason string `json:"reason"`
}

// Types of Suspicion Events
type SuspicionEventType string

const (
	SuspicionEventTooFast      SuspicionEventType = "TooFast"
	SuspicionEventWallPhase    SuspicionEventType = "WallPhase"
	SuspicionEventPlayerReport SuspicionEventType = "PlayerReport"
)

type SuspicionEvent struct {
	Type   SuspicionEventType
	Speed  float32
	TileX  uint32
	TileY  uint32
	Reason string
}
