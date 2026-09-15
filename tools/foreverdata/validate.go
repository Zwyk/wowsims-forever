package main

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

// validateKnownRecords deliberately validates only the publisher's stable
// envelope and record shapes. Unknown fields remain in the generic document and
// therefore in every hash. A known field changing shape blocks an update for
// human review instead of being silently discarded.
func validateKnownRecords(document map[string]any) error {
	if err := validateTalents(document["talents"].(map[string]any)); err != nil {
		return err
	}
	if err := validateSpellbooks(document["spellbooks"].(map[string]any)); err != nil {
		return err
	}
	if err := validateSpellDescriptions(document["spell_desc"].(map[string]any)); err != nil {
		return err
	}
	if err := validateRacials(document["racials"].(map[string]any)); err != nil {
		return err
	}
	if err := validateClassRacials(document["class_racials"].(map[string]any)); err != nil {
		return err
	}
	if err := validateClassAbilities(document["class_abilities"].(map[string]any)); err != nil {
		return err
	}
	if err := validateLegacy(document["legacy"].(map[string]any)); err != nil {
		return err
	}
	return validateChangelog(document["changelog"].([]any))
}

func validateTalents(classes map[string]any) error {
	for className, rawClass := range classes {
		classPath := "talents[" + strconv.Quote(className) + "]"
		class, err := objectAt(rawClass, classPath)
		if err != nil {
			return err
		}
		trees, err := arrayAt(class["trees"], classPath+".trees")
		if err != nil {
			return err
		}
		for treeIndex, rawTree := range trees {
			treePath := fmt.Sprintf("%s.trees[%d]", classPath, treeIndex)
			tree, err := objectAt(rawTree, treePath)
			if err != nil {
				return err
			}
			if _, err := nonEmptyStringAt(tree["name"], treePath+".name"); err != nil {
				return err
			}
			talents, err := arrayAt(tree["talents"], treePath+".talents")
			if err != nil {
				return err
			}
			positions := make(map[string]bool)
			for talentIndex, rawTalent := range talents {
				talentPath := fmt.Sprintf("%s.talents[%d]", treePath, talentIndex)
				talent, err := objectAt(rawTalent, talentPath)
				if err != nil {
					return err
				}
				if _, err := nonEmptyStringAt(talent["name"], talentPath+".name"); err != nil {
					return err
				}
				row, err := positiveIntegerAt(talent["row"], talentPath+".row")
				if err != nil {
					return err
				}
				column, err := positiveIntegerAt(talent["col"], talentPath+".col")
				if err != nil {
					return err
				}
				maxRank, err := positiveIntegerAt(talent["max"], talentPath+".max")
				if err != nil {
					return err
				}
				position := fmt.Sprintf("%d,%d", row, column)
				if positions[position] {
					return fmt.Errorf("%s has duplicate grid position %s", treePath, position)
				}
				positions[position] = true
				if _, ok := talent["passive"].(bool); !ok {
					return fmt.Errorf("%s.passive must be a boolean", talentPath)
				}
				if _, err := nonEmptyStringAt(talent["icon"], talentPath+".icon"); err != nil {
					return err
				}
				if _, ok := talent["complete"].(bool); !ok {
					return fmt.Errorf("%s.complete must be a boolean", talentPath)
				}
				if err := validateTalentDescriptions(talent["desc"], maxRank, talentPath+".desc"); err != nil {
					return err
				}
				classic, err := objectAt(talent["classic"], talentPath+".classic")
				if err != nil {
					return err
				}
				if err := validateClassicComparison(classic, talentPath+".classic"); err != nil {
					return err
				}
				if confirmed, exists := talent["confirmed"]; exists {
					ranks, err := arrayAt(confirmed, talentPath+".confirmed")
					if err != nil {
						return err
					}
					seenRanks := make(map[int64]bool)
					for rankIndex, rawRank := range ranks {
						rank, err := positiveIntegerAt(rawRank, fmt.Sprintf("%s.confirmed[%d]", talentPath, rankIndex))
						if err != nil {
							return err
						}
						if rank > maxRank {
							return fmt.Errorf("%s.confirmed[%d] is %d, greater than max rank %d", talentPath, rankIndex, rank, maxRank)
						}
						if seenRanks[rank] {
							return fmt.Errorf("%s.confirmed contains duplicate rank %d", talentPath, rank)
						}
						seenRanks[rank] = true
					}
				}
				if estimates, exists := talent["est"]; exists {
					if err := validateRankedStrings(estimates, maxRank, talentPath+".est"); err != nil {
						return err
					}
				}
				if fixed, exists := talent["fixed"]; exists {
					values, err := arrayAt(fixed, talentPath+".fixed")
					if err != nil {
						return err
					}
					if err := validateStringArray(values, talentPath+".fixed"); err != nil {
						return err
					}
				}
				if scaleIndexes, exists := talent["scaleIdx"]; exists {
					values, err := arrayAt(scaleIndexes, talentPath+".scaleIdx")
					if err != nil {
						return err
					}
					for scaleIndex, rawValue := range values {
						if _, err := nonNegativeIntegerAt(rawValue, fmt.Sprintf("%s.scaleIdx[%d]", talentPath, scaleIndex)); err != nil {
							return err
						}
					}
				}
				for _, field := range []string{"cost", "note", "req", "reqText"} {
					if value, exists := talent[field]; exists {
						if _, ok := value.(string); !ok {
							return fmt.Errorf("%s.%s must be a string", talentPath, field)
						}
					}
				}
			}
			if removed, exists := tree["removed"]; exists {
				removedRecords, err := arrayAt(removed, treePath+".removed")
				if err != nil {
					return err
				}
				for index, rawRemoved := range removedRecords {
					path := fmt.Sprintf("%s.removed[%d]", treePath, index)
					record, err := objectAt(rawRemoved, path)
					if err != nil {
						return err
					}
					if _, err := nonEmptyStringAt(record["name"], path+".name"); err != nil {
						return err
					}
					for _, field := range []string{"row", "max"} {
						if _, err := positiveIntegerAt(record[field], path+"."+field); err != nil {
							return err
						}
					}
					if _, err := nonEmptyStringAt(record["text"], path+".text"); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func validateClassicComparison(classic map[string]any, path string) error {
	status, ok := classic["status"].(string)
	if !ok {
		return fmt.Errorf("%s.status must be a string", path)
	}
	switch status {
	case "new":
		// A new Forever talent has no Classic comparison record.
	case "same", "changed", "moved":
		if _, err := nonEmptyStringAt(classic["tree"], path+".tree"); err != nil {
			return err
		}
		for _, field := range []string{"row", "col", "max"} {
			if _, err := positiveIntegerAt(classic[field], path+"."+field); err != nil {
				return err
			}
		}
		if _, err := nonEmptyStringAt(classic["text"], path+".text"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%s.status has unsupported value %q", path, status)
	}
	if moved, exists := classic["moved"]; exists {
		if _, ok := moved.(bool); !ok {
			return fmt.Errorf("%s.moved must be a boolean", path)
		}
	}
	if renamed, exists := classic["renamed"]; exists {
		if _, err := nonEmptyStringAt(renamed, path+".renamed"); err != nil {
			return err
		}
	}
	return nil
}

func validateRankedStrings(value any, maxRank int64, path string) error {
	ranks, err := objectAt(value, path)
	if err != nil {
		return err
	}
	for rankText, rawDescription := range ranks {
		rank, err := strconv.ParseInt(rankText, 10, 64)
		if err != nil || rank <= 0 || rank > maxRank {
			return fmt.Errorf("%s has invalid rank key %q", path, rankText)
		}
		if _, ok := rawDescription.(string); !ok {
			return fmt.Errorf("%s[%q] must be a string", path, rankText)
		}
	}
	return nil
}

func validateTalentDescriptions(value any, maxRank int64, path string) error {
	switch descriptions := value.(type) {
	case []any:
		if len(descriptions) == 0 || int64(len(descriptions)) > maxRank {
			return fmt.Errorf("%s must contain between 1 and %d ranks", path, maxRank)
		}
		for index, rawDescription := range descriptions {
			if _, ok := rawDescription.(string); !ok {
				return fmt.Errorf("%s[%d] must be a string", path, index)
			}
		}
	case map[string]any:
		if len(descriptions) == 0 {
			return fmt.Errorf("%s must contain at least one rank", path)
		}
		if err := validateRankedStrings(descriptions, maxRank, path); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%s must be an array or object", path)
	}
	return nil
}

func validateSpellbooks(classes map[string]any) error {
	for className, rawSpellbook := range classes {
		path := "spellbooks[" + strconv.Quote(className) + "]"
		spellbook, err := objectAt(rawSpellbook, path)
		if err != nil {
			return err
		}
		if _, err := nonEmptyStringAt(spellbook["race"], path+".race"); err != nil {
			return err
		}
		if _, err := positiveIntegerAt(spellbook["level"], path+".level"); err != nil {
			return err
		}
		if _, ok := spellbook["seen"].(string); !ok {
			return fmt.Errorf("%s.seen must be a string", path)
		}
		for _, field := range []string{"missing", "notes"} {
			values, err := arrayAt(spellbook[field], path+"."+field)
			if err != nil {
				return err
			}
			if err := validateStringArray(values, path+"."+field); err != nil {
				return err
			}
		}
		general, err := arrayAt(spellbook["general"], path+".general")
		if err != nil {
			return err
		}
		if err := validateStringTuples(general, 2, path+".general"); err != nil {
			return err
		}
		tabs, err := arrayAt(spellbook["tabs"], path+".tabs")
		if err != nil {
			return err
		}
		for index, rawTab := range tabs {
			tabPath := fmt.Sprintf("%s.tabs[%d]", path, index)
			tab, err := objectAt(rawTab, tabPath)
			if err != nil {
				return err
			}
			if _, err := nonEmptyStringAt(tab["name"], tabPath+".name"); err != nil {
				return err
			}
			spells, err := arrayAt(tab["spells"], tabPath+".spells")
			if err != nil {
				return err
			}
			if err := validateStringTuples(spells, 2, tabPath+".spells"); err != nil {
				return err
			}
		}
		if rawLevels, exists := spellbook["levels"]; exists {
			levels, err := objectAt(rawLevels, path+".levels")
			if err != nil {
				return err
			}
			for spellName, rawRanks := range levels {
				spellPath := path + ".levels[" + strconv.Quote(spellName) + "]"
				ranks, err := arrayAt(rawRanks, spellPath)
				if err != nil {
					return err
				}
				for rankIndex, rawRank := range ranks {
					if rawRank == nil {
						continue
					}
					if _, err := positiveIntegerAt(rawRank, fmt.Sprintf("%s[%d]", spellPath, rankIndex)); err != nil {
						return err
					}
				}
			}
			if _, ok := spellbook["levelsSource"].(string); !ok {
				return fmt.Errorf("%s.levelsSource must be a string when levels is present", path)
			}
		}
	}
	return nil
}

func validateSpellDescriptions(descriptions map[string]any) error {
	for key, rawDescription := range descriptions {
		path := fmt.Sprintf("spell_desc[%q]", key)
		description, err := objectAt(rawDescription, path)
		if err != nil {
			return err
		}
		for _, field := range []string{"d", "src", "r", "lv"} {
			if _, ok := description[field].(string); !ok {
				return fmt.Errorf("%s.%s must be a string", path, field)
			}
		}
		source, ok := description["s"].(string)
		if !ok || (source != "demo" && source != "classic") {
			return fmt.Errorf("%s.s must be \"demo\" or \"classic\"", path)
		}
		lines, err := arrayAt(description["l"], path+".l")
		if err != nil {
			return err
		}
		if err := validateStringTuples(lines, 2, path+".l"); err != nil {
			return err
		}
		if id, exists := description["id"]; !exists {
			return fmt.Errorf("%s.id is required", path)
		} else if id != nil {
			if _, err := positiveIntegerAt(id, path+".id"); err != nil {
				return fmt.Errorf("%s.id must be a positive integer or null", path)
			}
		}
	}
	return nil
}

func validateRacials(factions map[string]any) error {
	for _, factionName := range []string{"Alliance", "Horde"} {
		races, err := arrayAt(factions[factionName], "racials."+factionName)
		if err != nil {
			return err
		}
		for index, rawRace := range races {
			path := fmt.Sprintf("racials.%s[%d]", factionName, index)
			race, err := objectAt(rawRace, path)
			if err != nil {
				return err
			}
			if _, err := nonEmptyStringAt(race["race"], path+".race"); err != nil {
				return err
			}
			classes, err := arrayAt(race["classes"], path+".classes")
			if err != nil {
				return err
			}
			if err := validateStringArray(classes, path+".classes"); err != nil {
				return err
			}
			abilities, err := arrayAt(race["abilities"], path+".abilities")
			if err != nil {
				return err
			}
			if err := validateStringTuples(abilities, 3, path+".abilities"); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateClassRacials(classes map[string]any) error {
	for className, rawClass := range classes {
		path := "class_racials[" + strconv.Quote(className) + "]"
		class, err := objectAt(rawClass, path)
		if err != nil {
			return err
		}
		races, err := objectAt(class["races"], path+".races")
		if err != nil {
			return err
		}
		for raceName, rawAbilities := range races {
			racePath := path + ".races[" + strconv.Quote(raceName) + "]"
			abilities, err := arrayAt(rawAbilities, racePath)
			if err != nil {
				return err
			}
			if err := validateStringTuples(abilities, 3, racePath); err != nil {
				return err
			}
		}
		for _, field := range []string{"note", "sources"} {
			if value, exists := class[field]; exists {
				if _, ok := value.(string); !ok {
					return fmt.Errorf("%s.%s must be a string", path, field)
				}
			}
		}
	}
	return nil
}

func validateClassAbilities(classes map[string]any) error {
	for _, className := range requiredClasses {
		if _, exists := classes[className]; !exists {
			return fmt.Errorf("class_abilities.%s must be an array", className)
		}
	}
	for className, rawAbilities := range classes {
		path := "class_abilities[" + strconv.Quote(className) + "]"
		abilities, err := arrayAt(rawAbilities, path)
		if err != nil {
			return err
		}
		if err := validateStringTuples(abilities, 3, path); err != nil {
			return err
		}
	}
	return nil
}

func validateLegacy(legacy map[string]any) error {
	if note, exists := legacy["note"]; exists {
		if _, ok := note.(string); !ok {
			return fmt.Errorf("legacy.note must be a string")
		}
	}
	trees, err := arrayAt(legacy["trees"], "legacy.trees")
	if err != nil {
		return err
	}
	for index, rawTree := range trees {
		path := fmt.Sprintf("legacy.trees[%d]", index)
		tree, err := objectAt(rawTree, path)
		if err != nil {
			return err
		}
		if _, err := nonEmptyStringAt(tree["name"], path+".name"); err != nil {
			return err
		}
		perks, err := arrayAt(tree["perks"], path+".perks")
		if err != nil {
			return err
		}
		for perkIndex, rawPerk := range perks {
			perkPath := fmt.Sprintf("%s.perks[%d]", path, perkIndex)
			perk, err := arrayAt(rawPerk, perkPath)
			if err != nil {
				return err
			}
			if len(perk) != 4 {
				return fmt.Errorf("%s must contain exactly 4 values", perkPath)
			}
			if _, err := nonEmptyStringAt(perk[0], perkPath+"[0]"); err != nil {
				return err
			}
			if _, err := positiveIntegerAt(perk[1], perkPath+"[1]"); err != nil {
				return err
			}
			for field := 2; field < 4; field++ {
				if _, ok := perk[field].(string); !ok {
					return fmt.Errorf("%s[%d] must be a string", perkPath, field)
				}
			}
		}
	}
	return nil
}

func validateChangelog(entries []any) error {
	for index, rawEntry := range entries {
		path := fmt.Sprintf("changelog[%d]", index)
		entry, err := objectAt(rawEntry, path)
		if err != nil {
			return err
		}
		date, err := nonEmptyStringAt(entry["date"], path+".date")
		if err != nil {
			return err
		}
		if parsed, err := time.Parse(time.DateOnly, date); err != nil || parsed.Format(time.DateOnly) != date {
			return fmt.Errorf("%s.date %q is not YYYY-MM-DD", path, date)
		}
		if _, err := nonEmptyStringAt(entry["title"], path+".title"); err != nil {
			return err
		}
		if _, ok := entry["text"].(string); !ok {
			return fmt.Errorf("%s.text must be a string", path)
		}
	}
	return nil
}

func validateStringTuples(values []any, length int, path string) error {
	for index, rawTuple := range values {
		tuplePath := fmt.Sprintf("%s[%d]", path, index)
		tuple, err := arrayAt(rawTuple, tuplePath)
		if err != nil {
			return err
		}
		if len(tuple) != length {
			return fmt.Errorf("%s must contain exactly %d strings", tuplePath, length)
		}
		if err := validateStringArray(tuple, tuplePath); err != nil {
			return err
		}
		if tuple[0] == "" {
			return fmt.Errorf("%s[0] must be a non-empty string", tuplePath)
		}
	}
	return nil
}

func validateStringArray(values []any, path string) error {
	for index, value := range values {
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s[%d] must be a string", path, index)
		}
	}
	return nil
}

func objectAt(value any, path string) (map[string]any, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an object", path)
	}
	return object, nil
}

func arrayAt(value any, path string) ([]any, error) {
	array, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", path)
	}
	return array, nil
}

func nonEmptyStringAt(value any, path string) (string, error) {
	text, ok := value.(string)
	if !ok || text == "" {
		return "", fmt.Errorf("%s must be a non-empty string", path)
	}
	return text, nil
}

func positiveIntegerAt(value any, path string) (int64, error) {
	return integerAt(value, path, false)
}

func nonNegativeIntegerAt(value any, path string) (int64, error) {
	return integerAt(value, path, true)
}

func integerAt(value any, path string, allowZero bool) (int64, error) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, integerError(path, allowZero)
	}
	normalized, err := normalizeJSONNumber(number.String())
	if err != nil {
		return 0, integerError(path, allowZero)
	}
	mantissa, exponentText, hasExponent := strings.Cut(normalized, "e")
	integer := new(big.Int)
	if _, ok := integer.SetString(mantissa, 10); !ok || integer.Sign() < 0 || (!allowZero && integer.Sign() == 0) {
		return 0, integerError(path, allowZero)
	}
	if hasExponent {
		exponent := new(big.Int)
		if _, ok := exponent.SetString(exponentText, 10); !ok || exponent.Sign() < 0 || !exponent.IsInt64() || exponent.Int64() > 18 {
			return 0, integerError(path, allowZero)
		}
		power := new(big.Int).Exp(big.NewInt(10), exponent, nil)
		integer.Mul(integer, power)
	}
	if !integer.IsInt64() {
		return 0, integerError(path, allowZero)
	}
	return integer.Int64(), nil
}

func integerError(path string, allowZero bool) error {
	if allowZero {
		return fmt.Errorf("%s must be a non-negative integer", path)
	}
	return fmt.Errorf("%s must be a positive integer", path)
}
