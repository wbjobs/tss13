package server

type MessageType int

const (
	MsgHello       MessageType = 0
	MsgMapData     MessageType = 1
	MsgEntities    MessageType = 2
	MsgPlayerMove  MessageType = 3
	MsgPlayerInfo  MessageType = 4
	MsgCombat      MessageType = 5
	MsgChat        MessageType = 6
	MsgError       MessageType = 7
	MsgTurnUpdate  MessageType = 8
	MsgMapUpdate   MessageType = 9
)

type ClientMessage struct {
	Type MessageType `json:"type"`
	Data interface{} `json:"data"`
}

type ServerMessage struct {
	Type MessageType `json:"type"`
	Data interface{} `json:"data"`
}

type HelloMessage struct {
	PlayerID   int    `json:"playerId"`
	PlayerName string `json:"playerName"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

type TileInfo struct {
	X          int     `json:"x"`
	Y          int     `json:"y"`
	Tile       int     `json:"tile"`
	Brightness float64 `json:"brightness"`
}

type MapDataMessage struct {
	Tiles  []TileInfo `json:"tiles"`
	Width  int        `json:"width"`
	Height int        `json:"height"`
}

type EntityInfo struct {
	ID      int    `json:"id"`
	Type    int    `json:"type"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
	HP      int    `json:"hp"`
	MaxHP   int    `json:"maxHp"`
	Name    string `json:"name"`
	Symbol  string `json:"symbol"`
	Level   int    `json:"level,omitempty"`
}

type EntitiesMessage struct {
	Entities []EntityInfo `json:"entities"`
}

type PlayerMoveMessage struct {
	DX int `json:"dx"`
	DY int `json:"dy"`
}

type PlayerInfoMessage struct {
	ID        int    `json:"id"`
	HP        int    `json:"hp"`
	MaxHP     int    `json:"maxHp"`
	Attack    int    `json:"attack"`
	Defense   int    `json:"defense"`
	Level     int    `json:"level"`
	XP        int    `json:"xp"`
	XPToNext  int    `json:"xpToNext"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Name      string `json:"name"`
}

type CombatMessage struct {
	AttackerID   int    `json:"attackerId"`
	DefenderID   int    `json:"defenderId"`
	AttackerName string `json:"attackerName"`
	DefenderName string `json:"defenderName"`
	Hit          bool   `json:"hit"`
	Damage       int    `json:"damage"`
	DefenderHP   int    `json:"defenderHp"`
	DefenderDead bool   `json:"defenderDead"`
	Crit         bool   `json:"crit"`
	Roll         int    `json:"roll"`
}

type TurnUpdateMessage struct {
	Turn int `json:"turn"`
}

type ChatMessage struct {
	PlayerID   int    `json:"playerId"`
	PlayerName string `json:"playerName"`
	Text       string `json:"text"`
}

type ErrorMessage struct {
	Text string `json:"text"`
}
