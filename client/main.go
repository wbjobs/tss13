package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font/basicfont"
)

const (
	TileSize     = 24
	ViewWidth    = 40
	ViewHeight   = 28
	ScreenWidth  = TileSize * ViewWidth
	ScreenHeight = TileSize * ViewHeight
)

const (
	MsgHello       = 0
	MsgMapData     = 1
	MsgEntities    = 2
	MsgPlayerMove  = 3
	MsgPlayerInfo  = 4
	MsgCombat      = 5
	MsgChat        = 6
	MsgError       = 7
	MsgTurnUpdate  = 8
	MsgMapUpdate   = 9
)

type ClientMessage struct {
	Type int         `json:"type"`
	Data interface{} `json:"data"`
}

type ServerMessage struct {
	Type int             `json:"type"`
	Data json.RawMessage `json:"data"`
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
	ID     int    `json:"id"`
	Type   int    `json:"type"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	HP     int    `json:"hp"`
	MaxHP  int    `json:"maxHp"`
	Name   string `json:"name"`
	Symbol string `json:"symbol"`
	Level  int    `json:"level,omitempty"`
}

type EntitiesMessage struct {
	Entities []EntityInfo `json:"entities"`
}

type PlayerMoveMessage struct {
	DX int `json:"dx"`
	DY int `json:"dy"`
}

type PlayerInfoMessage struct {
	ID       int    `json:"id"`
	HP       int    `json:"hp"`
	MaxHP    int    `json:"maxHp"`
	Attack   int    `json:"attack"`
	Defense  int    `json:"defense"`
	Level    int    `json:"level"`
	XP       int    `json:"xp"`
	XPToNext int    `json:"xpToNext"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Name     string `json:"name"`
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

type Game struct {
	conn       *websocket.Conn
	playerInfo *PlayerInfoMessage
	tiles      map[[2]int]TileInfo
	entities   map[int]EntityInfo
	knownTiles map[[2]int]TileInfo
	playerName string
	serverAddr string
	mu         sync.Mutex
	combatLog  []string
	turn       int
	connected  bool
	mapWidth   int
	mapHeight  int
}

func NewGame(serverAddr, playerName string) *Game {
	return &Game{
		tiles:      make(map[[2]int]TileInfo),
		entities:   make(map[int]EntityInfo),
		knownTiles: make(map[[2]int]TileInfo),
		playerName: playerName,
		serverAddr: serverAddr,
		combatLog:  make([]string, 0, 10),
	}
}

func (g *Game) Connect() error {
	url := fmt.Sprintf("ws://%s/ws", g.serverAddr)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}
	g.conn = conn
	g.connected = true

	hello := HelloMessage{PlayerName: g.playerName}
	msg := ClientMessage{Type: MsgHello, Data: hello}
	if err := conn.WriteJSON(msg); err != nil {
		return err
	}

	go g.readLoop()
	return nil
}

func (g *Game) readLoop() {
	defer g.conn.Close()
	for {
		var msg ServerMessage
		err := g.conn.ReadJSON(&msg)
		if err != nil {
			log.Println("read error:", err)
			g.mu.Lock()
			g.connected = false
			g.mu.Unlock()
			return
		}
		g.handleMessage(msg)
	}
}

func (g *Game) handleMessage(msg ServerMessage) {
	g.mu.Lock()
	defer g.mu.Unlock()

	switch msg.Type {
	case MsgHello:
		var h HelloMessage
		if err := json.Unmarshal(msg.Data, &h); err == nil {
			g.mapWidth = h.Width
			g.mapHeight = h.Height
			g.addLog(fmt.Sprintf("Connected as %s (map %dx%d)", h.PlayerName, h.Width, h.Height))
		}
	case MsgMapData:
		var md MapDataMessage
		if err := json.Unmarshal(msg.Data, &md); err == nil {
			g.tiles = make(map[[2]int]TileInfo, len(md.Tiles))
			for _, t := range md.Tiles {
				key := [2]int{t.X, t.Y}
				g.tiles[key] = t
				g.knownTiles[key] = t
			}
		}
	case MsgEntities:
		var em EntitiesMessage
		if err := json.Unmarshal(msg.Data, &em); err == nil {
			g.entities = make(map[int]EntityInfo, len(em.Entities))
			for _, e := range em.Entities {
				g.entities[e.ID] = e
			}
		}
	case MsgPlayerInfo:
		var pi PlayerInfoMessage
		if err := json.Unmarshal(msg.Data, &pi); err == nil {
			g.playerInfo = &pi
		}
	case MsgCombat:
		var cm CombatMessage
		if err := json.Unmarshal(msg.Data, &cm); err == nil {
			if cm.Hit {
				critStr := ""
				if cm.Crit {
					critStr = " CRIT!"
				}
				deadStr := ""
				if cm.DefenderDead {
					deadStr = " (KILLED)"
				}
				g.addLog(fmt.Sprintf("%s hits %s for %d (roll %d)%s%s", cm.AttackerName, cm.DefenderName, cm.Damage, cm.Roll, critStr, deadStr))
			} else {
				g.addLog(fmt.Sprintf("%s misses %s (roll %d)", cm.AttackerName, cm.DefenderName, cm.Roll))
			}
		}
	case MsgTurnUpdate:
		var tu TurnUpdateMessage
		if err := json.Unmarshal(msg.Data, &tu); err == nil {
			g.turn = tu.Turn
		}
	case MsgChat:
		var cm ChatMessage
		if err := json.Unmarshal(msg.Data, &cm); err == nil {
			g.addLog(fmt.Sprintf("[%s] %s", cm.PlayerName, cm.Text))
		}
	}
}

func (g *Game) addLog(s string) {
	g.combatLog = append(g.combatLog, s)
	if len(g.combatLog) > 10 {
		g.combatLog = g.combatLog[len(g.combatLog)-10:]
	}
}

func (g *Game) sendMove(dx, dy int) {
	if g.conn == nil {
		return
	}
	msg := ClientMessage{Type: MsgPlayerMove, Data: PlayerMoveMessage{DX: dx, DY: dy}}
	g.conn.WriteJSON(msg)
}

func (g *Game) Update() error {
	var dx, dy int
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) || inpututil.IsKeyJustPressed(ebiten.KeyK) {
		dy = -1
	} else if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) || inpututil.IsKeyJustPressed(ebiten.KeyJ) {
		dy = 1
	} else if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) || inpututil.IsKeyJustPressed(ebiten.KeyA) || inpututil.IsKeyJustPressed(ebiten.KeyH) {
		dx = -1
	} else if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) || inpututil.IsKeyJustPressed(ebiten.KeyD) || inpututil.IsKeyJustPressed(ebiten.KeyL) {
		dx = 1
	} else if inpututil.IsKeyJustPressed(ebiten.KeyY) {
		dx, dy = -1, -1
	} else if inpututil.IsKeyJustPressed(ebiten.KeyU) {
		dx, dy = 1, -1
	} else if inpututil.IsKeyJustPressed(ebiten.KeyB) {
		dx, dy = -1, 1
	} else if inpututil.IsKeyJustPressed(ebiten.KeyN) {
		dx, dy = 1, 1
	}

	if dx != 0 || dy != 0 {
		g.sendMove(dx, dy)
	}

	return nil
}

func rgba(r, g, b, a float64) color.RGBA {
	return color.RGBA{
		R: uint8(clamp(r) * 255),
		G: uint8(clamp(g) * 255),
		B: uint8(clamp(b) * 255),
		A: uint8(clamp(a) * 255),
	}
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func tileBaseColor(tileType int) (float64, float64, float64) {
	switch tileType {
	case 0:
		return 0.25, 0.22, 0.30
	case 1:
		return 0.55, 0.50, 0.40
	case 2:
		return 0.40, 0.35, 0.30
	case 3:
		return 0.70, 0.45, 0.20
	case 4:
		return 0.55, 0.25, 0.25
	case 5:
		return 0.35, 0.55, 0.65
	case 6:
		return 0.65, 0.55, 0.35
	default:
		return 0.5, 0.5, 0.5
	}
}

func drawRect(screen *ebiten.Image, x, y, w, h float64, c color.Color) {
	ebitenutil.DrawRect(screen, x, y, w, h, c)
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.mu.Lock()
	defer g.mu.Unlock()

	var camX, camY int
	if g.playerInfo != nil {
		camX = g.playerInfo.X - ViewWidth/2
		camY = g.playerInfo.Y - ViewHeight/2
	}

	tileSizeF := float64(TileSize)

	for vy := 0; vy < ViewHeight; vy++ {
		for vx := 0; vx < ViewWidth; vx++ {
			tx := camX + vx
			ty := camY + vy
			key := [2]int{tx, ty}
			px := float64(vx * TileSize)
			py := float64(vy * TileSize)

			vt, visible := g.tiles[key]
			kt, known := g.knownTiles[key]

			if visible {
				r, gc, b := tileBaseColor(vt.Tile)
				br := vt.Brightness
				if br < 0.1 {
					br = 0.1
				}
				col := rgba(r*br, gc*br, b*br, 1.0)
				drawRect(screen, px, py, tileSizeF-1, tileSizeF-1, col)

				if vt.Tile == 4 {
					drawRect(screen, px+tileSizeF/2-2, py+tileSizeF/2-2, 4, 4, rgba(1, 0, 0, 0.8))
				}
				if vt.Tile == 5 || vt.Tile == 6 {
					sym := ">"
					if vt.Tile == 6 {
						sym = "<"
					}
					text.Draw(screen, sym, basicfont.Face7x13, int(px)+6, int(py)+16, color.White)
				}
				if vt.Tile == 3 {
					drawRect(screen, px+3, py+3, tileSizeF-6, tileSizeF-6, rgba(0.4, 0.25, 0.1, 1.0))
				}
			} else if known {
				r, gc, b := tileBaseColor(kt.Tile)
				col := rgba(r*0.2, gc*0.2, b*0.2, 1.0)
				drawRect(screen, px, py, tileSizeF-1, tileSizeF-1, col)
			} else {
				drawRect(screen, px, py, tileSizeF, tileSizeF, color.Black)
			}
		}
	}

	for _, e := range g.entities {
		vx := e.X - camX
		vy := e.Y - camY
		if vx < 0 || vx >= ViewWidth || vy < 0 || vy >= ViewHeight {
			continue
		}
		px := float64(vx * TileSize)
		py := float64(vy * TileSize)

		symbol := "?"
		if e.Symbol != "" {
			symbol = e.Symbol
		} else if e.Type == 0 {
			symbol = "@"
		}

		var col color.Color
		if e.Type == 0 {
			col = color.RGBA{255, 255, 120, 255}
		} else {
			col = color.RGBA{255, 80, 80, 255}
		}

		text.Draw(screen, symbol, basicfont.Face7x13, int(px)+5, int(py)+16, col)

		if e.MaxHP > 0 {
			barW := tileSizeF - 4
			barH := 3.0
			hpRatio := float64(e.HP) / float64(e.MaxHP)
			if hpRatio < 0 {
				hpRatio = 0
			}
			drawRect(screen, px+2, py+tileSizeF-5, barW, barH, rgba(0.3, 0, 0, 1))
			drawRect(screen, px+2, py+tileSizeF-5, barW*hpRatio, barH, rgba(0, 1, 0, 1))
		}
	}

	if g.playerInfo != nil {
		pi := g.playerInfo
		hudX := 8
		hudY := 16
		infoColor := color.RGBA{220, 220, 220, 255}
		text.Draw(screen, fmt.Sprintf("%s  Lv.%d", pi.Name, pi.Level), basicfont.Face7x13, hudX, hudY, infoColor)
		text.Draw(screen, fmt.Sprintf("HP: %d/%d  ATK:%d  DEF:%d", pi.HP, pi.MaxHP, pi.Attack, pi.Defense), basicfont.Face7x13, hudX, hudY+14, infoColor)
		text.Draw(screen, fmt.Sprintf("XP: %d/%d  Turn:%d", pi.XP, pi.XPToNext, g.turn), basicfont.Face7x13, hudX, hudY+28, infoColor)
		text.Draw(screen, fmt.Sprintf("Pos: (%d, %d)", pi.X, pi.Y), basicfont.Face7x13, hudX, hudY+42, infoColor)
	}

	logY := ScreenHeight - 14*len(g.combatLog) - 8
	for i, line := range g.combatLog {
		text.Draw(screen, line, basicfont.Face7x13, 8, logY+i*14, color.RGBA{200, 200, 200, 255})
	}

	if !g.connected {
		text.Draw(screen, "DISCONNECTED - check server", basicfont.Face7x13, ScreenWidth/2-100, ScreenHeight/2, color.RGBA{255, 100, 100, 255})
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return ScreenWidth, ScreenHeight
}

func main() {
	serverAddr := "localhost:8080"
	playerName := "Hero"

	game := NewGame(serverAddr, playerName)
	if err := game.Connect(); err != nil {
		log.Printf("Warning: could not connect: %v", err)
	}

	ebiten.SetWindowSize(ScreenWidth*2, ScreenHeight*2)
	ebiten.SetWindowTitle("Roguelike Client")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
