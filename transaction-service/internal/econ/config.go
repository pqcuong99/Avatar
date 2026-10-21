package econ

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/avatar/transaction-service/internal/inventory"
)

// DefaultXuCap is P0-035, used while no published config overrides it.
const DefaultXuCap int64 = 999_999_999

// Starting values of a new character (P0-030, P0-031, P0-068, TH-02).
const (
	StartingXu      int64 = 5000
	StartingDiamond int64 = 0
	StartingSlots         = 10
	StartingPlots         = 6
)

// ItemDef is catalog metadata of one item code.
type ItemDef struct {
	Code        string `json:"item_code"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"`
	StackLimit  int    `json:"stack_limit"`
	TradePolicy string `json:"trade_policy"`
}

func (d ItemDef) inv() inventory.Item {
	return inventory.Item{Code: d.Code, StackLimit: d.StackLimit, Instance: d.Category == "fashion"}
}

type CropDef struct {
	Code         string `json:"crop_code"`
	DisplayName  string `json:"display_name"`
	SeedItemCode string `json:"seed_item_code"`
	CropItemCode string `json:"crop_item_code"`
	SortOrder    int    `json:"sort_order"`
}

// Catalog is the static part of the item and crop catalog. It only changes
// through migrations, so it is loaded once at start.
type Catalog struct {
	Items     map[string]ItemDef
	Crops     map[string]CropDef
	CropOrder []string
	NPCs      map[string]NPCDef
	Fashion   map[string]FashionDef
}

type NPCDef struct {
	Code        string   `json:"npc_code"`
	DisplayName string   `json:"display_name"`
	Functions   []string `json:"functions"`
}

func loadCatalog(ctx context.Context, pool *pgxpool.Pool) (*Catalog, error) {
	c := &Catalog{Items: map[string]ItemDef{}, Crops: map[string]CropDef{}, NPCs: map[string]NPCDef{}}
	rows, err := pool.Query(ctx, `SELECT item_code, display_name, category, stack_limit, trade_policy FROM item_definitions`)
	if err != nil {
		return nil, err
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByPos[ItemDef])
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		c.Items[it.Code] = it
	}
	rows, err = pool.Query(ctx, `SELECT crop_code, display_name, seed_item_code, crop_item_code, sort_order FROM crop_definitions ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	crops, err := pgx.CollectRows(rows, pgx.RowToStructByPos[CropDef])
	if err != nil {
		return nil, err
	}
	for _, cr := range crops {
		c.Crops[cr.Code] = cr
		c.CropOrder = append(c.CropOrder, cr.Code)
	}
	if err := c.ReloadNPCs(ctx, pool); err != nil {
		return nil, err
	}
	if c.Fashion, err = loadFashion(ctx, pool); err != nil {
		return nil, err
	}
	return c, nil
}

// ReloadNPCs refreshes NPC names (they are created by Admin tooling or seeds).
func (c *Catalog) ReloadNPCs(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := pool.Query(ctx, `SELECT npc_code, display_name, functions FROM npc_definitions`)
	if err != nil {
		return err
	}
	npcs, err := pgx.CollectRows(rows, pgx.RowToStructByPos[NPCDef])
	if err != nil {
		return err
	}
	m := map[string]NPCDef{}
	for _, n := range npcs {
		m[n.Code] = n
	}
	c.NPCs = m
	return nil
}

// CropByItem finds the crop whose harvest item is code.
func (c *Catalog) CropByItem(code string) (CropDef, bool) {
	for _, cr := range c.Crops {
		if cr.CropItemCode == code {
			return cr, true
		}
	}
	return CropDef{}, false
}

// CropConfig is the per-version, Admin-configured part of a crop.
type CropConfig struct {
	GrowthSeconds   int     `json:"growth_seconds"`
	BaseYield       int     `json:"base_yield"`
	PestProbability float64 `json:"pest_probability"`
}

type InventoryUpgrade struct {
	Currency        string `json:"currency"`
	Price           int64  `json:"price"`
	SlotsPerUpgrade int    `json:"slots_per_upgrade"`
	MaxSlotCap      int    `json:"max_slot_cap"`
}

type PlotUnlock struct {
	Currency string `json:"currency"`
	Price    int64  `json:"price"`
	MaxPlots int    `json:"max_plots"`
}

// Config is the published configuration version. Features whose values are
// absent stay disabled (FEATURE_NOT_CONFIGURED); nothing falls back to an
// invented number.
type Config struct {
	VersionID        int64
	Label            string
	XuCap            int64
	DiamondCap       int64 // 0 = not configured
	InventoryUpgrade *InventoryUpgrade
	PlotUnlock       *PlotUnlock
	Crops            map[string]CropConfig
	Values           map[string]json.RawMessage
	// Daily quests; QuestLocation nil means quests are not configured.
	QuestLocation *time.Location
	Quests        []QuestDef
}

// MaxPossibleSlots is "the largest bag a player can ever have" used by the
// P0-083 pending exception. 0 means unknown (GAP-07).
func (c *Config) MaxPossibleSlots() int {
	if c.InventoryUpgrade == nil {
		return 0
	}
	return c.InventoryUpgrade.MaxSlotCap
}

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func loadPublishedConfig(ctx context.Context, q querier) (*Config, error) {
	cfg := &Config{XuCap: DefaultXuCap, Crops: map[string]CropConfig{}, Values: map[string]json.RawMessage{}}
	err := q.QueryRow(ctx, `SELECT id, label FROM game_config_versions WHERE state = 'published'`).Scan(&cfg.VersionID, &cfg.Label)
	if errors.Is(err, pgx.ErrNoRows) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, `SELECT key, value FROM game_config_values WHERE config_version_id = $1`, cfg.VersionID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var key string
		var raw json.RawMessage
		if err := rows.Scan(&key, &raw); err != nil {
			rows.Close()
			return nil, err
		}
		cfg.Values[key] = raw
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := decodeOptional(cfg.Values, "wallet.xu_cap", &cfg.XuCap); err != nil {
		return nil, err
	}
	if err := decodeOptional(cfg.Values, "wallet.diamond_cap", &cfg.DiamondCap); err != nil {
		return nil, err
	}
	if raw, ok := cfg.Values["inventory.upgrade"]; ok {
		cfg.InventoryUpgrade = &InventoryUpgrade{}
		if err := json.Unmarshal(raw, cfg.InventoryUpgrade); err != nil {
			return nil, fmt.Errorf("inventory.upgrade: %w", err)
		}
	}
	if raw, ok := cfg.Values["farm.plot_unlock"]; ok {
		cfg.PlotUnlock = &PlotUnlock{}
		if err := json.Unmarshal(raw, cfg.PlotUnlock); err != nil {
			return nil, fmt.Errorf("farm.plot_unlock: %w", err)
		}
	}
	rows, err = q.Query(ctx, `SELECT crop_code, growth_seconds, base_yield, pest_probability::float8
		FROM crop_configs WHERE config_version_id = $1`, cfg.VersionID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var code string
		var cc CropConfig
		if err := rows.Scan(&code, &cc.GrowthSeconds, &cc.BaseYield, &cc.PestProbability); err != nil {
			rows.Close()
			return nil, err
		}
		cfg.Crops[code] = cc
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := loadQuests(ctx, q, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func decodeOptional(values map[string]json.RawMessage, key string, dst any) error {
	raw, ok := values[key]
	if !ok {
		return nil
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	return nil
}

// PublicCatalog is what clients need to render items and the farm.
type PublicCatalog struct {
	ConfigVersion    int64             `json:"config_version"`
	ConfigLabel      string            `json:"config_label"`
	Items            []ItemDef         `json:"items"`
	Crops            []PublicCrop      `json:"crops"`
	InventoryUpgrade *InventoryUpgrade `json:"inventory_upgrade"`
	PlotUnlock       *PlotUnlock       `json:"plot_unlock"`
	XuCap            int64             `json:"xu_cap"`
	DiamondCap       *int64            `json:"diamond_cap"`
	Fashion          []FashionDef      `json:"fashion"`
}

type PublicCrop struct {
	CropDef
	Configured bool        `json:"configured"`
	Config     *CropConfig `json:"config,omitempty"`
}

func (s *Service) PublicCatalog(ctx context.Context) (*PublicCatalog, error) {
	cfg, err := loadPublishedConfig(ctx, s.pool)
	if err != nil {
		return nil, err
	}
	out := &PublicCatalog{
		ConfigVersion:    cfg.VersionID,
		ConfigLabel:      cfg.Label,
		InventoryUpgrade: cfg.InventoryUpgrade,
		PlotUnlock:       cfg.PlotUnlock,
		XuCap:            cfg.XuCap,
	}
	if cfg.DiamondCap > 0 {
		out.DiamondCap = &cfg.DiamondCap
	}
	for _, it := range s.catalog.Items {
		out.Items = append(out.Items, it)
	}
	sort.Slice(out.Items, func(i, j int) bool { return out.Items[i].Code < out.Items[j].Code })
	for _, f := range s.catalog.Fashion {
		out.Fashion = append(out.Fashion, f)
	}
	sort.Slice(out.Fashion, func(i, j int) bool { return out.Fashion[i].ItemCode < out.Fashion[j].ItemCode })
	for _, code := range s.catalog.CropOrder {
		pc := PublicCrop{CropDef: s.catalog.Crops[code]}
		if cc, ok := cfg.Crops[code]; ok {
			c := cc
			pc.Configured = true
			pc.Config = &c
		}
		out.Crops = append(out.Crops, pc)
	}
	return out, nil
}
