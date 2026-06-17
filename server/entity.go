package server

type EntityType int

const (
	EntityPlayer  EntityType = 0
	EntityMonster EntityType = 1
	EntityItem    EntityType = 2
)

type Entity struct {
	ID      int
	Type    EntityType
	X       int
	Y       int
	HP      int
	MaxHP   int
	Attack  int
	Defense int
	Name    string
	Alive   bool
	Symbol  rune
}

type Player struct {
	Entity
	Level       int
	XP          int
	XPToNext    int
	Visibility  int
	PlayerLight float64
	LastFOVX    int
	LastFOVY    int
	CachedFOV   map[[2]int]bool
	CachedTiles []VisibleTile
	FOVDirty    bool
}

type Monster struct {
	Entity
	SightRange int
	AIState    string
	TargetID   int
	XPValue    int
}

type Item struct {
	Entity
	ItemType string
	Value    int
}

func NewPlayer(id int, name string, x, y int) *Player {
	return &Player{
		Entity: Entity{
			ID:      id,
			Type:    EntityPlayer,
			X:       x,
			Y:       y,
			HP:      30,
			MaxHP:   30,
			Attack:  5,
			Defense: 2,
			Name:    name,
			Alive:   true,
			Symbol:  '@',
		},
		Level:       1,
		XP:          0,
		XPToNext:    100,
		Visibility:  10,
		PlayerLight: 0.9,
	}
}

func NewMonster(id int, name string, x, y int, template string) *Monster {
	m := &Monster{
		Entity: Entity{
			ID:     id,
			Type:   EntityMonster,
			X:      x,
			Y:      y,
			Name:   name,
			Alive:  true,
			Symbol: 'r',
		},
		AIState: "idle",
	}

	switch template {
	case "rat":
		m.HP = 5
		m.MaxHP = 5
		m.Attack = 2
		m.Defense = 0
		m.SightRange = 5
		m.XPValue = 10
		m.Symbol = 'r'
	case "goblin":
		m.HP = 12
		m.MaxHP = 12
		m.Attack = 4
		m.Defense = 1
		m.SightRange = 7
		m.XPValue = 25
		m.Symbol = 'g'
	case "skeleton":
		m.HP = 18
		m.MaxHP = 18
		m.Attack = 6
		m.Defense = 2
		m.SightRange = 8
		m.XPValue = 40
		m.Symbol = 's'
	case "orc":
		m.HP = 30
		m.MaxHP = 30
		m.Attack = 8
		m.Defense = 3
		m.SightRange = 6
		m.XPValue = 60
		m.Symbol = 'o'
	case "dragon":
		m.HP = 80
		m.MaxHP = 80
		m.Attack = 15
		m.Defense = 6
		m.SightRange = 10
		m.XPValue = 200
		m.Symbol = 'D'
	}

	return m
}

func NewItem(id int, name string, x, y int, itemType string, value int) *Item {
	var symbol rune
	switch itemType {
	case "health_potion":
		symbol = '!'
	case "weapon":
		symbol = '/'
	case "armor":
		symbol = ']'
	}

	return &Item{
		Entity: Entity{
			ID:     id,
			Type:   EntityItem,
			X:      x,
			Y:      y,
			Name:   name,
			Alive:  true,
			Symbol: symbol,
		},
		ItemType: itemType,
		Value:    value,
	}
}
