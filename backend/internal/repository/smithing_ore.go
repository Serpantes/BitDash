package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Smithing ore planner data: the ingot recipe chains (static, built once) and the per-player
// numbers that scale XP (hammer power, smithing power, experience rate, crafting speed).
// Both are kept in memory and on disk so the dashboard tab renders without waiting on BitJita.

const (
	smithingSkillID      = 6
	smithingChainsFile   = "smithing-ore-chains.json"
	smithingChainsFormat = 1

	statCraftingSpeed  = 15
	statSmithingSpeed  = 25
	statSmithingPower  = 38
	statExperienceRate = 50
)

var smithingIngotTiers = []struct {
	Tier    int
	IngotID string
}{
	{1, "1050001"}, {2, "2050001"}, {3, "3050001"}, {4, "4050001"}, {5, "5050001"},
	{6, "6050001"}, {7, "1899017490"}, {8, "1464752960"}, {9, "445742898"},
}

type IngotChain struct {
	Tier                 int     `json:"tier"`
	IngotName            string  `json:"ingotName"`
	IngotIcon            string  `json:"ingotIcon,omitempty"`
	PieceName            string  `json:"pieceName"`
	ChunkName            string  `json:"chunkName"`
	LevelRequired        int     `json:"levelRequired"`
	ToolLevelRequired    int     `json:"toolLevelRequired"`
	XPPerProgress        float64 `json:"xpPerProgress"`
	SwingTime            float64 `json:"swingTime"`
	ForgeEffort          int     `json:"forgeEffort"`
	ConcentrateEffort    int     `json:"concentrateEffort"`
	ConcentratesPerIngot int     `json:"concentratesPerIngot"`
	PiecesPerConcentrate int     `json:"piecesPerConcentrate"`
	PiecesPerChunk       int     `json:"piecesPerChunk"`
	FluxPerIngot         int     `json:"fluxPerIngot"`
}

type SmithingTool struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Power int    `json:"power"`
	Level int    `json:"level"`
	Icon  string `json:"icon,omitempty"`
}

type SmithingSetup struct {
	Tool             *SmithingTool `json:"tool"`
	SkillPower       float64       `json:"skillPower"`
	ProgressPerSwing float64       `json:"progressPerSwing"`
	XPRate           float64       `json:"xpRate"`
	Speed            float64       `json:"speed"`
	FetchedAt        int64         `json:"fetchedAt"`
}

type SmithingOrePayload struct {
	Chains []IngotChain   `json:"chains"`
	Setup  *SmithingSetup `json:"setup"`
}

type recipeStack struct {
	ID       flexibleID `json:"id"`
	Name     string     `json:"name"`
	Quantity int        `json:"quantity"`
}

type itemRecipe struct {
	Name              string  `json:"name"`
	ActionsRequired   int     `json:"actionsRequired"`
	TimeRequirement   float64 `json:"timeRequirement"`
	LevelRequirements []struct {
		Level int `json:"level"`
	} `json:"levelRequirements"`
	ToolRequirements []struct {
		Level int `json:"level"`
	} `json:"toolRequirements"`
	ExperiencePerProgress []struct {
		Quantity float64 `json:"quantity"`
		SkillID  int     `json:"skill_id"`
	} `json:"experiencePerProgress"`
	ConsumedItems []recipeStack `json:"consumedItems"`
	CraftedItems  []recipeStack `json:"craftedItems"`
}

type itemDetails struct {
	Item struct {
		Name          string `json:"name"`
		IconAssetName string `json:"iconAssetName"`
	} `json:"item"`
	CraftingRecipes []itemRecipe `json:"craftingRecipes"`
}

type smithingSetupEntry struct {
	setup      *SmithingSetup
	refreshing bool
}

type SmithingOre struct {
	bitjita  *BitjitaCache
	setupTTL time.Duration

	chainsMu sync.Mutex
	chains   []IngotChain

	setupMu sync.Mutex
	setups  map[string]*smithingSetupEntry
}

func NewSmithingOre(bitjita *BitjitaCache, setupTTL time.Duration) *SmithingOre {
	return &SmithingOre{bitjita: bitjita, setupTTL: setupTTL, setups: map[string]*smithingSetupEntry{}}
}

// Warm builds the chains ahead of the first request.
func (s *SmithingOre) Warm() {
	if _, err := s.Chains(); err != nil {
		log.Printf("smithing ore chains: %v", err)
	}
}

func (s *SmithingOre) Get(playerID string) (*SmithingOrePayload, error) {
	chains, err := s.Chains()
	if err != nil {
		return nil, err
	}
	setup, err := s.Setup(playerID)
	if err != nil {
		return nil, err
	}
	return &SmithingOrePayload{Chains: chains, Setup: setup}, nil
}

func (s *SmithingOre) Chains() ([]IngotChain, error) {
	s.chainsMu.Lock()
	defer s.chainsMu.Unlock()
	if s.chains != nil {
		return s.chains, nil
	}
	if cached := s.loadChainsFromDisk(); cached != nil {
		s.chains = cached
		return cached, nil
	}
	chains := make([]IngotChain, 0, len(smithingIngotTiers))
	for _, tier := range smithingIngotTiers {
		chain, err := s.buildChain(tier.Tier, tier.IngotID)
		if err != nil {
			return nil, fmt.Errorf("tier %d: %w", tier.Tier, err)
		}
		chains = append(chains, *chain)
	}
	if body, err := json.Marshal(struct {
		Format int          `json:"format"`
		Chains []IngotChain `json:"chains"`
	}{smithingChainsFormat, chains}); err == nil {
		_ = s.bitjita.apiFiles.Set(smithingChainsFile, body)
	}
	s.chains = chains
	return chains, nil
}

func (s *SmithingOre) loadChainsFromDisk() []IngotChain {
	body, ok := s.bitjita.apiFiles.Get(smithingChainsFile)
	if !ok {
		return nil
	}
	var stored struct {
		Format int          `json:"format"`
		Chains []IngotChain `json:"chains"`
	}
	if err := json.Unmarshal(body, &stored); err != nil || stored.Format != smithingChainsFormat || len(stored.Chains) != len(smithingIngotTiers) {
		return nil
	}
	return stored.Chains
}

func (s *SmithingOre) item(id string) (*itemDetails, error) {
	body, status, _, err := s.bitjita.GetAPI("/items/"+id, "")
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("item %s: upstream status %d", id, status)
	}
	var details itemDetails
	if err := json.Unmarshal(body, &details); err != nil {
		return nil, fmt.Errorf("item %s: %w", id, err)
	}
	return &details, nil
}

func findRecipe(details *itemDetails, outputID, prefix string) *itemRecipe {
	for i := range details.CraftingRecipes {
		r := &details.CraftingRecipes[i]
		if !strings.HasPrefix(r.Name, prefix) {
			continue
		}
		for _, out := range r.CraftedItems {
			if string(out.ID) == outputID {
				return r
			}
		}
	}
	return nil
}

func findConsumed(r *itemRecipe, match func(name string) bool) *recipeStack {
	for i := range r.ConsumedItems {
		if match(r.ConsumedItems[i].Name) {
			return &r.ConsumedItems[i]
		}
	}
	return nil
}

// buildChain walks ingot ← molten ← concentrate ← ore piece ← ore chunk.
func (s *SmithingOre) buildChain(tier int, ingotID string) (*IngotChain, error) {
	ingot, err := s.item(ingotID)
	if err != nil {
		return nil, err
	}
	forge := findRecipe(ingot, ingotID, "Forge")
	if forge == nil {
		return nil, errors.New("no forge recipe")
	}
	molten := findConsumed(forge, func(n string) bool { return strings.HasPrefix(n, "Molten") })
	if molten == nil {
		return nil, errors.New("forge recipe has no molten metal")
	}
	moltenItem, err := s.item(string(molten.ID))
	if err != nil {
		return nil, err
	}
	smelt := findRecipe(moltenItem, string(molten.ID), "Smelt")
	if smelt == nil {
		return nil, errors.New("no smelt recipe")
	}
	concentrate := findConsumed(smelt, func(n string) bool { return strings.Contains(n, "Concentrate") })
	if concentrate == nil {
		return nil, errors.New("smelt recipe has no concentrate")
	}
	concentrateItem, err := s.item(string(concentrate.ID))
	if err != nil {
		return nil, err
	}
	extract := findRecipe(concentrateItem, string(concentrate.ID), "Extract")
	if extract == nil || len(extract.ConsumedItems) == 0 {
		return nil, errors.New("no concentrate recipe")
	}
	piece := extract.ConsumedItems[0]
	pieceItem, err := s.item(string(piece.ID))
	if err != nil {
		return nil, err
	}

	chain := &IngotChain{
		Tier:                 tier,
		IngotName:            ingot.Item.Name,
		IngotIcon:            ingot.Item.IconAssetName,
		PieceName:            piece.Name,
		LevelRequired:        1,
		SwingTime:            forge.TimeRequirement,
		ForgeEffort:          forge.ActionsRequired,
		ConcentrateEffort:    extract.ActionsRequired,
		ConcentratesPerIngot: concentrate.Quantity * molten.Quantity,
		PiecesPerConcentrate: piece.Quantity,
	}
	if len(forge.LevelRequirements) > 0 {
		chain.LevelRequired = forge.LevelRequirements[0].Level
	}
	if len(forge.ToolRequirements) > 0 {
		chain.ToolLevelRequired = forge.ToolRequirements[0].Level
	}
	for _, e := range forge.ExperiencePerProgress {
		if e.SkillID == smithingSkillID {
			chain.XPPerProgress = e.Quantity
		}
	}
	if flux := findConsumed(forge, func(n string) bool { return strings.Contains(n, "Flux") }); flux != nil {
		chain.FluxPerIngot = flux.Quantity
	}
	if split := findRecipe(pieceItem, string(piece.ID), "Extract"); split != nil {
		if len(split.ConsumedItems) > 0 {
			chain.ChunkName = split.ConsumedItems[0].Name
		}
		for _, out := range split.CraftedItems {
			if out.ID == piece.ID {
				chain.PiecesPerChunk = out.Quantity
			}
		}
	}
	return chain, nil
}

// Setup returns the cached numbers at once and refreshes them in the background when stale;
// only a player seen for the first time waits on BitJita.
func (s *SmithingOre) Setup(playerID string) (*SmithingSetup, error) {
	s.setupMu.Lock()
	entry := s.setups[playerID]
	if entry == nil {
		entry = &smithingSetupEntry{setup: s.loadSetupFromDisk(playerID)}
		s.setups[playerID] = entry
	}
	if entry.setup == nil {
		s.setupMu.Unlock()
		setup, err := s.fetchSetup(playerID)
		if err != nil {
			return nil, err
		}
		s.storeSetup(playerID, setup)
		return setup, nil
	}
	setup := entry.setup
	if time.Since(time.UnixMilli(setup.FetchedAt)) > s.setupTTL && !entry.refreshing {
		entry.refreshing = true
		go s.refreshSetup(playerID)
	}
	s.setupMu.Unlock()
	return setup, nil
}

func (s *SmithingOre) refreshSetup(playerID string) {
	setup, err := s.fetchSetup(playerID)
	if err != nil {
		log.Printf("smithing setup %s: %v", playerID, err)
		s.setupMu.Lock()
		s.setups[playerID].refreshing = false
		s.setupMu.Unlock()
		return
	}
	s.storeSetup(playerID, setup)
}

func (s *SmithingOre) storeSetup(playerID string, setup *SmithingSetup) {
	s.setupMu.Lock()
	s.setups[playerID] = &smithingSetupEntry{setup: setup}
	s.setupMu.Unlock()
	if body, err := json.Marshal(setup); err == nil {
		_ = s.bitjita.apiFiles.Set(smithingSetupFile(playerID), body)
	}
}

func smithingSetupFile(playerID string) string {
	return "smithing-setup-" + playerID + ".json"
}

func (s *SmithingOre) loadSetupFromDisk(playerID string) *SmithingSetup {
	body, ok := s.bitjita.apiFiles.Get(smithingSetupFile(playerID))
	if !ok {
		return nil
	}
	var setup SmithingSetup
	if err := json.Unmarshal(body, &setup); err != nil || setup.FetchedAt == 0 {
		return nil
	}
	return &setup
}

func (s *SmithingOre) fetchSetup(playerID string) (*SmithingSetup, error) {
	statsBody, status, _, err := s.bitjita.GetAPI("/players/"+playerID+"/stats", "")
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("stats: upstream status %d", status)
	}
	invBody, status, _, err := s.bitjita.GetAPI("/players/"+playerID+"/inventories", "")
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("inventories: upstream status %d", status)
	}
	return buildSmithingSetup(statsBody, invBody, time.Now())
}

func buildSmithingSetup(statsBody, invBody []byte, now time.Time) (*SmithingSetup, error) {
	var stats struct {
		Stats struct {
			Values []float64 `json:"values"`
		} `json:"stats"`
	}
	if err := json.Unmarshal(statsBody, &stats); err != nil {
		return nil, fmt.Errorf("stats: %w", err)
	}
	var inv struct {
		Inventories []struct {
			InventoryName string `json:"inventoryName"`
			Pockets       []struct {
				Contents *struct {
					ItemID flexibleID `json:"itemId"`
				} `json:"contents"`
			} `json:"pockets"`
		} `json:"inventories"`
		Items map[string]struct {
			Name        string `json:"name"`
			Icon        string `json:"iconAssetName"`
			ToolPower   int    `json:"toolPower"`
			ToolLevel   int    `json:"toolLevel"`
			ToolSkillID int    `json:"toolSkillId"`
		} `json:"items"`
	}
	if err := json.Unmarshal(invBody, &inv); err != nil {
		return nil, fmt.Errorf("inventories: %w", err)
	}

	var tool *SmithingTool
	for _, container := range inv.Inventories {
		if container.InventoryName != "Toolbelt" {
			continue
		}
		for _, pocket := range container.Pockets {
			if pocket.Contents == nil {
				continue
			}
			id := string(pocket.Contents.ItemID)
			meta, ok := inv.Items[id]
			if !ok || meta.ToolSkillID != smithingSkillID {
				continue
			}
			if tool == nil || meta.ToolPower > tool.Power {
				tool = &SmithingTool{ID: id, Name: meta.Name, Power: meta.ToolPower, Level: meta.ToolLevel, Icon: meta.Icon}
			}
		}
	}

	values := stats.Stats.Values
	stat := func(index int, fallback float64) float64 {
		if index < len(values) && values[index] > 0 {
			return values[index]
		}
		return fallback
	}
	setup := &SmithingSetup{
		Tool:       tool,
		SkillPower: stat(statSmithingPower, 0),
		XPRate:     stat(statExperienceRate, 100) / 100,
		Speed:      stat(statCraftingSpeed, 1) * stat(statSmithingSpeed, 1),
		FetchedAt:  now.UnixMilli(),
	}
	if tool != nil {
		setup.ProgressPerSwing = float64(tool.Power) + setup.SkillPower
	}
	return setup, nil
}
