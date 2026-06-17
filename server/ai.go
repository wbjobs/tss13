package server

import (
	"container/heap"
	"math"
	"math/rand"
)

const maxPathSteps = 20

func isWalkable(dungeon *Dungeon, x, y int) bool {
	if x < 0 || x >= dungeon.Width || y < 0 || y >= dungeon.Height {
		return false
	}
	t := dungeon.Tiles[y][x]
	return t == TileFloor || t == TileCorridor || t == TileDoor || t == TileTrap || t == TileStairsDown || t == TileStairsUp
}

func distance(x1, y1, x2, y2 int) float64 {
	dx := float64(x2 - x1)
	dy := float64(y2 - y1)
	return math.Sqrt(dx*dx + dy*dy)
}

func findNearestPlayer(monster *Monster, players map[int]*Player) *Player {
	var nearest *Player
	nearestDist := math.MaxFloat64
	for _, p := range players {
		if !p.Alive {
			continue
		}
		d := distance(monster.X, monster.Y, p.X, p.Y)
		if d < nearestDist {
			nearestDist = d
			nearest = p
		}
	}
	return nearest
}

func findVisiblePlayer(monster *Monster, players map[int]*Player) *Player {
	var best *Player
	bestDist := math.MaxFloat64
	for _, p := range players {
		if !p.Alive {
			continue
		}
		d := distance(monster.X, monster.Y, p.X, p.Y)
		if d <= float64(monster.SightRange) && d < bestDist {
			bestDist = d
			best = p
		}
	}
	return best
}

type node struct {
	x, y   int
	g, f   float64
	parent *node
}

type openSet []node

func (o openSet) Len() int            { return len(o) }
func (o openSet) Less(i, j int) bool  { return o[i].f < o[j].f }
func (o openSet) Swap(i, j int)       { o[i], o[j] = o[j], o[i] }
func (o *openSet) Push(x interface{}) { *o = append(*o, x.(node)) }
func (o *openSet) Pop() interface{} {
	old := *o
	n := len(old)
	item := old[n-1]
	*o = old[:n-1]
	return item
}

func AStar(dungeon *Dungeon, sx, sy, tx, ty int) ([][2]int, bool) {
	if !isWalkable(dungeon, tx, ty) {
		return nil, false
	}

	closed := make(map[[2]int]bool)
	gScore := make(map[[2]int]float64)
	startKey := [2]int{sx, sy}
	gScore[startKey] = 0

	start := node{
		x:      sx,
		y:      sy,
		g:      0,
		f:      math.Abs(float64(tx-sx)) + math.Abs(float64(ty-sy)),
		parent: nil,
	}

	o := &openSet{start}
	heap.Init(o)

	dirs := [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}

	var endNode *node

	for o.Len() > 0 {
		current := heap.Pop(o).(node)
		currentKey := [2]int{current.x, current.y}

		if current.x == tx && current.y == ty {
			endNode = &current
			break
		}

		if closed[currentKey] {
			continue
		}
		closed[currentKey] = true

		for _, d := range dirs {
			nx, ny := current.x+d[0], current.y+d[1]
			nk := [2]int{nx, ny}

			if !isWalkable(dungeon, nx, ny) || closed[nk] {
				continue
			}

			ng := current.g + 1
			if prev, ok := gScore[nk]; ok && ng >= prev {
				continue
			}
			gScore[nk] = ng

			h := math.Abs(float64(tx-nx)) + math.Abs(float64(ty-ny))
			n := node{
				x:      nx,
				y:      ny,
				g:      ng,
				f:      ng + h,
				parent: &current,
			}
			heap.Push(o, n)
		}
	}

	if endNode == nil {
		return nil, false
	}

	var path [][2]int
	cur := endNode
	for cur != nil {
		path = append(path, [2]int{cur.x, cur.y})
		cur = cur.parent
	}

	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	if len(path) > maxPathSteps {
		path = path[:maxPathSteps]
	}

	return path, true
}

func UpdateMonsterAI(monster *Monster, dungeon *Dungeon, players map[int]*Player) {
	if !monster.Alive {
		return
	}

	target := findVisiblePlayer(monster, players)
	if target == nil {
		monster.AIState = "idle"
		monster.TargetID = 0
		if rand.Float64() < 0.2 {
			dirs := [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}
			d := dirs[rand.Intn(len(dirs))]
			nx, ny := monster.X+d[0], monster.Y+d[1]
			if isWalkable(dungeon, nx, ny) {
				monster.X = nx
				monster.Y = ny
			}
		}
		return
	}

	monster.TargetID = target.ID
	d := distance(monster.X, monster.Y, target.X, target.Y)
	if d <= 1.5 {
		monster.AIState = "attack"
		return
	}

	monster.AIState = "chase"
	path, ok := AStar(dungeon, monster.X, monster.Y, target.X, target.Y)
	if !ok || len(path) < 2 {
		return
	}
	next := path[1]
	if isWalkable(dungeon, next[0], next[1]) {
		monster.X = next[0]
		monster.Y = next[1]
	}
}

func UpdateMonsterAIWithFlow(monster *Monster, dungeon *Dungeon, players map[int]*Player, flow *FlowField) {
	if !monster.Alive {
		return
	}

	target := findVisiblePlayer(monster, players)
	if target == nil {
		monster.AIState = "idle"
		monster.TargetID = 0
		if rand.Float64() < 0.2 {
			dirs := [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}
			d := dirs[rand.Intn(len(dirs))]
			nx, ny := monster.X+d[0], monster.Y+d[1]
			if isWalkable(dungeon, nx, ny) {
				monster.X = nx
				monster.Y = ny
			}
		}
		return
	}

	monster.TargetID = target.ID
	d := distance(monster.X, monster.Y, target.X, target.Y)
	if d <= 1.5 {
		monster.AIState = "attack"
		return
	}

	monster.AIState = "chase"

	dx, dy, ok := flow.GetDirection(monster.X, monster.Y)
	if !ok {
		return
	}

	ndx, ndy := 0, 0
	if dx < 0 {
		ndx = -1
	} else if dx > 0 {
		ndx = 1
	}
	if dy < 0 {
		ndy = -1
	} else if dy > 0 {
		ndy = 1
	}

	if ndx == 0 && ndy == 0 {
		return
	}

	nx, ny := monster.X+ndx, monster.Y+ndy
	if isWalkable(dungeon, nx, ny) {
		monster.X = nx
		monster.Y = ny
	}
}
