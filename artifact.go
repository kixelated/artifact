package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/pprof"
	"sort"

	"github.com/frrad/gophersat/bf"
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

	err := solve(*players, *size)
	if err != nil {
		log.Fatal(err)
	}
}

func groupName(players ...int) (v string) {
	sort.Ints(players)

	for _, player := range players {
		v += fmt.Sprintf("%d,", player)
	}

	return v[:len(v)-1]
}

func solve(players int, size int) (err error) {
	f := bf.True

	// Make sure we play everybody at least once.
	for i := 0; i < players; i += 1 {
		for j := 0; j < players; j += 1 {
			if i == j {
				continue
			}

			groups := make([]string, 0, players*(players-2))

			for k := 0; k < players; k += 1 {
				if k == i || k == j {
					continue
				}

				groups = append(groups, groupName(i, j, k))
			}

			f = bf.And(f, bf.Unique(groups...))
		}
	}

	model := bf.Solve(f)
	if model == nil {
		return fmt.Errorf("no solution")
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

	/*
		out, err := os.Create("artifact.cnf")
		if err != nil {
			return err
		}

		defer out.Close()

		err = bf.Dimacs(f, out)
		if err != nil {
			return err
		}
	*/

	return nil
}
