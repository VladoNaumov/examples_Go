package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

// Simple ASCII Dungeon Crawler.
//
// Run: go run dungeon.go
//
// Controls:
//   w/a/s/d - move
//   i       - show inventory
//   p       - pick up item on current tile
//   u <idx> - use item by index
//   q       - quit
//

type TileType int

const (
	WallTile TileType = iota
	FloorTile
)

type Tile struct {
	X, Y   int
	Type   TileType
	Item   *Item
	Entity *Entity
}

type Item struct {
	Name string
	// Consumable: heal amount.
	Heal int
}

type Stats struct {
	HPMax   int
	HP      int
	Attack  int
	Defense int
	Speed   int
}

type Entity struct {
	Name     string
	X, Y     int
	Stats    Stats
	Inv      []Item
	IsPlayer bool
	Alive    bool
	AIType   string // e.g., "basic" for chase AI
}

type World struct {
	Width    int
	Height   int
	Tiles    [][]*Tile
	Player   *Entity
	Entities []*Entity
	Rand     *rand.Rand
}

// NewWorld creates a new game world with random interior walls.
func NewWorld(width, height int, r *rand.Rand) *World {
	tiles := make([][]*Tile, height)
	for y := 0; y < height; y++ {
		row := make([]*Tile, width)
		for x := 0; x < width; x++ {
			row[x] = &Tile{X: x, Y: y, Type: FloorTile}
		}
		tiles[y] = row
	}
	world := &World{
		Width:    width,
		Height:   height,
		Tiles:    tiles,
		Rand:     r,
		Entities: make([]*Entity, 0),
	}
	world.generateWalls()
	return world
}

// generateWalls sets border walls and adds random interior walls.
func (w *World) generateWalls() {
	// Border walls.
	for x := 0; x < w.Width; x++ {
		w.Tiles[0][x].Type = WallTile
		w.Tiles[w.Height-1][x].Type = WallTile
	}
	for y := 0; y < w.Height; y++ {
		w.Tiles[y][0].Type = WallTile
		w.Tiles[y][w.Width-1].Type = WallTile
	}

	// Random interior walls (10% density).
	numWalls := (w.Width * w.Height) / 10
	for i := 0; i < numWalls; i++ {
		x := w.Rand.Intn(w.Width-2) + 1
		y := w.Rand.Intn(w.Height-2) + 1
		w.Tiles[y][x].Type = WallTile
	}
}

// PlaceEntity places an entity at (x, y) if valid, marks it alive, and tracks it.
func (w *World) PlaceEntity(e *Entity, x, y int) {
	if x < 0 || x >= w.Width || y < 0 || y >= w.Height || w.Tiles[y][x].Type == WallTile {
		return // Invalid position.
	}
	e.X = x
	e.Y = y
	e.Alive = true
	w.Tiles[y][x].Entity = e
	w.Entities = append(w.Entities, e)
	if e.IsPlayer {
		w.Player = e
	}
}

// PlaceItem places an item at (x, y) if the tile is empty.
func (w *World) PlaceItem(it Item, x, y int) {
	if x < 0 || x >= w.Width || y < 0 || y >= w.Height ||
		w.Tiles[y][x].Type == WallTile || w.Tiles[y][x].Entity != nil {
		return // Invalid or occupied.
	}
	w.Tiles[y][x].Item = &it
}

// MoveEntity attempts to move an entity to (nx, ny). Returns true if successful.
func (w *World) MoveEntity(e *Entity, nx, ny int) bool {
	if nx < 0 || nx >= w.Width || ny < 0 || ny >= w.Height {
		return false
	}
	dest := w.Tiles[ny][nx]
	if dest.Type == WallTile {
		return false
	}
	if dest.Entity != nil {
		// Attack if hostile.
		if e.IsPlayer != dest.Entity.IsPlayer {
			w.resolveMelee(e, dest.Entity)
			return true
		}
		return false // Blocked by friendly.
	}
	// Valid move.
	w.Tiles[e.Y][e.X].Entity = nil
	e.X = nx
	e.Y = ny
	dest.Entity = e
	return true
}

// resolveMelee handles a melee attack between attacker and defender.
func (w *World) resolveMelee(attacker, defender *Entity) {
	damage := attacker.Stats.Attack - defender.Stats.Defense/2
	if damage < 1 {
		damage = 1
	}
	fmt.Printf("%s атакует %s на %d урона\n", attacker.Name, defender.Name, damage)
	defender.Stats.HP -= damage
	if defender.Stats.HP <= 0 {
		defender.Stats.HP = 0
		defender.Alive = false
		fmt.Printf("%s убит(а)!\n", defender.Name)
		w.Tiles[defender.Y][defender.X].Entity = nil
	}
}

// RemoveDeadEntities cleans up dead entities from the list.
func (w *World) RemoveDeadEntities() {
	var alive []*Entity
	for _, e := range w.Entities {
		if e.Alive {
			alive = append(alive, e)
		}
	}
	w.Entities = alive
}

// BFSStepTowards computes the next step (dx, dy) from src towards target using BFS.
// Returns (0, 0) if no path.
func (w *World) BFSStepTowards(src, target *Entity) (int, int) {
	type pos struct{ x, y int }

	height, width := w.Height, w.Width
	visited := make([][]bool, height)
	for i := 0; i < height; i++ {
		visited[i] = make([]bool, width)
	}

	prev := make(map[pos]pos)
	queue := []pos{{src.X, src.Y}}
	visited[src.Y][src.X] = true

	found := false
	var dest pos
	deltas := []pos{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

	for len(queue) > 0 && !found {
		curr := queue[0]
		queue = queue[1:]

		for _, delta := range deltas {
			nx, ny := curr.x+delta.x, curr.y+delta.y
			if nx < 0 || nx >= width || ny < 0 || ny >= height || visited[ny][nx] {
				continue
			}

			tile := w.Tiles[ny][nx]
			if tile.Type == WallTile {
				continue
			}

			visited[ny][nx] = true
			prev[pos{nx, ny}] = curr

			if nx == target.X && ny == target.Y {
				found = true
				dest = pos{nx, ny}
				break
			}

			queue = append(queue, pos{nx, ny})
		}
	}

	if !found {
		return 0, 0
	}

	// Backtrack to find the first step from src.
	curr := dest
	for {
		p := prev[curr]
		if p.x == src.X && p.y == src.Y {
			return curr.x - src.X, curr.y - src.Y
		}
		curr = p
	}
}

// Render prints the ASCII map and player HP.
func (w *World) Render() {
	fmt.Println()
	for y := 0; y < w.Height; y++ {
		var builder strings.Builder
		for x := 0; x < w.Width; x++ {
			tile := w.Tiles[y][x]
			var ch = '.'
			if tile.Type == WallTile {
				ch = '#'
			} else if tile.Entity != nil {
				if tile.Entity.IsPlayer {
					ch = '@'
				} else {
					ch = 'g' // Generic monster.
				}
			} else if tile.Item != nil {
				ch = '!'
			}
			builder.WriteRune(ch)
		}
		fmt.Println(builder.String())
	}
	fmt.Printf("HP: %d/%d\n", w.Player.Stats.HP, w.Player.Stats.HPMax)
}

// PlayerPickUp picks up the item on the player's current tile.
func (w *World) PlayerPickUp() {
	tile := w.Tiles[w.Player.Y][w.Player.X]
	if tile.Item == nil {
		fmt.Println("Здесь нет предметов")
		return
	}
	item := *tile.Item
	w.Player.Inv = append(w.Player.Inv, item)
	fmt.Printf("Подобрали: %s\n", item.Name)
	tile.Item = nil
}

// PlayerUseItem uses the item at the given index in the player's inventory.
func (w *World) PlayerUseItem(idx int) {
	if idx < 0 || idx >= len(w.Player.Inv) {
		fmt.Println("Неверный индекс")
		return
	}
	item := w.Player.Inv[idx]
	if item.Heal > 0 {
		healAmt := item.Heal
		w.Player.Stats.HP += healAmt
		if w.Player.Stats.HP > w.Player.Stats.HPMax {
			w.Player.Stats.HP = w.Player.Stats.HPMax
		}
		fmt.Printf("Использовано %s, восстановлено %d HP\n", item.Name, healAmt)
	}
	// Remove used item.
	w.Player.Inv = append(w.Player.Inv[:idx], w.Player.Inv[idx+1:]...)
}

// abs returns the absolute value of a.
func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

// setupWorld initializes the world with dimensions.
func setupWorld(randSrc rand.Source) *World {
	const (
		width  = 25
		height = 12
	)
	r := rand.New(randSrc)
	return NewWorld(width, height, r)
}

// setupPlayer creates and places the player in the world.
func setupPlayer(world *World) {
	player := &Entity{
		Name:     "Игрок",
		Stats:    Stats{HPMax: 30, HP: 30, Attack: 5, Defense: 2, Speed: 5},
		IsPlayer: true,
	}
	world.PlaceEntity(player, world.Width/2, world.Height/2)
}

// spawnMonsters adds a specified number of monsters to the world.
func spawnMonsters(world *World, numMonsters int) {
	const borderOffset = 4
	for i := 0; i < numMonsters; i++ {
		x := world.Rand.Intn(world.Width-borderOffset) + borderOffset/2
		y := world.Rand.Intn(world.Height-borderOffset) + borderOffset/2
		if world.Tiles[y][x].Entity == nil && world.Tiles[y][x].Type != WallTile {
			monster := &Entity{
				Name:     "Гоблин",
				Stats:    Stats{HPMax: 8, HP: 8, Attack: 3, Defense: 0, Speed: 3},
				IsPlayer: false,
				AIType:   "basic",
			}
			world.PlaceEntity(monster, x, y)
		}
	}
}

// spawnItems places a specified number of items in the world.
func spawnItems(world *World, numItems int) {
	const borderOffset = 4
	for i := 0; i < numItems; i++ {
		x := world.Rand.Intn(world.Width-borderOffset) + borderOffset/2
		y := world.Rand.Intn(world.Height-borderOffset) + borderOffset/2
		if world.Tiles[y][x].Item == nil && world.Tiles[y][x].Entity == nil && world.Tiles[y][x].Type != WallTile {
			item := Item{Name: "Фляга здоровья", Heal: 8}
			world.PlaceItem(item, x, y)
		}
	}
}

// runGameLoop handles the main game loop: input, player actions, monster turns, and cleanup.
func runGameLoop(world *World) {
	reader := bufio.NewReader(os.Stdin)
	for {
		world.Render()
		if world.Player.Stats.HP <= 0 {
			fmt.Println("Вы погибли. Игра окончена.")
			return
		}

		fmt.Print("<<Command (w/a/s/d, p pick up, i inv, u use <i>, q quit)>>: ")
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Input error: %v\n", err)
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, " ")
		cmd := parts[0]
		switch cmd {
		case "q":
			fmt.Println("Выход")
			return
		case "w", "a", "s", "d":
			dx, dy := 0, 0
			switch cmd {
			case "w":
				dy = -1
			case "s":
				dy = 1
			case "a":
				dx = -1
			case "d":
				dx = 1
			}
			world.MoveEntity(world.Player, world.Player.X+dx, world.Player.Y+dy)
		case "p":
			world.PlayerPickUp()
		case "i":
			fmt.Println("Инвентарь:")
			for idx, item := range world.Player.Inv {
				fmt.Printf("[%d] %s (heal:%d)\n", idx, item.Name, item.Heal)
			}
		case "u":
			if len(parts) < 2 {
				fmt.Println("u <index>")
				continue
			}
			idx, err := strconv.Atoi(parts[1])
			if err != nil {
				fmt.Println("Неверный индекс")
				continue
			}
			world.PlayerUseItem(idx)
		default:
			fmt.Println("Неизвестная команда")
		}

		// Monster turns: simple chase AI.
		for _, entity := range world.Entities {
			if entity.IsPlayer || !entity.Alive {
				continue
			}
			dx := world.Player.X - entity.X
			dy := world.Player.Y - entity.Y
			if abs(dx)+abs(dy) == 1 {
				world.resolveMelee(entity, world.Player)
			} else {
				stepX, stepY := world.BFSStepTowards(entity, world.Player)
				if stepX != 0 || stepY != 0 {
					world.MoveEntity(entity, entity.X+stepX, entity.Y+stepY)
				}
			}
		}

		// Cleanup.
		world.RemoveDeadEntities()
	}
}

func main() {
	var randSrc = rand.NewSource(time.Now().UnixNano())
	world := setupWorld(randSrc)
	setupPlayer(world)
	spawnMonsters(world, 6)
	spawnItems(world, 5)

	runGameLoop(world)
}
