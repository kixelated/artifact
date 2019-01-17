package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/pprof"
	"sort"

	//"github.com/frrad/gophersat/bf"
	"github.com/crillab/gophersat/maxsat"
)

func main() {
	players := flag.Int("players", 8, "number of players")
	size := flag.Int("size", 3, "size of each group")
	profile := flag.String("profile", "", "write cpu profile")
	flag.Parse()

	if *players == 0 {
		log.Fatal("missing number of players")
	}

	if *size != 3 {
		log.Fatal("not supported")
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
	c := []maxsat.Constr{}

	for i := 0; i < players; i += 1 {
		for j := i + 1; j < players; j += 1 {
			lits := make([]maxsat.Lit, 0, players-2)

			for k := 0; k < players; k += 1 {
				if i == k || j == k {
					continue
				}

				group := groupName(i, j, k)
				lits = append(lits, maxsat.Var(group))
			}

			c = append(c, maxsat.SoftClause(lits...))

			for k, l1 := range lits[:len(lits)-1] {
				for _, l2 := range lits[k+1:] {
					c = append(c, maxsat.HardClause(l1.Negation(), l2.Negation()))
				}
			}
		}
	}

	p := maxsat.New(c...)

	model, _ := p.Solve()
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

	return nil
}
