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

type Round struct {
	Players []int
}

func (r *Round) Copy() (r2 *Round) {
	r2 = new(Round)
	r2.Players = append([]int{}, r.Players...)
	return r2
}

type Tournament struct {
	PlayerSize int
	RoundSize  int

	Rounds []*Round
	Played [maxPlayers]uint16 // bitset
}

func NewTournament(playerSize int, roundSize int) (t *Tournament) {
	t = new(Tournament)
	t.PlayerSize = playerSize
	t.RoundSize = roundSize
	return t
}

func (t *Tournament) Copy() (t2 *Tournament) {
	t2 = new(Tournament)
	*t2 = *t
	return t2
}

func (t *Tournament) CanAddRound() (ok bool) {
	if len(t.Rounds) == 0 {
		return true
	}

	pending := t.Rounds[len(t.Rounds)-1]
	if len(pending.Players) < t.RoundSize {
		return false
	}

	for _, round := range t.Rounds {
		// TODO for now, make sure all round sizes are equal
		if len(round.Players) != len(pending.Players) {
			return false
		}
	}

	return true
}

func (t *Tournament) CanAddPlayer(player int) (ok bool) {
	if len(t.Rounds) == 0 {
		return false
	}

	// Insert rounds in order to avoid duplicate work.
	for _, round := range t.Rounds {
		if len(round.Players) > 0 && round.Players[0] > player {
			return false
		}
	}

	pending := t.Rounds[len(t.Rounds)-1]

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

func (t *Tournament) AddRound() (t2 *Tournament) {
	t2 = new(Tournament)
	*t2 = *t

	// Make a copy of the rounds array.
	t2.Rounds = append([]*Round{}, t2.Rounds...)
	t2.Rounds = append(t2.Rounds, new(Round))

	return t2
}

func (t *Tournament) AddPlayer(player int) (t2 *Tournament) {
	t2 = new(Tournament)
	*t2 = *t

	pending := t2.Rounds[len(t2.Rounds)-1]

	for _, other := range pending.Players {
		t2.Played[player] |= 1 << uint(other)
		t2.Played[other] |= 1 << uint(player)
	}

	// Make a copy of the rounds array.
	t2.Rounds = append([]*Round{}, t2.Rounds...)

	// Make a copy of the last round.
	pending = pending.Copy()
	pending.Players = append(pending.Players, player)

	// Add it back to the rounds.
	t2.Rounds[len(t2.Rounds)-1] = pending

	return t2
}

func (t *Tournament) Score() (score int) {
	matches := make([]int, t.PlayerSize)

	for _, r := range t.Rounds {
		// Make sure all of the rounds are filled.
		if len(r.Players) < t.RoundSize {
			return 0
		}

		// Number of matches for each player. (round-robin means play everybody else)
		playerMatches := len(r.Players) - 1

		for _, player := range r.Players {
			matches[player] += playerMatches
		}

		// Number of matches in the round total.
		roundMatches := (playerMatches*playerMatches + playerMatches) / 2
		score += roundMatches
	}

	// Make sure all players have the same number of matches.
	expected := matches[0]
	for _, match := range matches[1:] {
		if expected != match {
			return 0
		}
	}

	score += len(t.Rounds)

	return score
}

func (t *Tournament) Mutate() (best *Tournament) {
	//t.Print()

	newCount := atomic.AddUint64(&mutateCount, 1)
	if bits.OnesCount64(newCount) == 1 {
		fmt.Printf("mutations: %d\n", newCount)
	}

	results := make([]*Tournament, 0, t.PlayerSize+1)

	if t.CanAddRound() {
		t2 := t.AddRound()
		results = append(results, t2.Mutate())
	}

	for i := 0; i < t.PlayerSize; i += 1 {
		if !t.CanAddPlayer(i) {
			continue
		}

		t2 := t.AddPlayer(i)
		results = append(results, t2.Mutate())
	}

	best = t
	score := t.Score()

	for _, b2 := range results {
		if b2 == nil {
			continue
		}

		s2 := b2.Score()

		if s2 > score {
			best = b2
			score = s2
		}
	}

	return best
}

func (t *Tournament) Print() {
	printMutex.Lock()
	defer printMutex.Unlock()

	for _, r := range t.Rounds {
		for _, p := range r.Players {
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

	fmt.Println("result:")
	b.Print()
}
