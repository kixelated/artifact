package main

import (
	"fmt"
	"sync"
)

var threadMutex sync.RWMutex
var threadCount int

const threadSize = 32

type Round struct {
	Players []int
}

func NewRound(player int) (r *Round) {
	r = new(Round)
	r.Players = append(r.Players, player)
	return r
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
	Played map[int]map[int]struct{}
}

func NewTournament(playerSize int, roundSize int) (t *Tournament) {
	t = new(Tournament)
	t.PlayerSize = playerSize
	t.RoundSize = roundSize

	t.Played = make(map[int]map[int]struct{})
	for i := 0; i < t.PlayerSize; i += 1 {
		t.Played[i] = make(map[int]struct{})
	}

	return t
}

func (t *Tournament) Copy() (t2 *Tournament) {
	t2 = new(Tournament)
	t2.PlayerSize = t.PlayerSize
	t2.RoundSize = t.RoundSize

	t2.Rounds = append([]*Round{}, t.Rounds...)
	for i := range t2.Rounds {
		t2.Rounds[i] = t2.Rounds[i].Copy()
	}

	t2.Played = make(map[int]map[int]struct{})

	for player := range t.Played {
		newMap := make(map[int]struct{})

		for other := range t.Played[player] {
			newMap[other] = struct{}{}
		}

		t2.Played[player] = newMap
	}

	return t2
}

func (t *Tournament) CanAdd(player int) (ok bool) {
	if len(t.Rounds) == 0 {
		return true
	}

	pending := t.Rounds[len(t.Rounds)-1]
	if len(pending.Players) == t.RoundSize {
		return true
	}

	played := t.Played[player]

	for _, other := range pending.Players {
		// Only allow inserting in order to avoid duplicate work.
		if player <= other {
			return false
		}

		_, ok := played[other]
		if ok {
			return false
		}
	}

	return true
}

func (t *Tournament) Add(player int) {
	if len(t.Rounds) == 0 {
		t.Rounds = append(t.Rounds, NewRound(player))
		return
	}

	pending := t.Rounds[len(t.Rounds)-1]
	if len(pending.Players) == t.RoundSize {
		t.Rounds = append(t.Rounds, NewRound(player))
		return
	}

	for _, other := range pending.Players {
		t.Played[player][other] = struct{}{}
		t.Played[other][player] = struct{}{}
	}

	pending.Players = append(pending.Players, player)
}

func (t *Tournament) Score() int {
	sum := len(t.Rounds)

	for _, r := range t.Rounds {
		sum += len(r.Players)
	}

	return sum
}

func (t *Tournament) Mutate() (best *Tournament) {
	results := make(chan *Tournament, t.PlayerSize)

	for i := 0; i < t.PlayerSize; i += 1 {
		if !t.CanAdd(i) {
			results <- nil
			continue
		}

		t2 := t.Copy()
		t2.Add(i)

		threadMutex.RLock()
		parallel := threadCount < threadSize
		threadMutex.RUnlock()

		// Actually grab the lock to confirm our thread.
		if parallel {
			threadMutex.Lock()

			if threadCount < threadSize {
				threadCount += 1
			} else {
				parallel = false
			}

			threadMutex.Unlock()
		}

		if parallel {
			go func() {
				fmt.Println("new thread")

				results <- t2.Mutate()

				threadMutex.Lock()
				threadCount -= 1
				threadMutex.Unlock()

				fmt.Println("dead thread")
			}()
		} else {
			results <- t2.Mutate()
		}
	}

	best = t
	score := t.Score()

	for i := 0; i < t.PlayerSize; i += 1 {
		b2 := <-results
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
	for _, r := range t.Rounds {
		for _, p := range r.Players {
			fmt.Printf("%d ", p)
		}

		fmt.Println()
	}

	fmt.Println()
}

func main() {
	t := NewTournament(9, 3)
	b := t.Mutate()
	b.Print()
}
