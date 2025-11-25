// Package gamefi provides GameFi domain engine interfaces for the service layer.
//
// This package defines standard abstractions for blockchain gaming operations including
// game assets, player management, achievements, leaderboards, and in-game economies.
//
// # Implementation Status
//
// This package contains fully-defined interface specifications and type systems. It serves as
// an architectural contract for GameFi service implementations. Concrete implementations will be
// developed in internal/services/ as gaming services are built out.
//
// Services implementing these interfaces are optional extensions to the base ServiceModule.
// They should implement the GameFiEngine composite interface along with specific sub-interfaces
// (PlayerEngine, AssetEngine, MatchEngine, etc.) for their supported capabilities.
package gamefi

import (
	"context"
	"math/big"
	"time"

	"github.com/R3E-Network/service_layer/internal/engine"
)

// Common GameFi types

// GameAsset represents an in-game asset (can be NFT or fungible).
type GameAsset struct {
	ID          string            `json:"id"`
	GameID      string            `json:"game_id"`
	Type        AssetType         `json:"type"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Rarity      Rarity            `json:"rarity"`
	Attributes  map[string]any    `json:"attributes,omitempty"`
	ImageURL    string            `json:"image_url,omitempty"`
	TokenID     string            `json:"token_id,omitempty"`
	Contract    string            `json:"contract,omitempty"`
	ChainID     string            `json:"chain_id"`
	Tradable    bool              `json:"tradable"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// AssetType identifies the type of game asset.
type AssetType string

const (
	AssetTypeCharacter  AssetType = "character"
	AssetTypeWeapon     AssetType = "weapon"
	AssetTypeArmor      AssetType = "armor"
	AssetTypeConsumable AssetType = "consumable"
	AssetTypeLand       AssetType = "land"
	AssetTypePet        AssetType = "pet"
	AssetTypeVehicle    AssetType = "vehicle"
	AssetTypeCurrency   AssetType = "currency"
	AssetTypeTicket     AssetType = "ticket"
	AssetTypeLootbox    AssetType = "lootbox"
)

// Rarity indicates asset rarity.
type Rarity string

const (
	RarityCommon    Rarity = "common"
	RarityUncommon  Rarity = "uncommon"
	RarityRare      Rarity = "rare"
	RarityEpic      Rarity = "epic"
	RarityLegendary Rarity = "legendary"
	RarityMythic    Rarity = "mythic"
)

// Player represents a game player.
type Player struct {
	ID          string            `json:"id"`
	GameID      string            `json:"game_id"`
	Wallet      string            `json:"wallet"`
	Username    string            `json:"username"`
	Level       int               `json:"level"`
	Experience  uint64            `json:"experience"`
	JoinedAt    time.Time         `json:"joined_at"`
	LastActive  time.Time         `json:"last_active"`
	Stats       PlayerStats       `json:"stats"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// PlayerStats holds player statistics.
type PlayerStats struct {
	TotalGames     uint64  `json:"total_games"`
	Wins           uint64  `json:"wins"`
	Losses         uint64  `json:"losses"`
	WinRate        float64 `json:"win_rate"`
	TotalEarnings  float64 `json:"total_earnings_usd"`
	TotalSpending  float64 `json:"total_spending_usd"`
	PlayTime       time.Duration `json:"play_time"`
}

// Achievement represents a game achievement.
type Achievement struct {
	ID          string    `json:"id"`
	GameID      string    `json:"game_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Points      int       `json:"points"`
	Rarity      Rarity    `json:"rarity"`
	ImageURL    string    `json:"image_url,omitempty"`
	UnlockedAt  time.Time `json:"unlocked_at,omitempty"`
}

// LeaderboardEntry represents an entry in a leaderboard.
type LeaderboardEntry struct {
	Rank      int       `json:"rank"`
	PlayerID  string    `json:"player_id"`
	Username  string    `json:"username"`
	Score     int64     `json:"score"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Game represents a blockchain game.
type Game struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Developer   string            `json:"developer"`
	Genre       []string          `json:"genre"`
	ChainID     string            `json:"chain_id"`
	Contracts   map[string]string `json:"contracts"` // role -> address
	Status      GameStatus        `json:"status"`
	PlayerCount uint64            `json:"player_count"`
	LaunchedAt  time.Time         `json:"launched_at"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// GameStatus indicates game operational status.
type GameStatus string

const (
	GameStatusActive      GameStatus = "active"
	GameStatusMaintenance GameStatus = "maintenance"
	GameStatusBeta        GameStatus = "beta"
	GameStatusShutdown    GameStatus = "shutdown"
)

// Match represents a game match or session.
type Match struct {
	ID         string        `json:"id"`
	GameID     string        `json:"game_id"`
	Players    []string      `json:"players"`
	Winner     string        `json:"winner,omitempty"`
	Status     MatchStatus   `json:"status"`
	Mode       string        `json:"mode"`
	StartedAt  time.Time     `json:"started_at"`
	EndedAt    time.Time     `json:"ended_at,omitempty"`
	Duration   time.Duration `json:"duration"`
	Results    MatchResults  `json:"results,omitempty"`
}

// MatchStatus indicates match status.
type MatchStatus string

const (
	MatchStatusPending  MatchStatus = "pending"
	MatchStatusActive   MatchStatus = "active"
	MatchStatusComplete MatchStatus = "complete"
	MatchStatusCanceled MatchStatus = "canceled"
)

// MatchResults holds match outcome data.
type MatchResults struct {
	Scores      map[string]int64  `json:"scores"`
	Rewards     map[string][]Reward `json:"rewards"`
	Statistics  map[string]any    `json:"statistics"`
}

// Reward represents a game reward.
type Reward struct {
	Type    RewardType `json:"type"`
	AssetID string     `json:"asset_id,omitempty"`
	Amount  *big.Int   `json:"amount,omitempty"`
	TokenID string     `json:"token_id,omitempty"`
}

// RewardType identifies reward type.
type RewardType string

const (
	RewardTypeToken      RewardType = "token"
	RewardTypeNFT        RewardType = "nft"
	RewardTypeExperience RewardType = "experience"
	RewardTypeItem       RewardType = "item"
)

// Quest represents an in-game quest.
type Quest struct {
	ID          string       `json:"id"`
	GameID      string       `json:"game_id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Type        QuestType    `json:"type"`
	Objectives  []Objective  `json:"objectives"`
	Rewards     []Reward     `json:"rewards"`
	ExpiresAt   time.Time    `json:"expires_at,omitempty"`
	Difficulty  string       `json:"difficulty"`
}

// QuestType identifies quest type.
type QuestType string

const (
	QuestTypeDaily   QuestType = "daily"
	QuestTypeWeekly  QuestType = "weekly"
	QuestTypeStory   QuestType = "story"
	QuestTypeEvent   QuestType = "event"
)

// Objective represents a quest objective.
type Objective struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Target      int    `json:"target"`
	Current     int    `json:"current"`
	Completed   bool   `json:"completed"`
}

// Tournament represents a game tournament.
type Tournament struct {
	ID           string          `json:"id"`
	GameID       string          `json:"game_id"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	PrizePool    float64         `json:"prize_pool_usd"`
	EntryFee     float64         `json:"entry_fee_usd"`
	MaxPlayers   int             `json:"max_players"`
	Players      []string        `json:"players"`
	Status       TournamentStatus `json:"status"`
	StartsAt     time.Time       `json:"starts_at"`
	EndsAt       time.Time       `json:"ends_at"`
	Rules        map[string]any  `json:"rules"`
}

// TournamentStatus indicates tournament status.
type TournamentStatus string

const (
	TournamentStatusRegistration TournamentStatus = "registration"
	TournamentStatusActive       TournamentStatus = "active"
	TournamentStatusComplete     TournamentStatus = "complete"
	TournamentStatusCanceled     TournamentStatus = "canceled"
)

// Engine interfaces

// GameEngine handles game registration and discovery.
type GameEngine interface {
	engine.ServiceModule

	// GetGame retrieves game information.
	GetGame(ctx context.Context, gameID string) (*Game, error)

	// ListGames lists registered games.
	ListGames(ctx context.Context, chainID string, status GameStatus, limit int) ([]Game, error)

	// RegisterGame registers a new game.
	RegisterGame(ctx context.Context, game *Game) error

	// UpdateGameStatus updates game status.
	UpdateGameStatus(ctx context.Context, gameID string, status GameStatus) error
}

// PlayerEngine handles player management.
type PlayerEngine interface {
	engine.ServiceModule

	// GetPlayer retrieves player information.
	GetPlayer(ctx context.Context, gameID, playerID string) (*Player, error)

	// GetPlayerByWallet retrieves player by wallet address.
	GetPlayerByWallet(ctx context.Context, gameID, wallet string) (*Player, error)

	// RegisterPlayer registers a new player.
	RegisterPlayer(ctx context.Context, gameID, wallet, username string) (*Player, error)

	// UpdatePlayerStats updates player statistics.
	UpdatePlayerStats(ctx context.Context, gameID, playerID string, stats PlayerStats) error

	// AddExperience adds experience points to a player.
	AddExperience(ctx context.Context, gameID, playerID string, exp uint64) error

	// ListPlayers lists players in a game.
	ListPlayers(ctx context.Context, gameID string, limit, offset int) ([]Player, error)
}

// AssetEngine handles game asset management.
type AssetEngine interface {
	engine.ServiceModule

	// GetAsset retrieves asset information.
	GetAsset(ctx context.Context, gameID, assetID string) (*GameAsset, error)

	// ListAssets lists assets for a player or game.
	ListAssets(ctx context.Context, gameID string, ownerWallet string, assetType AssetType, limit int) ([]GameAsset, error)

	// MintAsset mints a new game asset.
	MintAsset(ctx context.Context, gameID string, asset *GameAsset, recipient string) (*GameAsset, error)

	// TransferAsset transfers an asset between players.
	TransferAsset(ctx context.Context, gameID, assetID, from, to string) error

	// BurnAsset burns/destroys an asset.
	BurnAsset(ctx context.Context, gameID, assetID, owner string) error

	// UpdateAssetAttributes updates asset attributes.
	UpdateAssetAttributes(ctx context.Context, gameID, assetID string, attrs map[string]any) error
}

// MatchEngine handles match/session management.
type MatchEngine interface {
	engine.ServiceModule

	// CreateMatch creates a new match.
	CreateMatch(ctx context.Context, gameID, mode string, players []string) (*Match, error)

	// GetMatch retrieves match information.
	GetMatch(ctx context.Context, matchID string) (*Match, error)

	// UpdateMatchStatus updates match status.
	UpdateMatchStatus(ctx context.Context, matchID string, status MatchStatus) error

	// EndMatch ends a match with results.
	EndMatch(ctx context.Context, matchID string, results MatchResults) (*Match, error)

	// ListMatches lists matches for a game or player.
	ListMatches(ctx context.Context, gameID, playerID string, status MatchStatus, limit int) ([]Match, error)
}

// LeaderboardEngine handles leaderboard management.
type LeaderboardEngine interface {
	engine.ServiceModule

	// GetLeaderboard retrieves leaderboard entries.
	GetLeaderboard(ctx context.Context, gameID, leaderboardID string, limit, offset int) ([]LeaderboardEntry, error)

	// GetPlayerRank gets a player's rank on a leaderboard.
	GetPlayerRank(ctx context.Context, gameID, leaderboardID, playerID string) (*LeaderboardEntry, error)

	// UpdateScore updates a player's score.
	UpdateScore(ctx context.Context, gameID, leaderboardID, playerID string, score int64) error

	// ListLeaderboards lists available leaderboards for a game.
	ListLeaderboards(ctx context.Context, gameID string) ([]string, error)
}

// AchievementEngine handles achievement management.
type AchievementEngine interface {
	engine.ServiceModule

	// GetAchievements lists achievements for a game.
	GetAchievements(ctx context.Context, gameID string) ([]Achievement, error)

	// GetPlayerAchievements gets achievements unlocked by a player.
	GetPlayerAchievements(ctx context.Context, gameID, playerID string) ([]Achievement, error)

	// UnlockAchievement unlocks an achievement for a player.
	UnlockAchievement(ctx context.Context, gameID, playerID, achievementID string) error

	// GetAchievementProgress gets progress toward an achievement.
	GetAchievementProgress(ctx context.Context, gameID, playerID, achievementID string) (int, int, error)
}

// QuestEngine handles quest management.
type QuestEngine interface {
	engine.ServiceModule

	// GetQuests lists available quests.
	GetQuests(ctx context.Context, gameID string, questType QuestType) ([]Quest, error)

	// GetPlayerQuests gets quests assigned to a player.
	GetPlayerQuests(ctx context.Context, gameID, playerID string) ([]Quest, error)

	// AcceptQuest assigns a quest to a player.
	AcceptQuest(ctx context.Context, gameID, playerID, questID string) error

	// UpdateQuestProgress updates quest objective progress.
	UpdateQuestProgress(ctx context.Context, gameID, playerID, questID, objectiveID string, progress int) error

	// CompleteQuest marks a quest as complete and awards rewards.
	CompleteQuest(ctx context.Context, gameID, playerID, questID string) ([]Reward, error)
}

// TournamentEngine handles tournament management.
type TournamentEngine interface {
	engine.ServiceModule

	// CreateTournament creates a new tournament.
	CreateTournament(ctx context.Context, tournament *Tournament) (*Tournament, error)

	// GetTournament retrieves tournament information.
	GetTournament(ctx context.Context, tournamentID string) (*Tournament, error)

	// JoinTournament registers a player for a tournament.
	JoinTournament(ctx context.Context, tournamentID, playerID string) error

	// ListTournaments lists tournaments for a game.
	ListTournaments(ctx context.Context, gameID string, status TournamentStatus, limit int) ([]Tournament, error)

	// GetTournamentStandings gets current tournament standings.
	GetTournamentStandings(ctx context.Context, tournamentID string) ([]LeaderboardEntry, error)
}

// RewardEngine handles reward distribution.
type RewardEngine interface {
	engine.ServiceModule

	// DistributeReward distributes rewards to a player.
	DistributeReward(ctx context.Context, gameID, playerID string, rewards []Reward) error

	// ClaimRewards claims pending rewards.
	ClaimRewards(ctx context.Context, gameID, playerID string) ([]Reward, error)

	// GetPendingRewards gets pending rewards for a player.
	GetPendingRewards(ctx context.Context, gameID, playerID string) ([]Reward, error)
}

// GameFiEngine is the composite interface for GameFi services.
type GameFiEngine interface {
	engine.ServiceModule

	// GameFiInfo returns GameFi engine information.
	GameFiInfo() GameFiInfo
}

// GameFiInfo provides metadata about a GameFi engine implementation.
type GameFiInfo struct {
	SupportedGames []string `json:"supported_games"`
	Chains         []string `json:"chains"`
	Capabilities   []string `json:"capabilities"` // player, asset, match, leaderboard, achievement, quest, tournament
}

// Capability markers

// PlayerCapable indicates player management support.
type PlayerCapable interface {
	HasPlayerEngine() bool
	PlayerEngine() PlayerEngine
}

// AssetCapable indicates asset management support.
type AssetCapable interface {
	HasAssetEngine() bool
	AssetEngine() AssetEngine
}

// MatchCapable indicates match management support.
type MatchCapable interface {
	HasMatchEngine() bool
	MatchEngine() MatchEngine
}

// LeaderboardCapable indicates leaderboard support.
type LeaderboardCapable interface {
	HasLeaderboardEngine() bool
	LeaderboardEngine() LeaderboardEngine
}

// AchievementCapable indicates achievement support.
type AchievementCapable interface {
	HasAchievementEngine() bool
	AchievementEngine() AchievementEngine
}

// QuestCapable indicates quest support.
type QuestCapable interface {
	HasQuestEngine() bool
	QuestEngine() QuestEngine
}

// TournamentCapable indicates tournament support.
type TournamentCapable interface {
	HasTournamentEngine() bool
	TournamentEngine() TournamentEngine
}
