package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/pprof"
	"sort"

	"github.com/crillab/gophersat/bf"
)

func main() {
	players := flag.Int("players", 9, "number of players")
	size := flag.Int("size", 3, "size of each group")
	profile := flag.String("profile", "", "write cpu profile")
	flag.Parse()

	if *players == 0 {
		log.Fatal("missing number of players")
	}

	if *profile != "" {
		f, err := os.Create(*profile)
		if err != nil {
			log.Fatal(err)
		}

		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	solve(*players, *size)
}

func groupName(players ...int) (v string) {
	sort.Ints(players)

	for _, player := range players {
		v += fmt.Sprintf("%d,", player)
	}

	return v[:len(v)-1]
}

func solve(players int, size int) {
	f := bf.True

	// Prevent a player from being in the same group twice.
	/*
		for i := 0; i < players; i += 1 {
			for j := 0; j < players; j += 1 {
				for k := 0; k < players; k += 1 {
					if i == j || j == k || k == i {
						f = bf.And(f, bf.Not(bf.Var(groupName(i, j, k))))
					}
				}
			}
		}
	*/

	// Make sure we play everybody at least once.
	for i := 0; i < players; i += 1 {
		for j := 0; j < players; j += 1 {
			if i == j {
				continue
			}

			f3 := bf.False
			for k := 0; k < players; k += 1 {
				if k == i || k == j {
					continue
				}

				f3 = bf.Or(f3, bf.Var(groupName(i, j, k)))
			}

			f = bf.And(f, f3)
		}
	}

	// Make sure we don't repeat an opponent.
	for i := 0; i < players; i += 1 {
		for j := 0; j < players; j += 1 {
			if i == j {
				continue
			}

			combos := make([]string, 0, players-2)
			for k := 0; k < players; k += 1 {
				if k == i || k == j {
					continue
				}

				combos = append(combos, groupName(i, j, k))
			}

			f = bf.And(f, bf.Unique(combos...))
		}
	}

	model := bf.Solve(f)
	if model == nil {
		log.Fatal("no answer")
	}

	for i := 0; i < players; i += 1 {
		for j := i + 1; j < players; j += 1 {
			for k := j + 1; k < players; k += 1 {
				if model[groupName(i, j, k)] {
					fmt.Printf("%d %d %d\n", i, j, k)
				}
			}
		}
	}

}
