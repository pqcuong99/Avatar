package econ

import (
	"time"

	"example.com/project-thanh/transaction-service/internal/apperr"
	"example.com/project-thanh/transaction-service/internal/farm"
)

// Farm commands (docs/02 §3-5, P0-067..102). All of them run inside Execute.

type plotRow struct {
	no        int
	planting  string // id of the growing planting, "" when empty
	crop      string
	matures   time.Time
	baseYield int
	pest      *pestRow
}

type pestRow struct {
	spawn, deadline time.Time
	caught          *time.Time
	penalty         int
}

// plots loads every unlocked plot with its growing planting, ordered by plot
// number. The character row lock already serializes commands; the plot rows
// are locked as well so that no other writer can slip in.
func (w *Work) plots() ([]plotRow, error) {
	rows, err := w.tx.Query(w.ctx, `SELECT f.plot_no, COALESCE(p.id::text, ''), COALESCE(p.crop_code, ''),
			p.matures_at, COALESCE(p.base_yield_snapshot, 0), i.spawn_at, i.deadline_at, i.caught_at, i.penalty_percent
		FROM farm_plots f
		LEFT JOIN crop_plantings p ON p.character_id = f.character_id AND p.plot_no = f.plot_no AND p.state = 'growing'
		LEFT JOIN pest_incidents i ON i.planting_id = p.id
		WHERE f.character_id = $1 ORDER BY f.plot_no
		FOR UPDATE OF f`, w.CharacterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []plotRow
	for rows.Next() {
		var r plotRow
		var matures, spawn, deadline *time.Time
		var penalty *int16
		var caught *time.Time
		if err := rows.Scan(&r.no, &r.planting, &r.crop, &matures, &r.baseYield, &spawn, &deadline, &caught, &penalty); err != nil {
			return nil, err
		}
		if matures != nil {
			r.matures = *matures
		}
		if spawn != nil {
			r.pest = &pestRow{spawn: *spawn, deadline: *deadline, caught: caught, penalty: int(*penalty)}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func findPlot(plots []plotRow, no int) (plotRow, error) {
	for _, p := range plots {
		if p.no == no {
			return p, nil
		}
	}
	return plotRow{}, apperr.PlotLocked.With(map[string]any{"plot_no": no})
}

type PlantResult struct {
	PlotNo    int       `json:"plot_no"`
	CropCode  string    `json:"crop_code"`
	MaturesAt time.Time `json:"matures_at"`
}

// plantOn removes one seed and creates the planting with its snapshots and its
// (single, persisted) pest roll.
func (w *Work) plantOn(plotNo int, crop CropDef, cc CropConfig) (PlantResult, error) {
	if err := w.RemoveItem(crop.SeedItemCode, 1, "FARM_PLANT"); err != nil {
		return PlantResult{}, err
	}
	id := newUUID()
	matures := w.Now.Add(time.Duration(cc.GrowthSeconds) * time.Second)
	if err := w.Exec(`INSERT INTO crop_plantings (id, character_id, plot_no, crop_code, config_version_id,
			planted_at, matures_at, growth_seconds_snapshot, base_yield_snapshot, pest_roll_completed, state)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, true, 'growing')`,
		id, w.CharacterID, plotNo, crop.Code, w.Config.VersionID, w.Now, matures, cc.GrowthSeconds, cc.BaseYield); err != nil {
		return PlantResult{}, err
	}
	if plan := farm.RollPest(w.Rand(), w.Now, cc.GrowthSeconds, cc.PestProbability); plan != nil {
		if err := w.Exec(`INSERT INTO pest_incidents (planting_id, spawn_at, deadline_at, penalty_percent)
			VALUES ($1, $2, $3, $4)`, id, plan.SpawnAt, plan.DeadlineAt, plan.PenaltyPercent); err != nil {
			return PlantResult{}, err
		}
	}
	return PlantResult{PlotNo: plotNo, CropCode: crop.Code, MaturesAt: matures}, nil
}

func (w *Work) cropForPlanting(code string) (CropDef, CropConfig, error) {
	crop, ok := w.Catalog().Crops[code]
	if !ok {
		return CropDef{}, CropConfig{}, apperr.Validation.With(map[string]any{"field": "crop_code"})
	}
	cc, ok := w.Config.Crops[code]
	if !ok {
		return CropDef{}, CropConfig{}, apperr.NotConfigured.With(map[string]any{"crop_code": code})
	}
	return crop, cc, nil
}

// Plant plants one seed on one plot; no confirmation (P0-091).
func (w *Work) Plant(plotNo int, cropCode string) (any, error) {
	crop, cc, err := w.cropForPlanting(cropCode)
	if err != nil {
		return nil, err
	}
	plots, err := w.plots()
	if err != nil {
		return nil, err
	}
	p, err := findPlot(plots, plotNo)
	if err != nil {
		return nil, err
	}
	if p.planting != "" {
		return nil, apperr.PlotOccupied.With(map[string]any{"plot_no": plotNo})
	}
	r, err := w.plantOn(plotNo, crop, cc)
	if err != nil {
		return nil, err
	}
	return r, w.QuestEvent("plant_crop", 1)
}

type PlantAllResult struct {
	CropCode  string        `json:"crop_code"`
	SeedsUsed int           `json:"seeds_used"`
	Planted   []PlantResult `json:"planted"`
}

// PlantAll uses one seed type on empty plots in increasing plot number until
// the seeds or the empty plots run out (P0-089). It needs the confirmation of
// P0-090 and is one atomic batch.
func (w *Work) PlantAll(cropCode string, confirmed bool) (any, error) {
	if !confirmed {
		return nil, apperr.ConfirmationRequired
	}
	crop, cc, err := w.cropForPlanting(cropCode)
	if err != nil {
		return nil, err
	}
	bag, err := w.Bag()
	if err != nil {
		return nil, err
	}
	seeds := bag.Count(crop.SeedItemCode)
	if seeds == 0 {
		return nil, apperr.ItemInsufficient.With(map[string]any{"item_code": crop.SeedItemCode, "required": 1, "held": 0})
	}
	plots, err := w.plots()
	if err != nil {
		return nil, err
	}
	res := PlantAllResult{CropCode: cropCode}
	for _, p := range plots {
		if int64(res.SeedsUsed) == seeds {
			break
		}
		if p.planting != "" {
			continue
		}
		r, err := w.plantOn(p.no, crop, cc)
		if err != nil {
			return nil, err
		}
		res.Planted = append(res.Planted, r)
		res.SeedsUsed++
	}
	if res.SeedsUsed == 0 {
		return nil, apperr.PlotOccupied.With(map[string]any{"reason": "no_empty_plot"})
	}
	return res, w.QuestEvent("plant_crop", res.SeedsUsed)
}

type HarvestResult struct {
	PlotNo         int    `json:"plot_no"`
	CropCode       string `json:"crop_code"`
	ItemCode       string `json:"item_code"`
	Quantity       int    `json:"quantity"`
	Destination    string `json:"destination"` // BAG or PENDING
	PenaltyPercent int    `json:"penalty_percent,omitempty"`
}

// harvestPlot gives the whole lot of one mature planting to the bag when it
// fits, otherwise the whole lot to pending (P0-075/076): a lot is never split.
func (w *Work) harvestPlot(p plotRow) (HarvestResult, error) {
	crop := w.Catalog().Crops[p.crop]
	hadPest := p.pest != nil
	caught := hadPest && p.pest.caught != nil
	penalty := 0
	if hadPest {
		penalty = p.pest.penalty
	}
	qty := farm.FinalYield(p.baseYield, hadPest, caught, penalty)
	res := HarvestResult{PlotNo: p.no, CropCode: p.crop, ItemCode: crop.CropItemCode, Quantity: qty}
	if hadPest && !caught {
		res.PenaltyPercent = penalty
	}

	bag, err := w.Bag()
	if err != nil {
		return res, err
	}
	item := w.Catalog().Items[crop.CropItemCode]
	if bag.CanFit(item.inv(), int64(qty)) {
		if err := w.AddItem(crop.CropItemCode, int64(qty), "FARM_HARVEST"); err != nil {
			return res, err
		}
		res.Destination = "BAG"
	} else {
		after, err := w.addPendingCrop(p.crop, int64(qty))
		if err != nil {
			return res, err
		}
		w.LogItem(crop.CropItemCode, "PENDING", int64(qty), after, "FARM_HARVEST_TO_PENDING")
		res.Destination = "PENDING"
	}
	if err := w.Exec(`UPDATE crop_plantings SET state = 'harvested', completed_at = $2,
			harvest_quantity = $3, harvest_destination = $4 WHERE id = $1`,
		p.planting, w.Now, qty, res.Destination); err != nil {
		return res, err
	}
	return res, nil
}

// addPendingCrop merges a lot into the crop's single pending line, keeping the
// earliest timestamp for FIFO (P0-081/082). The line row is locked by the
// upsert, so two concurrent harvests cannot lose an update.
func (w *Work) addPendingCrop(cropCode string, qty int64) (int64, error) {
	var after int64
	err := w.tx.QueryRow(w.ctx, `INSERT INTO pending_crops (character_id, crop_code, quantity, earliest_created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4)
		ON CONFLICT (character_id, crop_code) DO UPDATE
		SET quantity = pending_crops.quantity + EXCLUDED.quantity,
		    earliest_created_at = LEAST(pending_crops.earliest_created_at, EXCLUDED.earliest_created_at),
		    updated_at = EXCLUDED.updated_at
		RETURNING quantity`, w.CharacterID, cropCode, qty, w.Now).Scan(&after)
	return after, err
}

// Harvest collects one mature plot; a planting is harvested once (P0-067).
func (w *Work) Harvest(plotNo int) (any, error) {
	plots, err := w.plots()
	if err != nil {
		return nil, err
	}
	p, err := findPlot(plots, plotNo)
	if err != nil {
		return nil, err
	}
	if p.planting == "" {
		return nil, apperr.PlotEmpty.With(map[string]any{"plot_no": plotNo})
	}
	if w.Now.Before(p.matures) {
		return nil, apperr.PlotNotReady.With(map[string]any{"plot_no": plotNo, "matures_at": p.matures.UTC()})
	}
	h, err := w.harvestPlot(p)
	if err != nil {
		return nil, err
	}
	return h, w.QuestEvent("harvest_crop", 1)
}

type HarvestAllResult struct {
	Harvests []HarvestResult `json:"harvests"`
}

// HarvestAll collects only mature plants (P0-087), in increasing plot order;
// a lot that does not fit goes to pending and the loop continues (P0-088).
// No confirmation (P0-092).
func (w *Work) HarvestAll() (any, error) {
	plots, err := w.plots()
	if err != nil {
		return nil, err
	}
	var res HarvestAllResult
	for _, p := range plots {
		if p.planting == "" || w.Now.Before(p.matures) {
			continue
		}
		h, err := w.harvestPlot(p)
		if err != nil {
			return nil, err
		}
		res.Harvests = append(res.Harvests, h)
	}
	if len(res.Harvests) == 0 {
		return nil, apperr.PlotNotReady.With(map[string]any{"reason": "nothing_mature"})
	}
	return res, w.QuestEvent("harvest_crop", len(res.Harvests))
}

type UprootResult struct {
	Uprooted []int `json:"uprooted"`
}

// Uproot removes a plant, mature or not, losing seed and yield with no refund
// (P0-073/074). Always confirmed (P0-086).
func (w *Work) Uproot(plotNo int, confirmed bool) (any, error) {
	if !confirmed {
		return nil, apperr.ConfirmationRequired
	}
	plots, err := w.plots()
	if err != nil {
		return nil, err
	}
	p, err := findPlot(plots, plotNo)
	if err != nil {
		return nil, err
	}
	if p.planting == "" {
		return nil, apperr.PlotEmpty.With(map[string]any{"plot_no": plotNo})
	}
	if err := w.Exec(`UPDATE crop_plantings SET state = 'uprooted', completed_at = $2 WHERE id = $1`, p.planting, w.Now); err != nil {
		return nil, err
	}
	return UprootResult{Uprooted: []int{plotNo}}, nil
}

// UprootAll removes every plant, young and mature, after confirmation (P0-093).
func (w *Work) UprootAll(confirmed bool) (any, error) {
	if !confirmed {
		return nil, apperr.ConfirmationRequired
	}
	plots, err := w.plots()
	if err != nil {
		return nil, err
	}
	res := UprootResult{Uprooted: []int{}}
	for _, p := range plots {
		if p.planting == "" {
			continue
		}
		if err := w.Exec(`UPDATE crop_plantings SET state = 'uprooted', completed_at = $2 WHERE id = $1`, p.planting, w.Now); err != nil {
			return nil, err
		}
		res.Uprooted = append(res.Uprooted, p.no)
	}
	if len(res.Uprooted) == 0 {
		return nil, apperr.PlotEmpty.With(map[string]any{"reason": "nothing_planted"})
	}
	return res, nil
}

type CatchPestResult struct {
	PlotNo   int    `json:"plot_no"`
	ItemCode string `json:"item_code"`
	Quantity int    `json:"quantity"`
}

// CatchPest catches the plot's pest for free before its deadline and gives one
// worm (P0-097, P0-104).
//
// Bag full: the rule is not decided (GAP-06, test F-17). Until it is, the catch
// is refused with INVENTORY_CAPACITY_INSUFFICIENT and the pest stays: nothing
// is deleted and no worm is invented. [ĐỀ XUẤT tạm, cần chủ dự án xác nhận]
func (w *Work) CatchPest(plotNo int) (any, error) {
	plots, err := w.plots()
	if err != nil {
		return nil, err
	}
	p, err := findPlot(plots, plotNo)
	if err != nil {
		return nil, err
	}
	if p.planting == "" || p.pest == nil || p.pest.caught != nil ||
		w.Now.Before(p.pest.spawn) || !w.Now.Before(p.pest.deadline) {
		return nil, apperr.PestNotActive.With(map[string]any{"plot_no": plotNo})
	}
	if err := w.AddItem("worm", 1, "FARM_CATCH_PEST"); err != nil {
		return nil, err
	}
	if err := w.Exec(`UPDATE pest_incidents SET caught_at = $2 WHERE planting_id = $1`, p.planting, w.Now); err != nil {
		return nil, err
	}
	return CatchPestResult{PlotNo: plotNo, ItemCode: "worm", Quantity: 1}, w.QuestEvent("catch_pest", 1)
}

type UnlockPlotResult struct {
	PlotNo int    `json:"plot_no"`
	Price  int64  `json:"price"`
	Cur    string `json:"currency"`
}

// UnlockPlot opens the next plot number (P0-069..071). Price and limit are
// Admin config (GAP-05); without them the feature is disabled.
func (w *Work) UnlockPlot() (any, error) {
	u := w.Config.PlotUnlock
	if u == nil || u.Price <= 0 || u.MaxPlots <= 0 {
		return nil, apperr.NotConfigured.With(map[string]any{"config": "farm.plot_unlock"})
	}
	plots, err := w.plots()
	if err != nil {
		return nil, err
	}
	if len(plots) >= u.MaxPlots {
		return nil, apperr.LimitReached.With(map[string]any{"max_plots": u.MaxPlots})
	}
	next := 1
	if len(plots) > 0 {
		next = plots[len(plots)-1].no + 1
	}
	if err := w.Debit(u.Currency, u.Price, "FARM_PLOT_UNLOCK"); err != nil {
		return nil, err
	}
	if err := w.Exec(`INSERT INTO farm_plots (character_id, plot_no, unlocked_at) VALUES ($1, $2, $3)`, w.CharacterID, next, w.Now); err != nil {
		return nil, err
	}
	return UnlockPlotResult{PlotNo: next, Price: u.Price, Cur: u.Currency}, nil
}

type UpgradeInventoryResult struct {
	SlotCap int    `json:"slot_cap"`
	Price   int64  `json:"price"`
	Cur     string `json:"currency"`
}

// UpgradeInventory adds 5 bag slots (TH-02). Price and the maximum are Admin
// config (GAP-05); without them the feature is disabled.
func (w *Work) UpgradeInventory() (any, error) {
	u := w.Config.InventoryUpgrade
	if u == nil || u.Price <= 0 || u.MaxSlotCap <= 0 || u.SlotsPerUpgrade <= 0 {
		return nil, apperr.NotConfigured.With(map[string]any{"config": "inventory.upgrade"})
	}
	next := w.SlotCap() + u.SlotsPerUpgrade
	if next > u.MaxSlotCap {
		return nil, apperr.LimitReached.With(map[string]any{"max_slot_cap": u.MaxSlotCap})
	}
	if err := w.Debit(u.Currency, u.Price, "INVENTORY_UPGRADE"); err != nil {
		return nil, err
	}
	if err := w.Exec(`UPDATE characters SET inventory_slot_cap = $2, updated_at = $3 WHERE id = $1`, w.CharacterID, next, w.Now); err != nil {
		return nil, err
	}
	w.SetSlotCap(next)
	return UpgradeInventoryResult{SlotCap: next, Price: u.Price, Cur: u.Currency}, nil
}
