package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/big"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	sourceURL            = "https://talentsforever.com/data.json"
	sourceLicense        = "CC-BY-4.0"
	sourceAttribution    = "Data from talentsforever.com (https://talentsforever.com)"
	snapshotRelativePath = "third_party/talentsforever/data.json"
	manifestRelativePath = "third_party/talentsforever/manifest.json"
	noticeRelativePath   = "third_party/talentsforever/NOTICE.md"
	manifestSchema       = 1
	canonicalizerVersion = 1
	evidenceTier         = "community_secondary"
	runtimeStatus        = "reference_only"
	maxSourceBytes       = 4 << 20
	maxJSONDepth         = 128
	maxPrintedChanges    = 20
)

var requiredClasses = []string{
	"Druid", "Hunter", "Mage", "Paladin", "Priest", "Rogue", "Shaman", "Warlock", "Warrior",
}

var requiredObjectSections = []string{
	"talents", "spellbooks", "spell_desc", "racials", "class_racials", "class_abilities", "legacy",
}

var metadataKeys = map[string]bool{
	"_readme": true, "license": true, "attribution": true, "generated": true,
}

type repositoryFiles struct {
	root     string
	snapshot string
	manifest string
	notice   string
}

type fetchedSource struct {
	body         []byte
	lastModified string
}

type sourceManifest struct {
	URL          string `json:"url"`
	License      string `json:"license"`
	Attribution  string `json:"attribution"`
	Generated    string `json:"generated"`
	RetrievedAt  string `json:"retrievedAt"`
	LastModified string `json:"lastModified,omitempty"`
}

type snapshotManifest struct {
	Path               string `json:"path"`
	Bytes              int    `json:"bytes"`
	RawSHA256          string `json:"rawSha256"`
	DocumentBytes      int    `json:"documentBytes"`
	DocumentSHA256     string `json:"documentSha256"`
	EvidenceSHA256     string `json:"evidenceSha256"`
	SourceMetadataHash string `json:"sourceMetadataSha256"`
}

type sectionManifest struct {
	Type    string `json:"type"`
	Entries int    `json:"entries"`
	SHA256  string `json:"sha256"`
}

type snapshotMetrics struct {
	Classes                  int `json:"classes"`
	TalentTrees              int `json:"talentTrees"`
	Talents                  int `json:"talents"`
	CompleteTalents          int `json:"completeTalents"`
	TalentsWithConfirmedRank int `json:"talentsWithConfirmedRank"`
	SpellbookEntries         int `json:"spellbookEntries"`
	SpellDescriptions        int `json:"spellDescriptions"`
	DemoSpellDescriptions    int `json:"demoSpellDescriptions"`
	ClassicSpellDescriptions int `json:"classicSpellDescriptions"`
	RacialAbilities          int `json:"racialAbilities"`
	ClassRacialAbilities     int `json:"classRacialAbilities"`
	ClassAbilities           int `json:"classAbilities"`
	LegacyPerks              int `json:"legacyPerks"`
	ChangelogEntries         int `json:"changelogEntries"`
}

type dataManifest struct {
	SchemaVersion        int                        `json:"schemaVersion"`
	CanonicalizerVersion int                        `json:"canonicalizerVersion"`
	EvidenceTier         string                     `json:"evidenceTier"`
	RuntimeStatus        string                     `json:"runtimeStatus"`
	Source               sourceManifest             `json:"source"`
	Snapshot             snapshotManifest           `json:"snapshot"`
	Sections             map[string]sectionManifest `json:"sections"`
	Metrics              snapshotMetrics            `json:"metrics"`
}

type dataState struct {
	raw      []byte
	document map[string]any
	manifest dataManifest
}

func repositoryPaths(root string) repositoryFiles {
	return repositoryFiles{
		root:     root,
		snapshot: filepath.Join(root, filepath.FromSlash(snapshotRelativePath)),
		manifest: filepath.Join(root, filepath.FromSlash(manifestRelativePath)),
		notice:   filepath.Join(root, filepath.FromSlash(noticeRelativePath)),
	}
}

func findRepositoryRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		contents, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && bytes.Contains(contents, []byte("module github.com/wowsims/tbc")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("could not find the repository root")
		}
		dir = parent
	}
}

func defaultHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func fetchSource(ctx context.Context, source string, client *http.Client) (fetchedSource, error) {
	parsed, err := url.Parse(source)
	if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if err != nil {
			return fetchedSource{}, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "wowsims-forever-data/1")

		response, err := client.Do(req)
		if err != nil {
			return fetchedSource{}, err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fetchedSource{}, fmt.Errorf("%s returned HTTP %s", source, response.Status)
		}
		mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			return fetchedSource{}, fmt.Errorf("%s returned Content-Type %q, want application/json", source, response.Header.Get("Content-Type"))
		}
		if response.ContentLength > maxSourceBytes {
			return fetchedSource{}, fmt.Errorf("response is %d bytes, limit is %d", response.ContentLength, maxSourceBytes)
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, maxSourceBytes+1))
		if err != nil {
			return fetchedSource{}, err
		}
		if len(body) > maxSourceBytes {
			return fetchedSource{}, fmt.Errorf("decompressed response exceeds %d bytes", maxSourceBytes)
		}
		return fetchedSource{body: body, lastModified: response.Header.Get("Last-Modified")}, nil
	}

	body, err := os.ReadFile(source)
	if err != nil {
		return fetchedSource{}, err
	}
	if len(body) > maxSourceBytes {
		return fetchedSource{}, fmt.Errorf("file is %d bytes, limit is %d", len(body), maxSourceBytes)
	}
	return fetchedSource{body: body}, nil
}

func makeCandidate(fetched fetchedSource, source string, retrievedAt time.Time) (dataState, error) {
	if fetched.lastModified != "" {
		if _, err := http.ParseTime(fetched.lastModified); err != nil {
			return dataState{}, fmt.Errorf("invalid Last-Modified header %q: %w", fetched.lastModified, err)
		}
	}
	document, err := decodeDocument(fetched.body)
	if err != nil {
		return dataState{}, err
	}
	if err := validateDocument(document); err != nil {
		return dataState{}, err
	}
	manifest, err := buildManifest(document, fetched.body, sourceManifest{
		URL:          source,
		License:      sourceLicense,
		Attribution:  sourceAttribution,
		Generated:    document["generated"].(string),
		RetrievedAt:  retrievedAt.Truncate(time.Second).Format(time.RFC3339),
		LastModified: fetched.lastModified,
	})
	if err != nil {
		return dataState{}, err
	}
	return dataState{raw: fetched.body, document: document, manifest: manifest}, nil
}

func decodeDocument(raw []byte) (map[string]any, error) {
	if !utf8.Valid(raw) {
		return nil, errors.New("JSON is not valid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeJSONValue(decoder, 0)
	if err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if token, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("invalid JSON: unexpected trailing token %v", token)
		}
		return nil, fmt.Errorf("invalid JSON after document: %w", err)
	}
	document, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("top-level JSON value must be an object")
	}
	return document, nil
}

func decodeJSONValue(decoder *json.Decoder, depth int) (any, error) {
	if depth > maxJSONDepth {
		return nil, fmt.Errorf("nesting exceeds %d levels", maxJSONDepth)
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return token, nil
	}

	switch delimiter {
	case '{':
		object := make(map[string]any)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, fmt.Errorf("object key is %T, want string", keyToken)
			}
			if _, duplicate := object[key]; duplicate {
				return nil, fmt.Errorf("duplicate object key %q", key)
			}
			value, err := decodeJSONValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		closing, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		if closing != json.Delim('}') {
			return nil, fmt.Errorf("object ended with %v", closing)
		}
		return object, nil

	case '[':
		array := make([]any, 0)
		for decoder.More() {
			value, err := decodeJSONValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		closing, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		if closing != json.Delim(']') {
			return nil, fmt.Errorf("array ended with %v", closing)
		}
		return array, nil
	default:
		return nil, fmt.Errorf("unexpected delimiter %q", delimiter)
	}
}

func validateDocument(document map[string]any) error {
	readme, ok := document["_readme"].(string)
	if !ok || strings.TrimSpace(readme) == "" {
		return errors.New("_readme must be a non-empty string")
	}
	if !strings.Contains(readme, "https://creativecommons.org/licenses/by/4.0/") {
		return errors.New("_readme no longer contains the expected CC BY 4.0 link; manual license review required")
	}
	if document["license"] != sourceLicense {
		return fmt.Errorf("license is %s, want %q; manual license review required", describeJSONValue(document["license"]), sourceLicense)
	}
	if document["attribution"] != sourceAttribution {
		return fmt.Errorf("attribution is %s, want %q; manual attribution review required", describeJSONValue(document["attribution"]), sourceAttribution)
	}
	generated, ok := document["generated"].(string)
	if !ok {
		return errors.New("generated must be a YYYY-MM-DD string")
	}
	if parsed, err := time.Parse(time.DateOnly, generated); err != nil || parsed.Format(time.DateOnly) != generated {
		return fmt.Errorf("generated %q is not YYYY-MM-DD", generated)
	}

	for _, name := range requiredObjectSections {
		if _, ok := document[name].(map[string]any); !ok {
			return fmt.Errorf("%s must be an object", name)
		}
	}
	if _, ok := document["changelog"].([]any); !ok {
		return errors.New("changelog must be an array")
	}

	talents := document["talents"].(map[string]any)
	spellbooks := document["spellbooks"].(map[string]any)
	for _, class := range requiredClasses {
		classTalents, ok := talents[class].(map[string]any)
		if !ok {
			return fmt.Errorf("talents.%s must be an object", class)
		}
		trees, ok := classTalents["trees"].([]any)
		if !ok || len(trees) == 0 {
			return fmt.Errorf("talents.%s.trees must be a non-empty array", class)
		}
		seenTrees := make(map[string]bool)
		for index, rawTree := range trees {
			tree, ok := rawTree.(map[string]any)
			if !ok {
				return fmt.Errorf("talents.%s.trees[%d] must be an object", class, index)
			}
			name, ok := tree["name"].(string)
			if !ok || name == "" {
				return fmt.Errorf("talents.%s.trees[%d].name must be a non-empty string", class, index)
			}
			if seenTrees[name] {
				return fmt.Errorf("talents.%s has duplicate tree name %q", class, name)
			}
			seenTrees[name] = true
			if _, ok := tree["talents"].([]any); !ok {
				return fmt.Errorf("talents.%s tree %q talents must be an array", class, name)
			}
		}
		if _, ok := spellbooks[class].(map[string]any); !ok {
			return fmt.Errorf("spellbooks.%s must be an object", class)
		}
	}
	return validateKnownRecords(document)
}

func describeJSONValue(value any) string {
	if text, ok := value.(string); ok {
		return strconv.Quote(text)
	}
	return jsonType(value)
}

func buildManifest(document map[string]any, raw []byte, source sourceManifest) (dataManifest, error) {
	canonicalDocument, err := canonicalJSON(document)
	if err != nil {
		return dataManifest{}, err
	}
	metadata := make(map[string]any)
	evidence := make(map[string]any)
	sections := make(map[string]sectionManifest)
	for key, value := range document {
		if metadataKeys[key] {
			metadata[key] = value
			continue
		}
		sectionJSON, err := canonicalJSON(value)
		if err != nil {
			return dataManifest{}, err
		}
		sections[key] = sectionManifest{
			Type:    jsonType(value),
			Entries: entryCount(value),
			SHA256:  sha256Hex(sectionJSON),
		}
		if key != "changelog" {
			evidence[key] = value
		}
	}
	canonicalEvidence, err := canonicalJSON(evidence)
	if err != nil {
		return dataManifest{}, err
	}
	canonicalMetadata, err := canonicalJSON(metadata)
	if err != nil {
		return dataManifest{}, err
	}
	return dataManifest{
		SchemaVersion:        manifestSchema,
		CanonicalizerVersion: canonicalizerVersion,
		EvidenceTier:         evidenceTier,
		RuntimeStatus:        runtimeStatus,
		Source:               source,
		Snapshot: snapshotManifest{
			Path:               snapshotRelativePath,
			Bytes:              len(raw),
			RawSHA256:          sha256Hex(raw),
			DocumentBytes:      len(canonicalDocument),
			DocumentSHA256:     sha256Hex(canonicalDocument),
			EvidenceSHA256:     sha256Hex(canonicalEvidence),
			SourceMetadataHash: sha256Hex(canonicalMetadata),
		},
		Sections: sections,
		Metrics:  calculateMetrics(document),
	}, nil
}

func calculateMetrics(document map[string]any) snapshotMetrics {
	metrics := snapshotMetrics{}
	talents, _ := document["talents"].(map[string]any)
	metrics.Classes = len(talents)
	for _, rawClass := range talents {
		class, _ := rawClass.(map[string]any)
		trees, _ := class["trees"].([]any)
		metrics.TalentTrees += len(trees)
		for _, rawTree := range trees {
			tree, _ := rawTree.(map[string]any)
			talentList, _ := tree["talents"].([]any)
			metrics.Talents += len(talentList)
			for _, rawTalent := range talentList {
				talent, _ := rawTalent.(map[string]any)
				if complete, _ := talent["complete"].(bool); complete {
					metrics.CompleteTalents++
				}
				if confirmed, ok := talent["confirmed"].([]any); ok && len(confirmed) != 0 {
					metrics.TalentsWithConfirmedRank++
				}
			}
		}
	}
	spellDescriptions, _ := document["spell_desc"].(map[string]any)
	metrics.SpellDescriptions = len(spellDescriptions)
	for _, rawDescription := range spellDescriptions {
		description, _ := rawDescription.(map[string]any)
		source, _ := description["s"].(string)
		switch source {
		case "demo":
			metrics.DemoSpellDescriptions++
		case "classic":
			metrics.ClassicSpellDescriptions++
		}
	}
	spellbooks, _ := document["spellbooks"].(map[string]any)
	for _, rawSpellbook := range spellbooks {
		spellbook, _ := rawSpellbook.(map[string]any)
		general, _ := spellbook["general"].([]any)
		metrics.SpellbookEntries += len(general)
		tabs, _ := spellbook["tabs"].([]any)
		for _, rawTab := range tabs {
			tab, _ := rawTab.(map[string]any)
			spells, _ := tab["spells"].([]any)
			metrics.SpellbookEntries += len(spells)
		}
	}
	factions, _ := document["racials"].(map[string]any)
	for _, rawRaces := range factions {
		races, _ := rawRaces.([]any)
		for _, rawRace := range races {
			race, _ := rawRace.(map[string]any)
			abilities, _ := race["abilities"].([]any)
			metrics.RacialAbilities += len(abilities)
		}
	}
	classRacials, _ := document["class_racials"].(map[string]any)
	for _, rawClass := range classRacials {
		class, _ := rawClass.(map[string]any)
		races, _ := class["races"].(map[string]any)
		for _, rawAbilities := range races {
			abilities, _ := rawAbilities.([]any)
			metrics.ClassRacialAbilities += len(abilities)
		}
	}
	classAbilities, _ := document["class_abilities"].(map[string]any)
	for _, rawAbilities := range classAbilities {
		abilities, _ := rawAbilities.([]any)
		metrics.ClassAbilities += len(abilities)
	}
	legacy, _ := document["legacy"].(map[string]any)
	legacyTrees, _ := legacy["trees"].([]any)
	for _, rawTree := range legacyTrees {
		tree, _ := rawTree.(map[string]any)
		perks, _ := tree["perks"].([]any)
		metrics.LegacyPerks += len(perks)
	}
	changelog, _ := document["changelog"].([]any)
	metrics.ChangelogEntries = len(changelog)
	return metrics
}

func canonicalJSON(value any) ([]byte, error) {
	normalized, err := normalizeJSONNumbers(value)
	if err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(normalized); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

func marshalManifest(manifest dataManifest) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(manifest); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func readCommittedState(paths repositoryFiles) (dataState, error) {
	raw, err := os.ReadFile(paths.snapshot)
	if err != nil {
		return dataState{}, fmt.Errorf("read %s: %w", snapshotRelativePath, err)
	}
	manifestBytes, err := os.ReadFile(paths.manifest)
	if err != nil {
		return dataState{}, fmt.Errorf("read %s: %w", manifestRelativePath, err)
	}
	notice, err := os.ReadFile(paths.notice)
	if err != nil {
		return dataState{}, fmt.Errorf("read %s: %w", noticeRelativePath, err)
	}
	if err := validateNotice(notice); err != nil {
		return dataState{}, err
	}

	document, err := decodeDocument(raw)
	if err != nil {
		return dataState{}, fmt.Errorf("%s: %w", snapshotRelativePath, err)
	}
	if err := validateDocument(document); err != nil {
		return dataState{}, fmt.Errorf("%s: %w", snapshotRelativePath, err)
	}
	manifest, err := decodeManifest(manifestBytes)
	if err != nil {
		return dataState{}, fmt.Errorf("%s: %w", manifestRelativePath, err)
	}
	if manifest.SchemaVersion != manifestSchema {
		return dataState{}, fmt.Errorf("manifest schema is %d, want %d", manifest.SchemaVersion, manifestSchema)
	}
	if manifest.CanonicalizerVersion != canonicalizerVersion {
		return dataState{}, fmt.Errorf("canonicalizer version is %d, want %d", manifest.CanonicalizerVersion, canonicalizerVersion)
	}
	if _, err := time.Parse(time.RFC3339, manifest.Source.RetrievedAt); err != nil {
		return dataState{}, fmt.Errorf("manifest retrievedAt is invalid: %w", err)
	}
	if manifest.Source.LastModified != "" {
		if _, err := http.ParseTime(manifest.Source.LastModified); err != nil {
			return dataState{}, fmt.Errorf("manifest lastModified is invalid: %w", err)
		}
	}
	expected, err := buildManifest(document, raw, sourceManifest{
		URL:          sourceURL,
		License:      sourceLicense,
		Attribution:  sourceAttribution,
		Generated:    document["generated"].(string),
		RetrievedAt:  manifest.Source.RetrievedAt,
		LastModified: manifest.Source.LastModified,
	})
	if err != nil {
		return dataState{}, err
	}
	expectedBytes, err := marshalManifest(expected)
	if err != nil {
		return dataState{}, err
	}
	if !bytes.Equal(manifestBytes, expectedBytes) {
		return dataState{}, errors.New("manifest is noncanonical or does not match the snapshot; run the updater after review")
	}
	return dataState{raw: raw, document: document, manifest: manifest}, nil
}

func readCommittedStateIfPresent(paths repositoryFiles) (dataState, bool, error) {
	_, snapshotErr := os.Stat(paths.snapshot)
	_, manifestErr := os.Stat(paths.manifest)
	if errors.Is(snapshotErr, fs.ErrNotExist) && errors.Is(manifestErr, fs.ErrNotExist) {
		if notice, err := os.ReadFile(paths.notice); err != nil {
			return dataState{}, true, fmt.Errorf("read %s: %w", noticeRelativePath, err)
		} else if err := validateNotice(notice); err != nil {
			return dataState{}, true, err
		}
		return dataState{}, true, nil
	}
	if snapshotErr != nil || manifestErr != nil {
		return dataState{}, false, errors.New("snapshot and manifest must either both exist or both be absent")
	}
	state, err := readCommittedState(paths)
	return state, false, err
}

func rebuildCommittedManifest(paths repositoryFiles) (dataState, error) {
	release, err := acquireUpdateLock(paths)
	if err != nil {
		return dataState{}, err
	}
	defer release()

	raw, err := os.ReadFile(paths.snapshot)
	if err != nil {
		return dataState{}, fmt.Errorf("read %s: %w", snapshotRelativePath, err)
	}
	notice, err := os.ReadFile(paths.notice)
	if err != nil {
		return dataState{}, fmt.Errorf("read %s: %w", noticeRelativePath, err)
	}
	if err := validateNotice(notice); err != nil {
		return dataState{}, err
	}
	document, err := decodeDocument(raw)
	if err != nil {
		return dataState{}, fmt.Errorf("%s: %w", snapshotRelativePath, err)
	}
	if err := validateDocument(document); err != nil {
		return dataState{}, fmt.Errorf("%s: %w", snapshotRelativePath, err)
	}

	oldManifest, err := os.ReadFile(paths.manifest)
	if err != nil {
		return dataState{}, fmt.Errorf("read %s: %w", manifestRelativePath, err)
	}
	preserved, err := decodeManifestProvenance(oldManifest)
	if err != nil {
		return dataState{}, fmt.Errorf("cannot preserve provenance from %s: %w", manifestRelativePath, err)
	}
	provenance := preserved.Source
	if provenance.URL != sourceURL || provenance.License != sourceLicense || provenance.Attribution != sourceAttribution {
		return dataState{}, errors.New("old manifest provenance does not match the canonical source, license, and attribution")
	}
	if provenance.Generated != document["generated"] {
		return dataState{}, errors.New("old manifest generated date does not match the snapshot")
	}
	if _, err := time.Parse(time.RFC3339, provenance.RetrievedAt); err != nil {
		return dataState{}, fmt.Errorf("old manifest retrievedAt is invalid: %w", err)
	}
	if provenance.LastModified != "" {
		if _, err := http.ParseTime(provenance.LastModified); err != nil {
			return dataState{}, fmt.Errorf("old manifest lastModified is invalid: %w", err)
		}
	}
	if preserved.Snapshot.Path != snapshotRelativePath || preserved.Snapshot.Bytes != len(raw) || preserved.Snapshot.RawSHA256 != sha256Hex(raw) {
		return dataState{}, errors.New("old manifest raw snapshot identity does not match data.json; refusing to bless possible tampering")
	}
	manifest, err := buildManifest(document, raw, provenance)
	if err != nil {
		return dataState{}, err
	}
	manifestBytes, err := marshalManifest(manifest)
	if err != nil {
		return dataState{}, err
	}
	if err := atomicWrite(paths.manifest, manifestBytes); err != nil {
		return dataState{}, err
	}
	return dataState{raw: raw, document: document, manifest: manifest}, nil
}

type preservedManifestIdentity struct {
	Source   sourceManifest `json:"source"`
	Snapshot struct {
		Path      string `json:"path"`
		Bytes     int    `json:"bytes"`
		RawSHA256 string `json:"rawSha256"`
	} `json:"snapshot"`
}

func decodeManifestProvenance(raw []byte) (preservedManifestIdentity, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var envelope preservedManifestIdentity
	if err := decoder.Decode(&envelope); err != nil {
		return preservedManifestIdentity{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return preservedManifestIdentity{}, errors.New("manifest has a trailing JSON value")
		}
		return preservedManifestIdentity{}, err
	}
	return envelope, nil
}

func decodeManifest(raw []byte) (dataManifest, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var manifest dataManifest
	if err := decoder.Decode(&manifest); err != nil {
		return dataManifest{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return dataManifest{}, errors.New("manifest has a trailing JSON value")
		}
		return dataManifest{}, err
	}
	return manifest, nil
}

func validateNotice(notice []byte) error {
	normalizedNotice := strings.Join(strings.Fields(string(notice)), " ")
	required := []string{
		"https://talentsforever.com/data.json",
		"Data from talentsforever.com (https://talentsforever.com)",
		"CC BY 4.0",
		"https://creativecommons.org/licenses/by/4.0/",
		"unmodified response body",
		"not covered by the repository's MIT license",
		"Blizzard Entertainment",
		"fan-made transcription",
		"not simulator input",
	}
	for _, text := range required {
		if !strings.Contains(normalizedNotice, text) {
			return fmt.Errorf("%s must contain %q", noticeRelativePath, text)
		}
	}
	return nil
}

func writeCandidate(paths repositoryFiles, candidate dataState) error {
	if err := os.MkdirAll(filepath.Dir(paths.snapshot), 0o755); err != nil {
		return err
	}
	manifestBytes, err := marshalManifest(candidate.manifest)
	if err != nil {
		return err
	}
	snapshotTemp, err := stageFile(paths.snapshot, candidate.raw)
	if err != nil {
		return err
	}
	defer os.Remove(snapshotTemp)
	manifestTemp, err := stageFile(paths.manifest, manifestBytes)
	if err != nil {
		return err
	}
	defer os.Remove(manifestTemp)

	previousSnapshot, snapshotReadErr := os.ReadFile(paths.snapshot)
	hadSnapshot := snapshotReadErr == nil
	if snapshotReadErr != nil && !errors.Is(snapshotReadErr, fs.ErrNotExist) {
		return snapshotReadErr
	}
	if err := os.Rename(snapshotTemp, paths.snapshot); err != nil {
		return err
	}
	if err := os.Rename(manifestTemp, paths.manifest); err != nil {
		var rollbackErr error
		if hadSnapshot {
			rollbackErr = atomicWrite(paths.snapshot, previousSnapshot)
		} else {
			rollbackErr = os.Remove(paths.snapshot)
			if errors.Is(rollbackErr, fs.ErrNotExist) {
				rollbackErr = nil
			}
		}
		if rollbackErr != nil {
			return fmt.Errorf("replace manifest: %w; snapshot rollback also failed: %v", err, rollbackErr)
		}
		return fmt.Errorf("replace manifest: %w; snapshot was rolled back", err)
	}
	return nil
}

func persistCandidate(paths repositoryFiles, current, candidate dataState, missing, acceptRisky bool) (bool, []string, error) {
	if !missing && current.manifest.Snapshot.DocumentSHA256 == candidate.manifest.Snapshot.DocumentSHA256 {
		return false, nil, nil
	}
	if !missing && !acceptRisky {
		if risks := riskyChanges(current.manifest, candidate.manifest); len(risks) != 0 {
			return false, risks, nil
		}
	}
	if err := writeCandidate(paths, candidate); err != nil {
		return false, nil, err
	}
	return true, nil, nil
}

func acquireUpdateLock(paths repositoryFiles) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(paths.snapshot), 0o755); err != nil {
		return nil, err
	}
	lockPath := filepath.Join(filepath.Dir(paths.snapshot), ".foreverdata-update.lock")
	if err := os.Mkdir(lockPath, 0o700); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("another Forever data update is in progress (remove %s only if no updater is running)", lockPath)
		}
		return nil, err
	}
	return func() { _ = os.Remove(lockPath) }, nil
}

func stageFile(path string, contents []byte) (string, error) {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".foreverdata-*.tmp")
	if err != nil {
		return "", err
	}
	temporaryName := temporary.Name()
	failed := true
	defer func() {
		if failed {
			temporary.Close()
			os.Remove(temporaryName)
		}
	}()
	if err := temporary.Chmod(0o644); err != nil {
		return "", err
	}
	if _, err := temporary.Write(contents); err != nil {
		return "", err
	}
	if err := temporary.Sync(); err != nil {
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	failed = false
	return temporaryName, nil
}

func atomicWrite(path string, contents []byte) error {
	temporaryName, err := stageFile(path, contents)
	if err != nil {
		return err
	}
	defer os.Remove(temporaryName)
	return os.Rename(temporaryName, path)
}

func riskyChanges(oldManifest, newManifest dataManifest) []string {
	var risks []string
	oldDate, oldErr := time.Parse(time.DateOnly, oldManifest.Source.Generated)
	newDate, newErr := time.Parse(time.DateOnly, newManifest.Source.Generated)
	if oldErr == nil && newErr == nil && newDate.Before(oldDate) {
		risks = append(risks, fmt.Sprintf("generated date moved backwards from %s to %s", oldManifest.Source.Generated, newManifest.Source.Generated))
	}
	counts := []struct {
		name string
		old  int
		new  int
	}{
		{"talents", oldManifest.Metrics.Talents, newManifest.Metrics.Talents},
		{"talent trees", oldManifest.Metrics.TalentTrees, newManifest.Metrics.TalentTrees},
		{"spell descriptions", oldManifest.Metrics.SpellDescriptions, newManifest.Metrics.SpellDescriptions},
		{"spellbook entries", oldManifest.Metrics.SpellbookEntries, newManifest.Metrics.SpellbookEntries},
		{"racial abilities", oldManifest.Metrics.RacialAbilities, newManifest.Metrics.RacialAbilities},
		{"class racial abilities", oldManifest.Metrics.ClassRacialAbilities, newManifest.Metrics.ClassRacialAbilities},
		{"class abilities", oldManifest.Metrics.ClassAbilities, newManifest.Metrics.ClassAbilities},
		{"Legacy perks", oldManifest.Metrics.LegacyPerks, newManifest.Metrics.LegacyPerks},
	}
	for _, count := range counts {
		if count.old >= 10 && count.new*4 < count.old*3 {
			risks = append(risks, fmt.Sprintf("%s decreased by more than 25%% (%d to %d)", count.name, count.old, count.new))
		}
	}
	return risks
}

func printChangeSummary(writer io.Writer, oldState, newState dataState) {
	fmt.Fprintf(writer, "talentsforever change: generated %s -> %s; document %s -> %s; evidence %s -> %s\n",
		oldState.manifest.Source.Generated, newState.manifest.Source.Generated,
		shortHash(oldState.manifest.Snapshot.DocumentSHA256), shortHash(newState.manifest.Snapshot.DocumentSHA256),
		shortHash(oldState.manifest.Snapshot.EvidenceSHA256), shortHash(newState.manifest.Snapshot.EvidenceSHA256))

	sectionNames := make(map[string]bool)
	for name := range oldState.manifest.Sections {
		sectionNames[name] = true
	}
	for name := range newState.manifest.Sections {
		sectionNames[name] = true
	}
	sortedNames := make([]string, 0, len(sectionNames))
	for name := range sectionNames {
		sortedNames = append(sortedNames, name)
	}
	sort.Strings(sortedNames)
	for _, name := range sortedNames {
		oldSection, hadOld := oldState.manifest.Sections[name]
		newSection, hasNew := newState.manifest.Sections[name]
		switch {
		case !hadOld:
			fmt.Fprintf(writer, "  + section %s (%s, %d entries, %s)\n", strconv.Quote(name), newSection.Type, newSection.Entries, shortHash(newSection.SHA256))
		case !hasNew:
			fmt.Fprintf(writer, "  - section %s (%s, %d entries, %s)\n", strconv.Quote(name), oldSection.Type, oldSection.Entries, shortHash(oldSection.SHA256))
		case oldSection.SHA256 != newSection.SHA256:
			fmt.Fprintf(writer, "  ~ section %s (%d -> %d entries, %s -> %s)\n", strconv.Quote(name), oldSection.Entries, newSection.Entries, shortHash(oldSection.SHA256), shortHash(newSection.SHA256))
		}
	}

	if oldState.manifest.Metrics != newState.manifest.Metrics {
		fmt.Fprintf(writer, "  metrics: talents %d -> %d (complete %d -> %d, confirmed %d -> %d); spell descriptions %d -> %d (demo %d -> %d, classic %d -> %d)\n",
			oldState.manifest.Metrics.Talents, newState.manifest.Metrics.Talents,
			oldState.manifest.Metrics.CompleteTalents, newState.manifest.Metrics.CompleteTalents,
			oldState.manifest.Metrics.TalentsWithConfirmedRank, newState.manifest.Metrics.TalentsWithConfirmedRank,
			oldState.manifest.Metrics.SpellDescriptions, newState.manifest.Metrics.SpellDescriptions,
			oldState.manifest.Metrics.DemoSpellDescriptions, newState.manifest.Metrics.DemoSpellDescriptions,
			oldState.manifest.Metrics.ClassicSpellDescriptions, newState.manifest.Metrics.ClassicSpellDescriptions)
		fmt.Fprintf(writer, "  other metrics: spellbook entries %d -> %d; racial abilities %d -> %d; class racial abilities %d -> %d; class abilities %d -> %d; Legacy perks %d -> %d\n",
			oldState.manifest.Metrics.SpellbookEntries, newState.manifest.Metrics.SpellbookEntries,
			oldState.manifest.Metrics.RacialAbilities, newState.manifest.Metrics.RacialAbilities,
			oldState.manifest.Metrics.ClassRacialAbilities, newState.manifest.Metrics.ClassRacialAbilities,
			oldState.manifest.Metrics.ClassAbilities, newState.manifest.Metrics.ClassAbilities,
			oldState.manifest.Metrics.LegacyPerks, newState.manifest.Metrics.LegacyPerks)
	}

	records := changedRecordIdentities(oldState.document, newState.document)
	if len(records) != 0 {
		counts := make(map[string][3]int)
		for _, record := range records {
			category, _, _ := strings.Cut(record.identity, "/")
			value := counts[category]
			switch record.kind {
			case "+":
				value[0]++
			case "-":
				value[1]++
			case "~":
				value[2]++
			}
			counts[category] = value
		}
		categories := make([]string, 0, len(counts))
		for category := range counts {
			categories = append(categories, category)
		}
		sort.Strings(categories)
		fmt.Fprint(writer, "  record totals:")
		for _, category := range categories {
			value := counts[category]
			fmt.Fprintf(writer, " %s +%d -%d ~%d;", category, value[0], value[1], value[2])
		}
		fmt.Fprintln(writer)
	}
	for index, record := range records {
		if index == maxPrintedChanges {
			fmt.Fprintf(writer, "  ... %d additional record changes omitted\n", len(records)-maxPrintedChanges)
			break
		}
		fmt.Fprintf(writer, "  %s record %s\n", record.kind, strconv.Quote(record.identity))
	}
}

type recordChange struct {
	kind     string
	identity string
}

func changedRecordIdentities(oldDocument, newDocument map[string]any) []recordChange {
	oldRecords := indexedRecords(oldDocument)
	newRecords := indexedRecords(newDocument)
	identities := make(map[string]bool)
	for identity := range oldRecords {
		identities[identity] = true
	}
	for identity := range newRecords {
		identities[identity] = true
	}
	ordered := make([]string, 0, len(identities))
	for identity := range identities {
		ordered = append(ordered, identity)
	}
	sort.Strings(ordered)
	changes := make([]recordChange, 0)
	for _, identity := range ordered {
		oldHash, hadOld := oldRecords[identity]
		newHash, hasNew := newRecords[identity]
		switch {
		case !hadOld:
			changes = append(changes, recordChange{kind: "+", identity: identity})
		case !hasNew:
			changes = append(changes, recordChange{kind: "-", identity: identity})
		case oldHash != newHash:
			changes = append(changes, recordChange{kind: "~", identity: identity})
		}
	}
	return changes
}

func indexedRecords(document map[string]any) map[string]string {
	records := make(map[string]string)
	if talents, ok := document["talents"].(map[string]any); ok {
		for className, rawClass := range talents {
			class, _ := rawClass.(map[string]any)
			trees, _ := class["trees"].([]any)
			for _, rawTree := range trees {
				tree, _ := rawTree.(map[string]any)
				treeName, _ := tree["name"].(string)
				talents, _ := tree["talents"].([]any)
				for _, rawTalent := range talents {
					talent, _ := rawTalent.(map[string]any)
					name, _ := talent["name"].(string)
					row := scalarIdentity(talent["row"])
					column := scalarIdentity(talent["col"])
					identity := fmt.Sprintf("talent/%s/%s/%s,%s/%s", className, treeName, row, column, name)
					if encoded, err := canonicalJSON(talent); err == nil {
						records[identity] = sha256Hex(encoded)
					}
				}
			}
		}
	}
	if descriptions, ok := document["spell_desc"].(map[string]any); ok {
		for key, value := range descriptions {
			indexRecord(records, "spell_desc/"+key, value)
		}
	}
	if spellbooks, ok := document["spellbooks"].(map[string]any); ok {
		for className, rawSpellbook := range spellbooks {
			spellbook, _ := rawSpellbook.(map[string]any)
			indexNamedTuples(records, "spellbook/"+className+"/General", spellbook["general"])
			tabs, _ := spellbook["tabs"].([]any)
			for _, rawTab := range tabs {
				tab, _ := rawTab.(map[string]any)
				tabName, _ := tab["name"].(string)
				indexNamedTuples(records, "spellbook/"+className+"/"+tabName, tab["spells"])
			}
		}
	}
	if factions, ok := document["racials"].(map[string]any); ok {
		for factionName, rawRaces := range factions {
			races, _ := rawRaces.([]any)
			for _, rawRace := range races {
				race, _ := rawRace.(map[string]any)
				raceName, _ := race["race"].(string)
				indexNamedTuples(records, "racial/"+factionName+"/"+raceName, race["abilities"])
			}
		}
	}
	if classes, ok := document["class_racials"].(map[string]any); ok {
		for className, rawClass := range classes {
			class, _ := rawClass.(map[string]any)
			races, _ := class["races"].(map[string]any)
			for raceName, abilities := range races {
				indexNamedTuples(records, "class_racial/"+className+"/"+raceName, abilities)
			}
		}
	}
	if classes, ok := document["class_abilities"].(map[string]any); ok {
		for className, abilities := range classes {
			indexNamedTuples(records, "class_ability/"+className, abilities)
		}
	}
	if legacy, ok := document["legacy"].(map[string]any); ok {
		trees, _ := legacy["trees"].([]any)
		for _, rawTree := range trees {
			tree, _ := rawTree.(map[string]any)
			treeName, _ := tree["name"].(string)
			indexNamedTuples(records, "legacy/"+treeName, tree["perks"])
		}
	}
	return records
}

func indexNamedTuples(records map[string]string, prefix string, rawRecords any) {
	tuples, _ := rawRecords.([]any)
	for index, rawTuple := range tuples {
		tuple, _ := rawTuple.([]any)
		name := "?"
		if len(tuple) != 0 {
			if value, ok := tuple[0].(string); ok && value != "" {
				name = value
			}
		}
		identity := prefix + "/" + name
		if _, collision := records[identity]; collision {
			identity += fmt.Sprintf("#%d", index)
		}
		indexRecord(records, identity, rawTuple)
	}
}

func indexRecord(records map[string]string, identity string, value any) {
	if encoded, err := canonicalJSON(value); err == nil {
		records[identity] = sha256Hex(encoded)
	}
}

func scalarIdentity(value any) string {
	switch value := value.(type) {
	case string:
		return value
	case json.Number:
		normalized, err := normalizeJSONNumber(value.String())
		if err != nil {
			return "?"
		}
		return normalized
	default:
		return "?"
	}
}

func normalizeJSONNumbers(value any) (any, error) {
	switch value := value.(type) {
	case map[string]any:
		normalized := make(map[string]any, len(value))
		for key, child := range value {
			normalizedChild, err := normalizeJSONNumbers(child)
			if err != nil {
				return nil, err
			}
			normalized[key] = normalizedChild
		}
		return normalized, nil
	case []any:
		normalized := make([]any, len(value))
		for index, child := range value {
			normalizedChild, err := normalizeJSONNumbers(child)
			if err != nil {
				return nil, err
			}
			normalized[index] = normalizedChild
		}
		return normalized, nil
	case json.Number:
		normalized, err := normalizeJSONNumber(value.String())
		if err != nil {
			return nil, err
		}
		return json.Number(normalized), nil
	default:
		return value, nil
	}
}

// normalizeJSONNumber produces an exact, precision-preserving representation.
// It intentionally does not use float64: source IDs and future values may be
// larger or more precise than IEEE-754 can represent.
func normalizeJSONNumber(number string) (string, error) {
	original := number
	negative := strings.HasPrefix(number, "-")
	if negative {
		number = strings.TrimPrefix(number, "-")
	}

	exponent := new(big.Int)
	if index := strings.IndexAny(number, "eE"); index >= 0 {
		exponentText := number[index+1:]
		if _, ok := exponent.SetString(strings.TrimPrefix(exponentText, "+"), 10); !ok {
			return "", fmt.Errorf("invalid JSON number %q", original)
		}
		number = number[:index]
	}

	fractionDigits := 0
	if index := strings.IndexByte(number, '.'); index >= 0 {
		fractionDigits = len(number) - index - 1
		number = number[:index] + number[index+1:]
	}
	digits := strings.TrimLeft(number, "0")
	if digits == "" {
		return "0", nil
	}
	exponent.Sub(exponent, big.NewInt(int64(fractionDigits)))
	for strings.HasSuffix(digits, "0") {
		digits = strings.TrimSuffix(digits, "0")
		exponent.Add(exponent, big.NewInt(1))
	}
	if negative {
		digits = "-" + digits
	}
	if exponent.Sign() == 0 {
		return digits, nil
	}
	return digits + "e" + exponent.String(), nil
}

func jsonType(value any) string {
	switch value.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case json.Number:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}

func entryCount(value any) int {
	switch value := value.(type) {
	case map[string]any:
		return len(value)
	case []any:
		return len(value)
	default:
		return 1
	}
}

func sha256Hex(contents []byte) string {
	hash := sha256.Sum256(contents)
	return hex.EncodeToString(hash[:])
}

func shortHash(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12]
}
