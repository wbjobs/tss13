package server

import (
	"encoding/json"
	"math/rand"
	"sync"
	"time"
)

type Game struct {
	Dungeon     *Dungeon
	Players     map[int]*Player
	Monsters    map[int]*Monster
	Items       map[int]*Item
	hub         *Hub
	nextEntityID int
	mu          sync.RWMutex
	turn        int
	turnTimer   *time.Timer
	turnDelay   time.Duration
	rand        *rand.Rand
	KnownTiles  map[int]map[[2]int]bool
}

func NewGame(seed int64) *Game {
	dungeon := GenerateDungeon(60, 60, seed)
	return &Game{
		Dungeon:      dungeon,
		Players:      make(map[int]*Player),
		Monsters:     make(map[int]*Monster),
		Items:        make(map[int]*Item),
		nextEntityID: 1000,
		turnDelay:    300 * time.Millisecond,
		rand:         rand.New(rand.NewSource(seed + 1)),
		KnownTiles:   make(map[int]map[[2]int]bool),
	}
}

func (g *Game) SetHub(hub *Hub) {
	g.hub = hub
}

func (g *Game) SpawnMonsters() {
	if len(g.Dungeon.Rooms) < 2 {
		return
	}
	for i, r := range g.Dungeon.Rooms {
		if i == 0 {
			continue
		}
		numMonsters := 1 + g.rand.Intn(4)
		for j := 0; j < numMonsters; j++ {
			x := r.X + 1 + g.rand.Intn(r.W-2)
			y := r.Y + 1 + g.rand.Intn(r.H-2)
			if g.Dungeon.Tiles[y][x] != TileFloor {
				continue
			}

			tpl := "rat"
			r2 := g.rand.Intn(100)
			if r2 < 40 {
				tpl = "rat"
			} else if r2 < 70 {
				tpl = "goblin"
			} else if r2 < 88 {
				tpl = "skeleton"
			} else if r2 < 97 {
				tpl = "orc"
			} else {
				tpl = "dragon"
			}

			m := NewMonster(g.nextEntityID, tpl, x, y, tpl)
			g.nextEntityID++
			g.Monsters[m.ID] = m
		}
	}
}

func (g *Game) AddPlayer(clientID int, name string) *Player {
	g.mu.Lock()
	defer g.mu.Unlock()

	var sx, sy int
	if len(g.Dungeon.Rooms) > 0 {
		r := g.Dungeon.Rooms[0]
		sx = r.X + r.W/2
		sy = r.Y + r.H/2
	} else {
		sx = g.Dungeon.Width / 2
		sy = g.Dungeon.Height / 2
	}

	player := NewPlayer(clientID, name, sx, sy)
	g.Players[clientID] = player
	g.KnownTiles[clientID] = make(map[[2]int]bool)
	return player
}

func (g *Game) RemovePlayer(clientID int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.Players, clientID)
	delete(g.KnownTiles, clientID)
}

func (g *Game) isTileWalkable(x, y int) bool {
	if x < 0 || x >= g.Dungeon.Width || y < 0 || y >= g.Dungeon.Height {
		return false
	}
	t := g.Dungeon.Tiles[y][x]
	return t == TileFloor || t == TileCorridor || t == TileDoor || t == TileTrap || t == TileStairsUp || t == TileStairsDown
}

func (g *Game) isTileBlocked(x, y int, ignorePlayerID int) bool {
	if !g.isTileWalkable(x, y) {
		return true
	}
	for id, p := range g.Players {
		if id == ignorePlayerID {
			continue
		}
		if !p.Alive {
			continue
		}
		if p.X == x && p.Y == y {
			return true
		}
	}
	for _, m := range g.Monsters {
		if !m.Alive {
			continue
		}
		if m.X == x && m.Y == y {
			return true
		}
	}
	return false
}

func (g *Game) findMonsterAt(x, y int) *Monster {
	for _, m := range g.Monsters {
		if m.Alive && m.X == x && m.Y == y {
			return m
		}
	}
	return nil
}

func (g *Game) HandleClientMessage(c *Client, msg ClientMessage) {
	switch msg.Type {
	case MsgPlayerMove:
		dataBytes, _ := json.Marshal(msg.Data)
		var move PlayerMoveMessage
		json.Unmarshal(dataBytes, &move)
		g.handlePlayerMove(c, move)
	case MsgChat:
		dataBytes, _ := json.Marshal(msg.Data)
		var chat ChatMessage
		json.Unmarshal(dataBytes, &chat)
		g.hub.broadcast <- ServerMessage{Type: MsgChat, Data: chat}
	}
}

func (g *Game) handlePlayerMove(c *Client, move PlayerMoveMessage) {
	g.mu.Lock()
	defer g.mu.Unlock()

	p := c.player
	if p == nil || !p.Alive {
		return
	}

	nx := p.X + move.DX
	ny := p.Y + move.DY

	target := g.findMonsterAt(nx, ny)
	if target != nil {
		result := PlayerAttackMonster(p, target)
		combatMsg := CombatMessage{
			AttackerID:   p.ID,
			DefenderID:   target.ID,
			AttackerName: p.Name,
			DefenderName: target.Name,
			Hit:          result.Hit,
			Damage:       result.Damage,
			DefenderHP:   result.DefenderHP,
			DefenderDead: result.DefenderDead,
			Crit:         result.Crit,
			Roll:         result.Roll,
		}
		g.hub.broadcast <- ServerMessage{Type: MsgCombat, Data: combatMsg}
		g.sendPlayerInfo(c)
		g.processTurn()
		return
	}

	if !g.isTileBlocked(nx, ny, p.ID) {
		p.X = nx
		p.Y = ny
	}

	g.sendPlayerInfo(c)
	g.sendMapData(c)
	g.processTurn()
}

func (g *Game) processTurn() {
	g.turn++

	for _, m := range g.Monsters {
		UpdateMonsterAI(m, g.Dungeon, g.Players)
	}

	results := ProcessMonsterAttacks(g.Monsters, g.Players)
	for _, r := range results {
		g.hub.broadcast <- ServerMessage{Type: MsgCombat, Data: CombatMessage{
			Hit:          r.Hit,
			Damage:       r.Damage,
			DefenderHP:   r.DefenderHP,
			DefenderDead: r.DefenderDead,
			Crit:         r.Crit,
			Roll:         r.Roll,
		}}
	}

	g.hub.broadcast <- ServerMessage{Type: MsgTurnUpdate, Data: TurnUpdateMessage{Turn: g.turn}}
	g.broadcastEntities()
	g.broadcastMapsToAll()
}

func (g *Game) broadcastMapsToAll() {
	g.hub.mu.RLock()
	defer g.hub.mu.RUnlock()
	for _, c := range g.hub.clients {
		g.sendMapData(c)
	}
}

func (g *Game) sendMapData(c *Client) {
	p := c.player
	if p == nil {
		return
	}

	visible := ComputeVisibleTiles(g.Dungeon, p.X, p.Y, p.Visibility, p.PlayerLight)
	known := g.KnownTiles[c.ID]

	tiles := make([]TileInfo, 0, len(visible))
	for _, vt := range visible {
		tiles = append(tiles, TileInfo{
			X:          vt.X,
			Y:          vt.Y,
			Tile:       int(vt.Tile),
			Brightness: vt.Brightness,
		})
		known[[2]int{vt.X, vt.Y}] = true
	}

	mapMsg := MapDataMessage{
		Tiles:  tiles,
		Width:  g.Dungeon.Width,
		Height: g.Dungeon.Height,
	}
	c.Send(ServerMessage{Type: MsgMapData, Data: mapMsg})
}

func (g *Game) sendPlayerInfo(c *Client) {
	p := c.player
	if p == nil {
		return
	}
	info := PlayerInfoMessage{
		ID:       p.ID,
		HP:       p.HP,
		MaxHP:    p.MaxHP,
		Attack:   p.Attack,
		Defense:  p.Defense,
		Level:    p.Level,
		XP:       p.XP,
		XPToNext: p.XPToNext,
		X:        p.X,
		Y:        p.Y,
		Name:     p.Name,
	}
	c.Send(ServerMessage{Type: MsgPlayerInfo, Data: info})
}

func (g *Game) broadcastEntities() {
	g.hub.mu.RLock()
	defer g.hub.mu.RUnlock()

	for _, c := range g.hub.clients {
		p := c.player
		if p == nil {
			continue
		}
		fov := ComputeFOV(g.Dungeon, p.X, p.Y, p.Visibility)
		var entities []EntityInfo

		for id, op := range g.Players {
			if !op.Alive {
				continue
			}
			if fov[[2]int{op.X, op.Y}] || id == p.ID {
				entities = append(entities, EntityInfo{
					ID:     id,
					Type:   int(EntityPlayer),
					X:      op.X,
					Y:      op.Y,
					HP:     op.HP,
					MaxHP:  op.MaxHP,
					Name:   op.Name,
					Symbol: string(op.Symbol),
					Level:  op.Level,
				})
			}
		}

		for id, m := range g.Monsters {
			if !m.Alive {
				continue
			}
			if fov[[2]int{m.X, m.Y}] {
				entities = append(entities, EntityInfo{
					ID:     id,
					Type:   int(EntityMonster),
					X:      m.X,
					Y:      m.Y,
					HP:     m.HP,
					MaxHP:  m.MaxHP,
					Name:   m.Name,
					Symbol: string(m.Symbol),
				})
			}
		}

		c.Send(ServerMessage{Type: MsgEntities, Data: EntitiesMessage{Entities: entities}})
	}
}

func (g *Game) SendWelcome(c *Client, name string) {
	g.mu.Lock()
	player := g.AddPlayer(c.ID, name)
	c.player = player
	g.mu.Unlock()

	hello := HelloMessage{
		PlayerID:   c.ID,
		PlayerName: name,
		Width:      g.Dungeon.Width,
		Height:     g.Dungeon.Height,
	}
	c.Send(ServerMessage{Type: MsgHello, Data: hello})

	g.sendPlayerInfo(c)
	g.sendMapData(c)
	g.broadcastEntities()
}
