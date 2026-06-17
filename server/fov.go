package server

import "math"

func ComputeFOV(dungeon *Dungeon, cx, cy, radius int) map[[2]int]bool {
	visible := map[[2]int]bool{}
	visible[[2]int{cx, cy}] = true

	for octant := 0; octant < 8; octant++ {
		castLight(dungeon, cx, cy, 1, 1.0, 0.0, radius, octant, visible)
	}

	return visible
}

func castLight(dungeon *Dungeon, cx, cy, row int, startSlope, endSlope float64, radius, octant int, visible map[[2]int]bool) {
	if startSlope < endSlope {
		return
	}

	nextStartSlope := startSlope
	blocked := false

	for dist := row; dist <= radius && !blocked; dist++ {
		for dx := 0; dx <= dist; dx++ {
			dy := dist - dx

			x, y := octantTransform(octant, cx, cy, dx, dy)

			if x < 0 || x >= dungeon.Width || y < 0 || y >= dungeon.Height {
				continue
			}

			lSlope := (float64(dx) - 0.5) / (float64(dy) + 0.5)
			rSlope := (float64(dx) + 0.5) / (float64(dy) - 0.5)

			if startSlope < rSlope {
				continue
			}

			if endSlope > lSlope {
				break
			}

			dxAbs := dx
			dyAbs := dy
			if dxAbs < 0 {
				dxAbs = -dxAbs
			}
			if dyAbs < 0 {
				dyAbs = -dyAbs
			}
			if dxAbs+dyAbs <= radius {
				visible[[2]int{x, y}] = true
			}

			isWall := dungeon.Tiles[y][x] == TileWall

			if blocked {
				if isWall {
					nextStartSlope = -rSlope
				} else {
					blocked = false
					startSlope = nextStartSlope
				}
			} else if isWall && dist < radius {
				blocked = true
				castLight(dungeon, cx, cy, dist+1, startSlope, -lSlope, radius, octant, visible)
				nextStartSlope = -rSlope
			}
		}
	}
}

func octantTransform(octant, cx, cy, dx, dy int) (int, int) {
	switch octant {
	case 0:
		return cx + dx, cy - dy
	case 1:
		return cx + dy, cy - dx
	case 2:
		return cx - dy, cy - dx
	case 3:
		return cx - dx, cy - dy
	case 4:
		return cx - dx, cy + dy
	case 5:
		return cx - dy, cy + dx
	case 6:
		return cx + dy, cy + dx
	case 7:
		return cx + dx, cy + dy
	default:
		return cx, cy
	}
}

func ComputeVisibleTiles(dungeon *Dungeon, px, py, radius int, playerLight float64) []VisibleTile {
	fov := ComputeFOV(dungeon, px, py, radius)
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
