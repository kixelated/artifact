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

var printMutex sync.Mutex

var mutateCount uint64

const threadSize = 32
const maxPlayers = 16 // TODO use

type Group struct {
	Players []int
}

func NewGroup(player int) (g *Group) {
	g = new(Group)
	g.Players = []int{player}
	return g
}

func (g *Group) Copy() (g2 *Group) {
	g2 = new(Group)
	*g2 = *g
	g2.Players = append([]int{}, g.Players...)
	return g2
}

type Tournament struct {
	PlayerSize int
	GroupSize  int

	Groups []*Group
	Played [maxPlayers]uint16
}

func NewTournament(playerSize int, groupSize int) (t *Tournament) {
	t = new(Tournament)
	t.PlayerSize = playerSize
	t.GroupSize = groupSize
	return t
}

func (t *Tournament) Copy() (t2 *Tournament) {
	t2 = new(Tournament)
	*t2 = *t

	t2.Groups = make([]*Group, len(t.Groups))
	for i := range t.Groups {
		t2.Groups[i] = t.Groups[i].Copy()
	}

	return t2
}

func (t *Tournament) CanAddGroup(player int) (ok bool) {
	if len(t.Groups) == 0 {
		return true
	}

	pending := t.Groups[len(t.Groups)-1]
	if len(pending.Players) < t.GroupSize {
		return false
	}

	/*
		for _, group := range t.Groups[:len(t.Groups)-1] {
			// TODO for now, make sure all group sizes are equal
			if len(group.Players) != len(pending.Players) {
				return false
			}
		}
	*/

	for _, group := range t.Groups {
		leader := group.Players[0]

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
	if len(pending.Players) >= t.GroupSize {
		return false
	}

	for _, other := range pending.Players {
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
	g := NewGroup(player)
	t.Groups = append(t.Groups, g)
}

func (t *Tournament) AddPlayer(player int) {
	pending := t.Groups[len(t.Groups)-1]

	for _, other := range pending.Players {
		t.Played[player] |= 1 << uint(other)
		t.Played[other] |= 1 << uint(player)
	}

	// Append to the last group in the array.
	pending.Players = append(pending.Players, player)
}

func (t *Tournament) RemoveGroup() {
	t.Groups = t.Groups[:len(t.Groups)-1]
}

func (t *Tournament) RemovePlayer() {
	pending := t.Groups[len(t.Groups)-1]

	player := pending.Players[len(pending.Players)-1]
	pending.Players = pending.Players[:len(pending.Players)-1]

	for _, other := range pending.Players {
		t.Played[player] &= ^(1 << uint(other))
		t.Played[other] &= ^(1 << uint(player))
	}
}

func (t Tournament) Score() (score int) {
	matches := make([]int, t.PlayerSize)

	for _, g := range t.Groups {
		// Make sure all of the groups are filled.
		if len(g.Players) < t.GroupSize {
			return 0
		}

		// Number of matches for each player.
		// round-robin means play everybody else
		playerMatches := len(g.Players) - 1

		for _, player := range g.Players {
			matches[player] += playerMatches
		}

		// Number of matches in the group total.
		groupMatches := (playerMatches*playerMatches + playerMatches) / 2
		score += groupMatches
	}

	// Make sure all players have the same number of matches.
	expected := matches[0]
	for _, match := range matches[1:] {
		if expected != match {
			return 0
		}
	}

	score += len(t.Groups)

	return score
}

func (t *Tournament) Mutate() (best *Tournament) {
	//t.Print()

	newCount := atomic.AddUint64(&mutateCount, 1)
	if bits.OnesCount64(newCount) == 1 {
		fmt.Printf("mutations: %d\n", newCount)
	}

	best = nil
	score := 0

	for i := 0; i < t.PlayerSize; i += 1 {
		if t.CanAddGroup(i) {
			t.AddGroup(i)

			t2 := t.Mutate()
			if t2 != nil {
				s2 := t2.Score()
				if s2 > score {
					best = t2
					score = s2
				}
			}

			t.RemoveGroup()
		}

		if t.CanAddPlayer(i) {
			t.AddPlayer(i)

			t2 := t.Mutate()
			if t2 != nil {
				s2 := t2.Score()
				if s2 > score {
					best = t2
					score = s2
				}
			}

			t.RemovePlayer()
		}
	}

	if t.Score() > score {
		best = t.Copy()
	}

	return best
}

func (t Tournament) Print() {
	printMutex.Lock()
	defer printMutex.Unlock()

	for _, g := range t.Groups {
		for _, p := range g.Players {
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

	if *players > 16 {
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

	if b == nil {
		log.Fatal("no result")
	}

	fmt.Println("result:")
	b.Print()
}
