package main

import (
	"flag"
	"fmt"
	"log"
	"math/bits"
	"os"
	"runtime/pprof"
	"sync"
	"sync/atomic"
)

const maxPlayers = 16

var printMutex sync.Mutex
var mutateCount uint64

type Group struct {
	players uint32 // every 4 bits is a player id (max 16)
	size    int
}

func (g *Group) Add(player int) {
	g.players |= (uint32(player) & 0xf) << (uint(g.size) * 4)
	g.size += 1
}

func (g *Group) Remove() (player int) {
	g.size -= 1
	player = g.Player(g.size)
	g.players &= ^(0xf << (4 * uint(g.size)))

	return player
}

func (g Group) Player(index int) (player int) {
	return int((g.players >> (uint(index) * 4)) & 0xf)
}

func (g Group) Size() int {
	return g.size
}

type Tournament struct {
	PlayerSize int
	GroupSize  int

	Groups []Group
	Played [maxPlayers]uint16 // bitmask if they've played each player
}

func NewTournament(playerSize int, groupSize int) (t *Tournament) {
	t = new(Tournament)
	t.PlayerSize = playerSize
	t.GroupSize = groupSize
	return t
}

func (t Tournament) Copy() (t2 Tournament) {
	t2 = t

	// Make a copy of the groups slice
	t2.Groups = append([]Group{}, t.Groups...)

	return t2
}

func (t *Tournament) CanAddGroup(player int) (ok bool) {
	if len(t.Groups) == 0 {
		return true
	}

	pending := t.Groups[len(t.Groups)-1]
	if pending.Size() < t.GroupSize {
		return false
	}

	for _, group := range t.Groups {
		leader := group.Player(0)

		// Insert groups in order to avoid duplicate work.
		if leader > player {
			return false
		}
	}

	myMatches := bits.OnesCount16(t.Played[player])

	for i := 0; i < player; i += 1 {
		otherMatches := bits.OnesCount16(t.Played[i])
		if otherMatches < myMatches {
			return false
		}
	}

	return true
}

func (t Tournament) CanAddPlayer(player int) (ok bool) {
	if len(t.Groups) == 0 {
		return false
	}

	pending := t.Groups[len(t.Groups)-1]
	if pending.Size() >= t.GroupSize {
		return false
	}

	// Prevent duplicate work by scanning the previous group.
	if len(t.Groups) >= 2 {
		prev := t.Groups[len(t.Groups)-2]
		similar := true

		// The idea is that we should do work in ascending order.
		// If the previous group was: "0 3 4":
		// Then we know that we must have tried "0 1 *" and "0 2 *" in another branch.

		for i := 0; i < pending.Size(); i += 1 {
			if pending.Player(i) > prev.Player(i) {
				similar = false
				break
			}
		}

		if similar && player < prev.Player(pending.Size()) {
			return false
		}
	}

	for i := 0; i < pending.Size(); i += 1 {
		other := pending.Player(i)

		// Only allow inserting in order to avoid duplicate work.
		if player <= other {
			return false
		}

		// Prevent rematches.
		if t.Played[player]&(1<<uint(other)) != 0 {
			return false
		}
	}

	return true
}

func (t *Tournament) AddGroup(player int) {
	g := Group{}
	g.Add(player)

	t.Groups = append(t.Groups, g)
}

func (t *Tournament) AddPlayer(player int) {
	pending := &t.Groups[len(t.Groups)-1]

	for i := 0; i < pending.Size(); i += 1 {
		other := pending.Player(i)

		t.Played[player] |= 1 << uint(other)
		t.Played[other] |= 1 << uint(player)
	}

	// Append to the last group in the array.
	pending.Add(player)
}

func (t *Tournament) RemoveGroup() {
	t.Groups = t.Groups[:len(t.Groups)-1]
}

func (t *Tournament) RemovePlayer() {
	pending := &t.Groups[len(t.Groups)-1]

	player := pending.Remove()

	for i := 0; i < pending.Size(); i += 1 {
		other := pending.Player(i)

		t.Played[player] &= ^(1 << uint(other))
		t.Played[other] &= ^(1 << uint(player))
	}
}

func (t Tournament) Score() (score int) {
	if len(t.Groups) == 0 {
		return 0
	}

	// Make sure all of the groups are filled.
	if t.Groups[len(t.Groups)-1].Size() < t.GroupSize {
		return 0
	}

	// Make sure all players have the same number of matches.
	count := bits.OnesCount16(t.Played[0])

	for i := 1; i < t.PlayerSize; i += 1 {
		if bits.OnesCount16(t.Played[i]) != count {
			return 0
		}
	}

	// Score by the total number of matches and prefer more groups.
	return int(count)*t.PlayerSize + len(t.Groups)
}

func (t *Tournament) Mutate() (best Tournament) {
	//t.Print()

	newCount := atomic.AddUint64(&mutateCount, 1)
	if bits.OnesCount64(newCount) == 1 {
		fmt.Printf("mutations: %d\n", newCount)
	}

	score := 0

	for i := 0; i < t.PlayerSize; i += 1 {
		if t.CanAddGroup(i) {
			t.AddGroup(i)

			t2 := t.Mutate()
			s2 := t2.Score()
			if s2 > score {
				best = t2
				score = s2
			}

			t.RemoveGroup()
		}

		if t.CanAddPlayer(i) {
			t.AddPlayer(i)

			t2 := t.Mutate()
			s2 := t2.Score()
			if s2 > score {
				best = t2
				score = s2
			}

			t.RemovePlayer()
		}
	}

	if t.Score() > score {
		best = t.Copy()
	}

	return best
}

func (t *Tournament) Print() {
	printMutex.Lock()
	defer printMutex.Unlock()

	for _, g := range t.Groups {
		for i := 0; i < g.Size(); i += 1 {
			p := g.Player(i)
			fmt.Printf("%d ", p)
		}

		fmt.Println()
	}

	fmt.Println()
}

func main() {
	players := flag.Int("players", 0, "number of players")
	size := flag.Int("size", 4, "size of each group")
	profile := flag.String("profile", "", "write cpu profile")
	flag.Parse()

	if *players == 0 {
		log.Fatal("missing number of players")
	}

	if *players > maxPlayers {
		log.Fatal("too many players")
	}

	if *profile != "" {
		f, err := os.Create(*profile)
		if err != nil {
			log.Fatal(err)
		}

		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	t := NewTournament(*players, *size)
	b := t.Mutate()

	fmt.Printf("mutations: %d\n\n", mutateCount)

	fmt.Println("result:")
	b.Print()
}
