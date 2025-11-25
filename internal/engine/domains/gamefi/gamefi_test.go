package gamefi

import (
	"math/big"
	"testing"
	"time"
)

func TestGameAsset(t *testing.T) {
	asset := GameAsset{
		ID:          "asset-1",
		GameID:      "game-1",
		Type:        AssetTypeWeapon,
		Name:        "Dragon Slayer",
		Description: "A legendary sword",
		Rarity:      RarityLegendary,
		Attributes:  map[string]any{"damage": 100, "durability": 500},
		ImageURL:    "https://example.com/sword.png",
		TokenID:     "token-123",
		Contract:    "0x1234",
		ChainID:     "neo-mainnet",
		Tradable:    true,
	}

	if asset.Type != AssetTypeWeapon {
		t.Errorf("expected weapon, got %s", asset.Type)
	}
	if asset.Rarity != RarityLegendary {
		t.Errorf("expected legendary, got %s", asset.Rarity)
	}
}

func TestAssetTypes(t *testing.T) {
	types := []AssetType{
		AssetTypeCharacter,
		AssetTypeWeapon,
		AssetTypeArmor,
		AssetTypeConsumable,
		AssetTypeLand,
		AssetTypePet,
		AssetTypeVehicle,
		AssetTypeCurrency,
		AssetTypeTicket,
		AssetTypeLootbox,
	}

	for _, at := range types {
		asset := GameAsset{Type: at}
		if asset.Type != at {
			t.Errorf("expected %s, got %s", at, asset.Type)
		}
	}
}

func TestRarity(t *testing.T) {
	rarities := []Rarity{
		RarityCommon,
		RarityUncommon,
		RarityRare,
		RarityEpic,
		RarityLegendary,
		RarityMythic,
	}

	for _, r := range rarities {
		asset := GameAsset{Rarity: r}
		if asset.Rarity != r {
			t.Errorf("expected %s, got %s", r, asset.Rarity)
		}
	}
}

func TestPlayer(t *testing.T) {
	now := time.Now()
	player := Player{
		ID:         "player-1",
		GameID:     "game-1",
		Wallet:     "NeoAddr123",
		Username:   "DragonSlayer42",
		Level:      50,
		Experience: 125000,
		JoinedAt:   now.Add(-24 * time.Hour * 365),
		LastActive: now,
		Stats: PlayerStats{
			TotalGames:    500,
			Wins:          350,
			Losses:        150,
			WinRate:       0.7,
			TotalEarnings: 10000.0,
			TotalSpending: 500.0,
			PlayTime:      200 * time.Hour,
		},
	}

	if player.Level != 50 {
		t.Errorf("expected level 50, got %d", player.Level)
	}
	if player.Stats.WinRate != 0.7 {
		t.Errorf("expected win rate 0.7, got %f", player.Stats.WinRate)
	}
}

func TestAchievement(t *testing.T) {
	ach := Achievement{
		ID:          "ach-1",
		GameID:      "game-1",
		Name:        "First Blood",
		Description: "Win your first battle",
		Points:      10,
		Rarity:      RarityCommon,
		ImageURL:    "https://example.com/badge.png",
		UnlockedAt:  time.Now(),
	}

	if ach.Points != 10 {
		t.Errorf("expected 10 points, got %d", ach.Points)
	}
}

func TestLeaderboardEntry(t *testing.T) {
	entry := LeaderboardEntry{
		Rank:      1,
		PlayerID:  "player-1",
		Username:  "ChampionX",
		Score:     999999,
		UpdatedAt: time.Now(),
	}

	if entry.Rank != 1 {
		t.Errorf("expected rank 1, got %d", entry.Rank)
	}
}

func TestGame(t *testing.T) {
	game := Game{
		ID:          "game-1",
		Name:        "Neo Legends",
		Description: "Epic blockchain RPG",
		Developer:   "R3E Network",
		Genre:       []string{"RPG", "Adventure"},
		ChainID:     "neo-mainnet",
		Contracts: map[string]string{
			"game":   "0x1234",
			"assets": "0x5678",
		},
		Status:      GameStatusActive,
		PlayerCount: 10000,
		LaunchedAt:  time.Now().Add(-24 * time.Hour * 30),
	}

	if game.Status != GameStatusActive {
		t.Errorf("expected active, got %s", game.Status)
	}
	if len(game.Contracts) != 2 {
		t.Errorf("expected 2 contracts, got %d", len(game.Contracts))
	}
}

func TestGameStatus(t *testing.T) {
	statuses := []GameStatus{
		GameStatusActive,
		GameStatusMaintenance,
		GameStatusBeta,
		GameStatusShutdown,
	}

	for _, s := range statuses {
		game := Game{Status: s}
		if game.Status != s {
			t.Errorf("expected %s, got %s", s, game.Status)
		}
	}
}

func TestMatch(t *testing.T) {
	now := time.Now()
	match := Match{
		ID:        "match-1",
		GameID:    "game-1",
		Players:   []string{"player-1", "player-2"},
		Winner:    "player-1",
		Status:    MatchStatusComplete,
		Mode:      "1v1",
		StartedAt: now.Add(-30 * time.Minute),
		EndedAt:   now,
		Duration:  30 * time.Minute,
		Results: MatchResults{
			Scores: map[string]int64{
				"player-1": 1500,
				"player-2": 1200,
			},
			Rewards: map[string][]Reward{
				"player-1": {{Type: RewardTypeToken, Amount: big.NewInt(100)}},
			},
		},
	}

	if match.Status != MatchStatusComplete {
		t.Errorf("expected complete, got %s", match.Status)
	}
	if match.Winner != "player-1" {
		t.Errorf("expected player-1 winner, got %s", match.Winner)
	}
}

func TestQuest(t *testing.T) {
	quest := Quest{
		ID:          "quest-1",
		GameID:      "game-1",
		Name:        "Dragon Hunt",
		Description: "Defeat 10 dragons",
		Type:        QuestTypeWeekly,
		Objectives: []Objective{
			{ID: "obj-1", Description: "Kill dragons", Target: 10, Current: 5, Completed: false},
		},
		Rewards: []Reward{
			{Type: RewardTypeNFT, AssetID: "dragon-sword"},
			{Type: RewardTypeExperience, Amount: big.NewInt(1000)},
		},
		ExpiresAt:  time.Now().Add(7 * 24 * time.Hour),
		Difficulty: "hard",
	}

	if quest.Type != QuestTypeWeekly {
		t.Errorf("expected weekly, got %s", quest.Type)
	}
	if len(quest.Objectives) != 1 {
		t.Errorf("expected 1 objective, got %d", len(quest.Objectives))
	}
}

func TestTournament(t *testing.T) {
	tournament := Tournament{
		ID:          "tourney-1",
		GameID:      "game-1",
		Name:        "World Championship",
		Description: "Annual tournament",
		PrizePool:   100000.0,
		EntryFee:    10.0,
		MaxPlayers:  256,
		Players:     []string{"player-1", "player-2"},
		Status:      TournamentStatusRegistration,
		StartsAt:    time.Now().Add(24 * time.Hour),
		EndsAt:      time.Now().Add(48 * time.Hour),
		Rules:       map[string]any{"format": "double_elimination"},
	}

	if tournament.Status != TournamentStatusRegistration {
		t.Errorf("expected registration, got %s", tournament.Status)
	}
	if tournament.PrizePool != 100000.0 {
		t.Errorf("expected prize pool 100000, got %f", tournament.PrizePool)
	}
}

func TestGameFiInfo(t *testing.T) {
	info := GameFiInfo{
		SupportedGames: []string{"game-1", "game-2"},
		Chains:         []string{"neo-mainnet", "eth-mainnet"},
		Capabilities:   []string{"player", "asset", "match", "leaderboard"},
	}

	if len(info.SupportedGames) != 2 {
		t.Errorf("expected 2 games, got %d", len(info.SupportedGames))
	}
	if len(info.Capabilities) != 4 {
		t.Errorf("expected 4 capabilities, got %d", len(info.Capabilities))
	}
}
