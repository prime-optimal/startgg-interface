package main

import (
	"fmt"
	"jacobrlewis/startgg-interface/startgg"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	// .env is optional: when absent we rely on api_key already being in the
	// environment (e.g. injected by `fnox exec`).
	_ = godotenv.Load()

	apiKey := os.Getenv("api_key")
	if apiKey == "" {
		log.Fatal("api_key not set: provide it via .env or `fnox exec -- go run .`")
	}

	client := startgg.CreateClient(apiKey)

	const slug = "genesis-x"
	id := client.GetTournamentIdFromSlug(slug)
	fmt.Printf("%s -> tournament id %d\n", slug, id)

	// --- "Tournament page" data, assembled from read queries ---
	fmt.Println("\nEvents:")
	for _, e := range client.GetEvents(slug) {
		fmt.Printf("  %d  %s\n", e.Id, e.Name)
	}

	const meleeSingles = 985241 // Genesis X "Melee Singles"

	_, total := client.GetEntrants(meleeSingles, 1)
	fmt.Printf("\nMelee Singles: %d entrants\n", total)

	fmt.Println("\nTop 8 standings:")
	for _, s := range client.GetStandings(meleeSingles, 8) {
		fmt.Printf("  %d. %s\n", s.Placement, s.Entrant.Name)
	}

	fmt.Println("\nTop 8 sets:")
	for _, n := range client.GetTop8(meleeSingles) {
		fmt.Printf("  [%s] %s\n", n.FullRoundText, n.DisplayScore)
	}
}
