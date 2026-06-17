package server

import (
	"encoding/json"
	"math"
	"math/rand"
	"sync"
	"time"
)

type Floor struct {
	Level    int
	Dungeon  *Dungeon
	Flow     *FlowField
	Players  map[int]*Player
	Monsters map[int]*Monster
	Items    map[int]*Item
	rand     *rand.Rand
	mu       sync.RWMutex
}

func NewFloor(level int, seed int64) *Floor {
	dungeon := GenerateDungeon(60, 60, seed+int64(level)*1000)
	flow := NewFlowField(dungeon)
	return &Floor{
		Level:    level,
		Dungeon:  dungeon,
		Flow:     flow,
		Players:  make(map[int]*Player),
		Monsters: make(map[int]*Monster),
		Items:    make(map[int]*Item),
		rand:     rand.New(rand.NewSource(seed + int64(level)*1000 + 1)),
	}
}

func (f *Floor) SpawnMonsters(nextEntityID *int) {
	if len(f.Dungeon.Rooms) < 2 {
		return
	}
	for i, r := range f.Dungeon.Rooms {
		if i == 0 {
			continue
		}
		numMonsters := 1 + f.rand.Intn(4)
		for j := 0; j < numMonsters; j++ {
			x := r.X + 1 + f.rand.Intn(r.W-2)
			y := r.Y + 1 + f.rand.Intn(r.H-2)
			if f.Dungeon.Tiles[y][x] != TileFloor {
				continue
			}

			tpl := "rat"
			r2 := f.rand.Intn(100)
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

			m := NewMonster(*nextEntityID, tpl, x, y, tpl)
			*nextEntityID++
			f.Monsters[m.ID] = m
		}
	}
}

func (f *Floor) isTileWalkable(x, y int) bool {
	if x < 0 || x >= f.Dungeon.Width || y < 0 || y >= f.Dungeon.Height {
		return false
	}
	t := f.Dungeon.Tiles[y][x]
	return t == TileFloor || t == TileCorridor || t == TileDoor || t == TileTrap || t == TileStairsUp || t == TileStairsDown
}

func (f *Floor) isTileBlocked(x, y int, ignorePlayerID int) bool {
	if !f.isTileWalkable(x, y) {
		return true
	}
	for id, p := range f.Players {
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
	for _, m := range f.Monsters {
		if !m.Alive {
			continue
		}
		if m.X == x && m.Y == y {
			return true
		}
	}
	return false
}

func (f *Floor) findMonsterAt(x, y int) *Monster {
	for _, m := range f.Monsters {
		if m.Alive && m.X == x && m.Y == y {
			return m
		}
	}
	return nil
}

func (f *Floor) updateFlowField() {
	f.Flow.Compute(f.Players)
}

type Game struct {
	Floors       map[int]*Floor
	PlayerFloors map[int]int
	clients      map[int]*Client
	clientMu     sync.RWMutex
	nextEntityID int
	turn         int
	turnDelay    time.Duration
	rand         *rand.Rand
	KnownTiles   map[int]map[[2]int]bool
	mu           sync.RWMutex
	baseSeed     int64
}

func NewGame(seed int64) *Game {
	g := &Game{
		Floors:       make(map[int]*Floor),
		PlayerFloors: make(map[int]int),
		clients:      make(map[int]*Client),
		nextEntityID: 1000,
		turnDelay:    300 * time.Millisecond,
		rand:         rand.New(rand.NewSource(seed + 1)),
		KnownTiles:   make(map[int]map[[2]int]bool),
		baseSeed:     seed,
	}

	f1 := NewFloor(1, seed)
	g.Floors[1] = f1
	return g
}

func (g *Game) SpawnMonsters() {
	for _, f := range g.Floors {
		f.SpawnMonsters(&g.nextEntityID)
	}
}

func (g *Game) getFloor(level int) *Floor {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Floors[level]
}

func (g *Game) ensureFloor(level int) *Floor {
	g.mu.Lock()
	defer g.mu.Unlock()
	f, ok := g.Floors[level]
	if !ok {
		f = NewFloor(level, g.baseSeed+int64(level)*1000)
		f.SpawnMonsters(&g.nextEntityID)
		g.Floors[level] = f
	}
	return f
}

func (g *Game) playerFloor(playerID int) *Floor {
	g.mu.RLock()
	level, ok := g.PlayerFloors[playerID]
	g.mu.RUnlock()
	if !ok {
		return nil
	}
	return g.getFloor(level)
}

func (g *Game) SetHub(hub *Hub) {
	hub.SetListener(g)
}

func (g *Game) OnClientConnected(c *Client) {
	g.clientMu.Lock()
	defer g.clientMu.Unlock()
	g.clients[c.ID] = c
}

func (g *Game) OnClientDisconnected(c *Client) {
	g.clientMu.Lock()
	delete(g.clients, c.ID)
	g.clientMu.Unlock()
	g.RemovePlayer(c.ID)
}

func (g *Game) getClientList() []*Client {
	g.clientMu.RLock()
	defer g.clientMu.RUnlock()
	list := make([]*Client, 0, len(g.clients))
	for _, c := range g.clients {
		list = append(list, c)
	}
	return list
}

func (g *Game) AddPlayer(clientID int, name string) *Player {
	f := g.getFloor(1)
	if f == nil {
		f = g.ensureFloor(1)
	}
	f.mu.Lock()

	var sx, sy int
	if len(f.Dungeon.Rooms) > 0 {
		r := f.Dungeon.Rooms[0]
		sx = r.X + r.W/2
		sy = r.Y + r.H/2
	} else {
		sx = f.Dungeon.Width / 2
		sy = f.Dungeon.Height / 2
	}

	player := NewPlayer(clientID, name, sx, sy)
	player.FOVDirty = true
	f.Players[clientID] = player
	f.mu.Unlock()

	g.mu.Lock()
	g.PlayerFloors[clientID] = 1
	g.KnownTiles[clientID] = make(map[[2]int]bool)
	g.mu.Unlock()

	return player
}

func (g *Game) RemovePlayer(clientID int) {
	g.mu.Lock()
	level, ok := g.PlayerFloors[clientID]
	delete(g.PlayerFloors, clientID)
	delete(g.KnownTiles, clientID)
	g.mu.Unlock()

	if ok {
		f := g.getFloor(level)
		if f != nil {
			f.mu.Lock()
			delete(f.Players, clientID)
			f.mu.Unlock()
		}
	}
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
		clients := g.getClientList()
		for _, client := range clients {
			client.Send(ServerMessage{Type: MsgChat, Data: chat})
		}
	}
}

func (g *Game) handlePlayerMove(c *Client, move PlayerMoveMessage) {
	p := c.player
	if p == nil || !p.Alive {
		return
	}

	f := g.playerFloor(p.ID)
	if f == nil {
		return
	}

	f.mu.Lock()
	g.mu.Lock()

	nx := p.X + move.DX
	ny := p.Y + move.DY

	target := f.findMonsterAt(nx, ny)
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

		g.mu.Unlock()
		f.mu.Unlock()

		clients := g.getClientList()
		for _, client := range clients {
			client.Send(ServerMessage{Type: MsgCombat, Data: combatMsg})
		}

		g.sendPlayerInfo(c)

		g.processTurn(f)
		return
	}

	if !f.isTileBlocked(nx, ny, p.ID) {
		dx := nx - p.X
		dy := ny - p.Y
		p.X = nx
		p.Y = ny
		if p.CachedFOV != nil {
			if absInt(dx)+absInt(dy) > 2 {
				p.FOVDirty = true
			}
		} else {
			p.FOVDirty = true
		}
	}

	g.mu.Unlock()
	f.mu.Unlock()

	g.sendPlayerInfo(c)
	g.sendMapData(c)
	g.processTurn(f)
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (g *Game) processTurn(f *Floor) {
	f.mu.Lock()
	g.mu.Lock()
	g.turn++

	f.updateFlowField()

	for _, m := range f.Monsters {
		UpdateMonsterAIWithFlow(m, f.Dungeon, f.Players, f.Flow)
	}

	combatResults := ProcessMonsterAttacks(f.Monsters, f.Players)

	turn := g.turn

	g.mu.Unlock()
	f.mu.Unlock()

	clients := g.getClientList()

	for _, r := range combatResults {
		msg := ServerMessage{Type: MsgCombat, Data: CombatMessage{
			Hit:          r.Hit,
			Damage:       r.Damage,
			DefenderHP:   r.DefenderHP,
			DefenderDead: r.DefenderDead,
			Crit:         r.Crit,
			Roll:         r.Roll,
		}}
		for _, client := range clients {
			client.Send(msg)
		}
	}

	turnMsg := ServerMessage{Type: MsgTurnUpdate, Data: TurnUpdateMessage{Turn: turn}}
	for _, client := range clients {
		client.Send(turnMsg)
	}

	g.broadcastEntities()
	g.broadcastMapsToAll()
}

func (g *Game) broadcastMapsToAll() {
	clients := g.getClientList()
	for _, c := range clients {
		g.sendMapData(c)
	}
}

func (g *Game) computePlayerFOV(p *Player, f *Floor) {
	if !p.FOVDirty && p.CachedFOV != nil && p.LastFOVX == p.X && p.LastFOVY == p.Y {
		return
	}

	fov := ComputeFOV(f.Dungeon, p.X, p.Y, p.Visibility)
	visible := ComputeVisibleTilesCached(f.Dungeon, p.X, p.Y, p.Visibility, p.PlayerLight, fov)

	p.CachedFOV = fov
	p.CachedTiles = visible
	p.LastFOVX = p.X
	p.LastFOVY = p.Y
	p.FOVDirty = false
}

func (g *Game) sendMapData(c *Client) {
	p := c.player
	if p == nil {
		return
	}

	f := g.playerFloor(p.ID)
	if f == nil {
		return
	}

	f.mu.RLock()
	g.mu.RLock()
	g.computePlayerFOV(p, f)
	visible := p.CachedTiles
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
	g.mu.RUnlock()
	f.mu.RUnlock()

	mapMsg := MapDataMessage{
		Tiles:  tiles,
		Width:  f.Dungeon.Width,
		Height: f.Dungeon.Height,
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
	clients := g.getClientList()

	for _, c := range clients {
		p := c.player
		if p == nil {
			continue
		}

		f := g.playerFloor(p.ID)
		if f == nil {
			continue
		}

		f.mu.RLock()
		g.mu.RLock()

		g.computePlayerFOV(p, f)
		fov := p.CachedFOV
		var entities []EntityInfo

		for id, op := range f.Players {
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

		for id, m := range f.Monsters {
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

		g.mu.RUnlock()
		f.mu.RUnlock()

		c.Send(ServerMessage{Type: MsgEntities, Data: EntitiesMessage{Entities: entities}})
	}
}

func (g *Game) SendWelcome(c *Client, name string) {
	player := g.AddPlayer(c.ID, name)
	c.player = player

	f := g.playerFloor(c.ID)
	if f == nil {
		f = g.ensureFloor(1)
	}

	hello := HelloMessage{
		PlayerID:   c.ID,
		PlayerName: name,
		Width:      f.Dungeon.Width,
		Height:     f.Dungeon.Height,
	}
	c.Send(ServerMessage{Type: MsgHello, Data: hello})

	g.sendPlayerInfo(c)
	g.sendMapData(c)
	g.broadcastEntities()
}

func ComputeVisibleTilesCached(dungeon *Dungeon, px, py, radius int, playerLight float64, fov map[[2]int]bool) []VisibleTile {
	result := []VisibleTile{}
	radiusF := float64(radius)

	for coord := range fov {
		x, y := coord[0], coord[1]

		dx := float64(x - px)
		dy := float64(y - py)
		dist := math.Sqrt(dx*dx + dy*dy)

		distanceFalloff := 1.0 - (dist/radiusF)*0.5

		lightLevel := dungeon.LightMap[y][x]
		brightness := lightLevel * distanceFalloff * playerLight

		if brightness > 0.05 {
			result = append(result, VisibleTile{
				X:          x,
				Y:          y,
				Brightness: brightness,
				Tile:       dungeon.Tiles[y][x],
			})
		}
	}

	return result
}
