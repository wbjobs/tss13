package server

import (
	"math"
	"math/rand"
)

type Tile int

const (
	TileWall      Tile = 0
	TileFloor     Tile = 1
	TileCorridor  Tile = 2
	TileDoor      Tile = 3
	TileTrap      Tile = 4
	TileStairsDown Tile = 5
	TileStairsUp   Tile = 6
)

type Room struct {
	X, Y, W, H int
}

type Dungeon struct {
	Tiles    [][]Tile
	LightMap [][]float64
	Width    int
	Height   int
	Rooms    []Room
	Seed     int64
}

type VisibleTile struct {
	X          int
	Y          int
	Brightness float64
	Tile       Tile
}

func tileAllowed(a, b Tile) bool {
	if a == TileWall || b == TileWall {
		return true
	}
	if a == TileTrap && b == TileWall {
		return true
	}
	if b == TileTrap && a == TileWall {
		return true
	}
	switch a {
	case TileFloor:
		return b == TileFloor || b == TileCorridor || b == TileDoor || b == TileTrap || b == TileStairsUp || b == TileStairsDown
	case TileCorridor:
		return b == TileFloor || b == TileCorridor || b == TileDoor || b == TileTrap
	case TileDoor:
		return true
	case TileTrap:
		return b == TileFloor || b == TileCorridor || b == TileDoor || b == TileTrap
	case TileStairsUp, TileStairsDown:
		return b == TileFloor || b == TileCorridor || b == TileDoor
	}
	return true
}

func entropy(counts map[Tile]int) float64 {
	total := 0
	for _, c := range counts {
		total += c
	}
	if total == 0 {
		return 0
	}
	e := 0.0
	for _, c := range counts {
		if c == 0 {
			continue
		}
		p := float64(c) / float64(total)
		e -= p * math.Log2(p)
	}
	return e
}

func GenerateDungeon(width, height int, seed int64) *Dungeon {
	rng := rand.New(rand.NewSource(seed))

	if width < 50 {
		width = 50
	}
	if height < 50 {
		height = 50
	}

	tiles := make([][]Tile, height)
	for y := 0; y < height; y++ {
		tiles[y] = make([]Tile, width)
		for x := 0; x < width; x++ {
			tiles[y][x] = TileWall
		}
	}

	lightMap := make([][]float64, height)
	for y := 0; y < height; y++ {
		lightMap[y] = make([]float64, width)
		for x := 0; x < width; x++ {
			lightMap[y][x] = 0.0
		}
	}

	dungeon := &Dungeon{
		Tiles:    tiles,
		LightMap: lightMap,
		Width:    width,
		Height:   height,
		Seed:     seed,
	}

	dungeon.carveRooms(rng)
	dungeon.connectRooms(rng)
	dungeon.placeDoors(rng)
	dungeon.placeTraps(rng)
	dungeon.placeStairs(rng)
	dungeon.generateLightMap(rng)

	return dungeon
}

func (d *Dungeon) carveRooms(rng *rand.Rand) {
	numRooms := 10 + rng.Intn(6)
	rooms := make([]Room, 0, numRooms)
	attempts := 0
	maxAttempts := numRooms * 20

	for len(rooms) < numRooms && attempts < maxAttempts {
		attempts++
		w := 6 + rng.Intn(7)
		h := 6 + rng.Intn(7)
		x := 1 + rng.Intn(d.Width-w-2)
		y := 1 + rng.Intn(d.Height-h-2)

		newRoom := Room{X: x, Y: y, W: w, H: h}
		overlap := false
		for _, r := range rooms {
			if x < r.X+r.W+2 && x+w+2 > r.X && y < r.Y+r.H+2 && y+h+2 > r.Y {
				overlap = true
				break
			}
		}
		if overlap {
			continue
		}

		for ry := y; ry < y+h; ry++ {
			for rx := x; rx < x+w; rx++ {
				d.Tiles[ry][rx] = TileFloor
			}
		}
		rooms = append(rooms, newRoom)
	}
	d.Rooms = rooms
}

func (d *Dungeon) connectRooms(rng *rand.Rand) {
	for i := 1; i < len(d.Rooms); i++ {
		a := d.Rooms[i-1]
		b := d.Rooms[i]
		ax := a.X + a.W/2
		ay := a.Y + a.H/2
		bx := b.X + b.W/2
		by := b.Y + b.H/2

		if rng.Intn(2) == 0 {
			d.hCorridor(ax, bx, ay)
			d.vCorridor(ay, by, bx)
		} else {
			d.vCorridor(ay, by, ax)
			d.hCorridor(ax, bx, by)
		}
	}

	if len(d.Rooms) > 4 {
		extraConns := len(d.Rooms) / 3
		for i := 0; i < extraConns; i++ {
			ai := rng.Intn(len(d.Rooms))
			bi := rng.Intn(len(d.Rooms))
			if ai == bi {
				continue
			}
			a := d.Rooms[ai]
			b := d.Rooms[bi]
			ax := a.X + a.W/2
			ay := a.Y + a.H/2
			bx := b.X + b.W/2
			by := b.Y + b.H/2
			if rng.Intn(2) == 0 {
				d.hCorridor(ax, bx, ay)
				d.vCorridor(ay, by, bx)
			} else {
				d.vCorridor(ay, by, ax)
				d.hCorridor(ax, bx, by)
			}
		}
	}
}

func (d *Dungeon) hCorridor(x1, x2, y int) {
	minX := x1
	maxX := x2
	if x1 > x2 {
		minX, maxX = x2, x1
	}
	for x := minX; x <= maxX; x++ {
		if y > 0 && y < d.Height-1 && x > 0 && x < d.Width-1 {
			if d.Tiles[y][x] == TileWall {
				d.Tiles[y][x] = TileCorridor
			}
		}
	}
}

func (d *Dungeon) vCorridor(y1, y2, x int) {
	minY := y1
	maxY := y2
	if y1 > y2 {
		minY, maxY = y2, y1
	}
	for y := minY; y <= maxY; y++ {
		if y > 0 && y < d.Height-1 && x > 0 && x < d.Width-1 {
			if d.Tiles[y][x] == TileWall {
				d.Tiles[y][x] = TileCorridor
			}
		}
	}
}

func (d *Dungeon) isInRoom(x, y int) bool {
	for _, r := range d.Rooms {
		if x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H {
			return true
		}
	}
	return false
}

func (d *Dungeon) placeDoors(rng *rand.Rand) {
	for y := 1; y < d.Height-1; y++ {
		for x := 1; x < d.Width-1; x++ {
			if d.Tiles[y][x] != TileCorridor {
				continue
			}
			adjFloor := 0
			adjCorridor := 0
			if d.Tiles[y-1][x] == TileFloor {
				adjFloor++
			}
			if d.Tiles[y+1][x] == TileFloor {
				adjFloor++
			}
			if d.Tiles[y][x-1] == TileFloor {
				adjFloor++
			}
			if d.Tiles[y][x+1] == TileFloor {
				adjFloor++
			}
			if d.Tiles[y-1][x] == TileCorridor {
				adjCorridor++
			}
			if d.Tiles[y+1][x] == TileCorridor {
				adjCorridor++
			}
			if d.Tiles[y][x-1] == TileCorridor {
				adjCorridor++
			}
			if d.Tiles[y][x+1] == TileCorridor {
				adjCorridor++
			}
			if adjFloor >= 1 && adjFloor <= 2 && adjCorridor <= 2 {
				if rng.Float64() < 0.5 {
					d.Tiles[y][x] = TileDoor
				}
			}
		}
	}
}

func (d *Dungeon) placeTraps(rng *rand.Rand) {
	numTraps := 5 + rng.Intn(6)
	placed := 0
	attempts := 0
	maxAttempts := numTraps * 50

	for placed < numTraps && attempts < maxAttempts {
		attempts++
		x := 1 + rng.Intn(d.Width-2)
		y := 1 + rng.Intn(d.Height-2)
		if d.Tiles[y][x] != TileFloor {
			continue
		}
		d.Tiles[y][x] = TileTrap
		placed++
	}
}

func (d *Dungeon) placeStairs(rng *rand.Rand) {
	if len(d.Rooms) < 2 {
		return
	}
	first := d.Rooms[0]
	last := d.Rooms[len(d.Rooms)-1]

	ux := first.X + first.W/2
	uy := first.Y + first.H/2
	if d.Tiles[uy][ux] == TileFloor {
		d.Tiles[uy][ux] = TileStairsUp
	}

	dx := last.X + last.W/2
	dy := last.Y + last.H/2
	if d.Tiles[dy][dx] == TileFloor {
		d.Tiles[dy][dx] = TileStairsDown
	}
}

func (d *Dungeon) generateLightMap(rng *rand.Rand) {
	for y := 0; y < d.Height; y++ {
		for x := 0; x < d.Width; x++ {
			switch d.Tiles[y][x] {
			case TileWall:
				d.LightMap[y][x] = 0.15
			case TileCorridor:
				d.LightMap[y][x] = 0.3
			case TileFloor:
				d.LightMap[y][x] = 0.6
			case TileDoor:
				d.LightMap[y][x] = 0.5
			case TileTrap:
				d.LightMap[y][x] = 0.4
			case TileStairsUp, TileStairsDown:
				d.LightMap[y][x] = 0.8
			}
		}
	}

	numLights := 3 + rng.Intn(4)
	lightRooms := make(map[int]bool)

	for i := 0; i < numLights; i++ {
		if len(d.Rooms) == 0 {
			break
		}
		ri := rng.Intn(len(d.Rooms))
		if lightRooms[ri] {
			continue
		}
		lightRooms[ri] = true
		r := d.Rooms[ri]
		lx := r.X + r.W/2
		ly := r.Y + r.H/2
		radius := 6.0

		for dy := -int(math.Ceil(radius)); dy <= int(math.Ceil(radius)); dy++ {
			for dx := -int(math.Ceil(radius)); dx <= int(math.Ceil(radius)); dx++ {
				px := lx + dx
				py := ly + dy
				if px < 0 || px >= d.Width || py < 0 || py >= d.Height {
					continue
				}
				dist := math.Sqrt(float64(dx*dx + dy*dy))
				if dist > radius {
					continue
				}
				falloff := 1.0 - (dist / radius)
				brightness := falloff * 1.0
				if brightness > d.LightMap[py][px] {
					d.LightMap[py][px] = brightness
				}
			}
		}
	}

	for y := 0; y < d.Height; y++ {
		for x := 0; x < d.Width; x++ {
			if d.LightMap[y][x] > 1.0 {
				d.LightMap[y][x] = 1.0
			}
			if d.LightMap[y][x] < 0.0 {
				d.LightMap[y][x] = 0.0
			}
		}
	}
}
