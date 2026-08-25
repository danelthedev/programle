package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"runtime"
)

func main() {
	step := flag.String("step", "all", "whitelist|github|tiobe|generate|years|snippets|all")
	flag.Parse()
	base := findBase()
	need := func(s string) bool { return *step == "all" || *step == s }
	if need("whitelist") {
		fmt.Println("== whitelist ==")
		if err := runWhitelist(base); err != nil {
			log.Fatal(err)
		}
	}
	if need("github") {
		fmt.Println("== github ==")
		if err := runGithub(base); err != nil {
			log.Fatal(err)
		}
	}
	if need("tiobe") {
		fmt.Println("== tiobe ==")
		if err := runTiobe(base); err != nil {
			log.Fatal(err)
		}
	}
	if need("generate") {
		fmt.Println("== generate ==")
		if err := runGenerate(base); err != nil {
			log.Fatal(err)
		}
	}
	if need("years") {
		fmt.Println("== years ==")
		if err := runYears(base); err != nil {
			log.Printf("years warn: %v", err)
		}
	}
	if need("snippets") {
		fmt.Println("== snippets ==")
		if err := runSnippets(base); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println("pipeline done")
}

func findBase() string {
	_, f, _, ok := runtime.Caller(0)
	if ok {
		return filepath.Join(filepath.Dir(f), "..")
	}
	return "."
}
