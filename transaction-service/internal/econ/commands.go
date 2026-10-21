package econ

import (
	"bytes"
	"context"
	"encoding/json"

	"example.com/avatar/transaction-service/internal/apperr"
)

// command is one asset operation. Its JSON form (without request_id) is the
// idempotency digest input, so the field order is fixed by these structs.
type command interface {
	exec(w *Work) (any, error)
}

// commands is the closed list of operations the service accepts.
var commands = map[string]func() command{
	"farm.plant":                func() command { return &plantCmd{} },
	"farm.plant_all":            func() command { return &plantAllCmd{} },
	"farm.harvest":              func() command { return &plotCmd{fn: (*Work).Harvest} },
	"farm.harvest_all":          func() command { return &noArgCmd{fn: (*Work).HarvestAll} },
	"farm.uproot":               func() command { return &confirmPlotCmd{fn: (*Work).Uproot} },
	"farm.uproot_all":           func() command { return &confirmCmd{fn: (*Work).UprootAll} },
	"farm.catch_pest":           func() command { return &plotCmd{fn: (*Work).CatchPest} },
	"farm.unlock_plot":          func() command { return &noArgCmd{fn: (*Work).UnlockPlot} },
	"inventory.upgrade":         func() command { return &noArgCmd{fn: (*Work).UpgradeInventory} },
	"pending.crop.claim":        func() command { return &cropCmd{} },
	"pending.crop.claim_all":    func() command { return &noArgCmd{fn: (*Work).ClaimAllPendingCrops} },
	"pending.diamond.claim":     func() command { return &diamondCmd{} },
	"pending.diamond.claim_all": func() command { return &noArgCmd{fn: (*Work).ClaimAllPendingDiamonds} },
	"npc.buy":                   func() command { return &tradeCmd{buy: true} },
	"npc.sell":                  func() command { return &tradeCmd{} },
	"fashion.equip":             func() command { return &equipCmd{} },
	"fashion.unequip":           func() command { return &unequipCmd{} },
	"quests.claim":              func() command { return &questCmd{} },
	"character.rename":          func() command { return &renameCmd{} },
	// Internal: progress for events counted outside this service.
	"quest.event": func() command { return &questEventCmd{} },
	// Internal: an approved two-person Admin adjustment.
	"admin.adjust": func() command { return &adminAdjustCmd{} },
}

type equipCmd struct {
	InstanceID string `json:"instance_id"`
}

func (c *equipCmd) exec(w *Work) (any, error) { return w.Equip(c.InstanceID) }

type unequipCmd struct {
	EquipSlot string `json:"equip_slot"`
}

func (c *unequipCmd) exec(w *Work) (any, error) { return w.Unequip(c.EquipSlot) }

type questCmd struct {
	QuestCode string `json:"quest_code"`
}

func (c *questCmd) exec(w *Work) (any, error) { return w.ClaimQuest(c.QuestCode) }

type questEventCmd struct {
	EventType string `json:"event_type"`
	Amount    int    `json:"amount"`
}

func (c *questEventCmd) exec(w *Work) (any, error) {
	return w.ExternalQuestEvent(c.EventType, c.Amount)
}

type plantCmd struct {
	PlotNo   int    `json:"plot_no"`
	CropCode string `json:"crop_code"`
}

func (c *plantCmd) exec(w *Work) (any, error) { return w.Plant(c.PlotNo, c.CropCode) }

type plantAllCmd struct {
	CropCode  string `json:"crop_code"`
	Confirmed bool   `json:"confirmed"`
}

func (c *plantAllCmd) exec(w *Work) (any, error) { return w.PlantAll(c.CropCode, c.Confirmed) }

type plotCmd struct {
	PlotNo int `json:"plot_no"`
	fn     func(*Work, int) (any, error)
}

func (c *plotCmd) exec(w *Work) (any, error) { return c.fn(w, c.PlotNo) }

type confirmPlotCmd struct {
	PlotNo    int  `json:"plot_no"`
	Confirmed bool `json:"confirmed"`
	fn        func(*Work, int, bool) (any, error)
}

func (c *confirmPlotCmd) exec(w *Work) (any, error) { return c.fn(w, c.PlotNo, c.Confirmed) }

type confirmCmd struct {
	Confirmed bool `json:"confirmed"`
	fn        func(*Work, bool) (any, error)
}

func (c *confirmCmd) exec(w *Work) (any, error) { return c.fn(w, c.Confirmed) }

type noArgCmd struct {
	fn func(*Work) (any, error)
}

func (c *noArgCmd) exec(w *Work) (any, error) { return c.fn(w) }

type cropCmd struct {
	CropCode string `json:"crop_code"`
}

func (c *cropCmd) exec(w *Work) (any, error) { return w.ClaimPendingCrop(c.CropCode) }

type diamondCmd struct {
	AwardID string `json:"award_id"`
}

func (c *diamondCmd) exec(w *Work) (any, error) { return w.ClaimPendingDiamond(c.AwardID) }

type tradeCmd struct {
	NPCCode       string `json:"npc_code"`
	OfferID       string `json:"offer_id"`
	Quantity      int64  `json:"quantity"`
	ConfigVersion int64  `json:"config_version"`
	buy           bool
}

func (c *tradeCmd) exec(w *Work) (any, error) {
	if c.buy {
		return w.Buy(c.NPCCode, c.OfferID, c.Quantity, c.ConfigVersion)
	}
	return w.Sell(c.NPCCode, c.OfferID, c.Quantity, c.ConfigVersion)
}

// IsCommand reports whether op is a known operation.
func IsCommand(op string) bool {
	_, ok := commands[op]
	return ok
}

// Command decodes args strictly and runs the operation through Execute.
func (s *Service) Command(ctx context.Context, characterID, requestID, op string, args json.RawMessage) (*Outcome, error) {
	newCmd, ok := commands[op]
	if !ok {
		return nil, apperr.NotFound.With(map[string]any{"op": op})
	}
	cmd := newCmd()
	if len(bytes.TrimSpace(args)) > 0 {
		dec := json.NewDecoder(bytes.NewReader(args))
		dec.DisallowUnknownFields()
		if err := dec.Decode(cmd); err != nil {
			return nil, apperr.Validation.With(map[string]any{"reason": "invalid_arguments"})
		}
	}
	return s.Execute(ctx, characterID, requestID, op, cmd, cmd.exec)
}
