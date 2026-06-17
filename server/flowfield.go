package server

import (
	"math"
)

type FlowField struct {
	Dungeon   *Dungeon
	Width     int
	Height    int
	distance  [][]float64
	direction [][][2]int
	valid     bool
}

func NewFlowField(dungeon *Dungeon) *FlowField {
	w := dungeon.Width
	h := dungeon.Height
	ff := &FlowField{
		Dungeon:   dungeon,
		Width:     w,
		Height:    h,
		distance:  make([][]float64, h),
		direction: make([][][2]int, h),
	}
	for y := 0; y < h; y++ {
		ff.distance[y] = make([]float64, w)
		ff.direction[y] = make([][2]int, w)
		for x := 0; x < w; x++ {
			ff.distance[y][x] = math.Inf(1)
			ff.direction[y][x] = [2]int{0, 0}
		}
	}
	return ff
}

func (ff *FlowField) Compute(players map[int]*Player) {
	w := ff.Width
	h := ff.Height

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ff.distance[y][x] = math.Inf(1)
			ff.direction[y][x] = [2]int{0, 0}
		}
	}

	queue := make([][2]int, 0, w*h)

	for _, p := range players {
		if !p.Alive {
			continue
		}
		if p.X >= 0 && p.X < w && p.Y >= 0 && p.Y < h {
			if ff.isWalkable(p.X, p.Y) {
				ff.distance[p.Y][p.X] = 0
				queue = append(queue, [2]int{p.X, p.Y})
			}
		}
	}

	dirs := [][2]int{
		{0, -1}, {0, 1}, {-1, 0}, {1, 0},
		{-1, -1}, {1, -1}, {-1, 1}, {1, 1},
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		cx, cy := cur[0], cur[1]
		cd := ff.distance[cy][cx]

		for _, d := range dirs {
			nx, ny := cx+d[0], cy+d[1]
			if nx < 0 || nx >= w || ny < 0 || ny >= h {
				continue
			}
			if !ff.isWalkable(nx, ny) {
				continue
			}

			moveCost := 1.0
			if d[0] != 0 && d[1] != 0 {
				moveCost = 1.414
			}

			nd := cd + moveCost
			if nd < ff.distance[ny][nx] {
				ff.distance[ny][nx] = nd
				queue = append(queue, [2]int{nx, ny})
			}
		}
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if math.IsInf(ff.distance[y][x], 1) {
				continue
			}
			bestDist := ff.distance[y][x]
			bestDir := [2]int{0, 0}
			hasBetter := false

			for _, d := range dirs {
				nx, ny := x+d[0], y+d[1]
				if nx < 0 || nx >= w || ny < 0 || ny >= h {
					continue
				}
				if math.IsInf(ff.distance[ny][nx], 1) {
					continue
				}
				if ff.distance[ny][nx] < bestDist {
					bestDist = ff.distance[ny][nx]
					bestDir = d
					hasBetter = true
				}
			}

			if hasBetter {
				ff.direction[y][x] = bestDir
			}
		}
	}

	ff.valid = true
}

func (ff *FlowField) isWalkable(x, y int) bool {
	if x < 0 || x >= ff.Width || y < 0 || y >= ff.Height {
		return false
	}
	t := ff.Dungeon.Tiles[y][x]
	return t == TileFloor || t == TileCorridor || t == TileDoor || t == TileTrap || t == TileStairsDown || t == TileStairsUp
}

func (ff *FlowField) GetDirection(x, y int) (int, int, bool) {
	if !ff.valid || x < 0 || x >= ff.Width || y < 0 || y >= ff.Height {
		return 0, 0, false
	}
	d := ff.direction[y][x]
	if d[0] == 0 && d[1] == 0 {
		return 0, 0, false
	}
	return d[0], d[1], true
}

func (ff *FlowField) GetDistance(x, y int) (float64, bool) {
	if !ff.valid || x < 0 || x >= ff.Width || y < 0 || y >= ff.Height {
		return 0, false
	}
	d := ff.distance[y][x]
	if math.IsInf(d, 1) {
		return 0, false
	}
	return d, true
}
