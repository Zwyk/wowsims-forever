package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDecodeDocumentRejectsAmbiguousJSON(t *testing.T) {
	deep := strings.Repeat("[", maxJSONDepth+2) + "0" + strings.Repeat("]", maxJSONDepth+2)
	tests := []struct {
		name string
		raw  []byte
		want string
	}{
		{name: "duplicate key", raw: []byte(`{"a":1,"a":2}`), want: "duplicate object key"},
		{name: "nested duplicate key", raw: []byte(`{"a":{"b":1,"b":2}}`), want: "duplicate object key"},
		{name: "trailing value", raw: []byte(`{} []`), want: "trailing token"},
		{name: "invalid UTF-8", raw: []byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'}, want: "valid UTF-8"},
		{name: "excessive nesting", raw: []byte(deep), want: "nesting exceeds"},
		{name: "non-object root", raw: []byte(`[]`), want: "top-level JSON value"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := decodeDocument(test.raw)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("decodeDocument() error = %v, want text %q", err, test.want)
			}
		})
	}
}

func TestCanonicalJSONIsStableAndPreservesValues(t *testing.T) {
	first, err := decodeDocument([]byte(`{"z":{"html":"<b>x</b>","number":1.00},"array":[2,1],"_readme":"x","license":"x","attribution":"x","generated":"2026-09-15","talents":{},"spellbooks":{},"spell_desc":{},"racials":{},"class_racials":{},"class_abilities":{},"legacy":{},"changelog":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	second, err := decodeDocument([]byte("{\n\t\"array\": [2, 1], \"z\": {\"number\": 1.00, \"html\": \"<b>x</b>\"}, \"legacy\":{},\"class_abilities\":{},\"class_racials\":{},\"racials\":{},\"spell_desc\":{},\"spellbooks\":{},\"talents\":{},\"generated\":\"2026-09-15\",\"attribution\":\"x\",\"license\":\"x\",\"_readme\":\"x\",\"changelog\":[]}"))
	if err != nil {
		t.Fatal(err)
	}
	firstCanonical, err := canonicalJSON(first)
	if err != nil {
		t.Fatal(err)
	}
	secondCanonical, err := canonicalJSON(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstCanonical, secondCanonical) {
		t.Fatalf("canonical JSON differs:\n%s\n%s", firstCanonical, secondCanonical)
	}
	if !bytes.Contains(firstCanonical, []byte(`"html":"<b>x</b>"`)) {
		t.Fatalf("HTML was escaped in canonical JSON: %s", firstCanonical)
	}
	if !bytes.Contains(firstCanonical, []byte(`"number":1`)) {
		t.Fatalf("number was not normalized exactly in canonical JSON: %s", firstCanonical)
	}
	if got := sha256Hex([]byte(`{"a":[2,1],"b":1.00}`)); got != "c45efb8141dc299a0a013365b048c8495850ec0fd8f046b2b0e59e60a25da16b" {
		t.Fatalf("known SHA-256 = %s", got)
	}
	third := mapsClone(first)
	third["array"] = []any{json.Number("1"), json.Number("2")}
	thirdCanonical, err := canonicalJSON(third)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(firstCanonical, thirdCanonical) {
		t.Fatal("array reordering did not change canonical JSON")
	}

	for _, equivalent := range []string{"1", "1.0", "1.00e+0", "10e-1"} {
		document, err := decodeDocument([]byte(`{"value":` + equivalent + `}`))
		if err != nil {
			t.Fatal(err)
		}
		canonical, err := canonicalJSON(document)
		if err != nil {
			t.Fatal(err)
		}
		if string(canonical) != `{"value":1}` {
			t.Fatalf("canonical number %s = %s", equivalent, canonical)
		}
	}
	emptyArray, _ := decodeDocument([]byte(`{"value":[]}`))
	nullValue, _ := decodeDocument([]byte(`{"value":null}`))
	emptyCanonical, _ := canonicalJSON(emptyArray)
	nullCanonical, _ := canonicalJSON(nullValue)
	if bytes.Equal(emptyCanonical, nullCanonical) || string(emptyCanonical) != `{"value":[]}` {
		t.Fatalf("empty array collided with null: %s vs %s", emptyCanonical, nullCanonical)
	}
}

func TestValidateDocumentEnvelope(t *testing.T) {
	valid := validTestDocument()
	if err := validateDocument(valid); err != nil {
		t.Fatalf("valid document rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{name: "license change", mutate: func(document map[string]any) { document["license"] = "MIT" }, want: "manual license review"},
		{name: "attribution change", mutate: func(document map[string]any) { document["attribution"] = "unknown" }, want: "manual attribution review"},
		{name: "bad date", mutate: func(document map[string]any) { document["generated"] = "September 15" }, want: "YYYY-MM-DD"},
		{name: "missing section", mutate: func(document map[string]any) { delete(document, "legacy") }, want: "legacy must be an object"},
		{name: "changed section type", mutate: func(document map[string]any) { document["spell_desc"] = []any{} }, want: "spell_desc must be an object"},
		{name: "missing class", mutate: func(document map[string]any) { delete(document["talents"].(map[string]any), "Mage") }, want: "talents.Mage"},
		{name: "missing trees", mutate: func(document map[string]any) { document["talents"].(map[string]any)["Mage"] = map[string]any{} }, want: "talents.Mage.trees"},
		{name: "corrupt talent record", mutate: func(document map[string]any) {
			class := document["talents"].(map[string]any)["Druid"].(map[string]any)
			tree := class["trees"].([]any)[0].(map[string]any)
			tree["talents"].([]any)[0] = nil
		}, want: "must be an object"},
		{name: "invalid confirmed rank", mutate: func(document map[string]any) {
			class := document["talents"].(map[string]any)["Druid"].(map[string]any)
			tree := class["trees"].([]any)[0].(map[string]any)
			tree["talents"].([]any)[0].(map[string]any)["confirmed"] = []any{"not-a-rank"}
		}, want: "must be a positive integer"},
		{name: "confirmed rank exceeds max", mutate: func(document map[string]any) {
			class := document["talents"].(map[string]any)["Druid"].(map[string]any)
			tree := class["trees"].([]any)[0].(map[string]any)
			tree["talents"].([]any)[0].(map[string]any)["confirmed"] = []any{json.Number("2")}
		}, want: "greater than max rank"},
		{name: "missing classic status", mutate: func(document map[string]any) {
			class := document["talents"].(map[string]any)["Druid"].(map[string]any)
			tree := class["trees"].([]any)[0].(map[string]any)
			tree["talents"].([]any)[0].(map[string]any)["classic"] = map[string]any{}
		}, want: "classic.status"},
		{name: "corrupt spell description", mutate: func(document map[string]any) {
			document["spell_desc"].(map[string]any)["Druid|Example|1"] = nil
		}, want: "must be an object"},
		{name: "invalid spell ID", mutate: func(document map[string]any) {
			document["spell_desc"].(map[string]any)["Druid|Example|1"].(map[string]any)["id"] = json.Number("-1")
		}, want: "positive integer or null"},
		{name: "corrupt spellbook tuple", mutate: func(document map[string]any) {
			spellbook := document["spellbooks"].(map[string]any)["Druid"].(map[string]any)
			spellbook["general"] = []any{[]any{"Only one value"}}
		}, want: "exactly 2 strings"},
		{name: "corrupt racial tuple", mutate: func(document map[string]any) {
			document["racials"].(map[string]any)["Alliance"] = []any{map[string]any{
				"race": "Human", "classes": []any{"Warrior"}, "abilities": []any{nil},
			}}
		}, want: "must be an array"},
		{name: "missing class ability family", mutate: func(document map[string]any) {
			delete(document["class_abilities"].(map[string]any), "Warrior")
		}, want: "class_abilities.Warrior"},
		{name: "corrupt changelog", mutate: func(document map[string]any) {
			document["changelog"] = []any{map[string]any{"date": "soon", "title": "x", "text": "x"}}
		}, want: "not YYYY-MM-DD"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := validTestDocument()
			test.mutate(document)
			err := validateDocument(document)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateDocument() error = %v, want text %q", err, test.want)
			}
		})
	}

	withUnknown := validTestDocument()
	withUnknown["future_section"] = map[string]any{"preserved": true}
	if err := validateDocument(withUnknown); err != nil {
		t.Fatalf("unknown field should be preserved, got %v", err)
	}
}

func TestPositiveIntegerAcceptsEquivalentJSONNumbers(t *testing.T) {
	for _, value := range []json.Number{"1", "1.0", "10e-1", "0.01e2"} {
		integer, err := positiveIntegerAt(value, "value")
		if err != nil || integer != 1 {
			t.Fatalf("positiveIntegerAt(%q) = %d, %v", value, integer, err)
		}
	}
	for _, value := range []json.Number{"0", "-1", "1.5", "1e100"} {
		if _, err := positiveIntegerAt(value, "value"); err == nil {
			t.Fatalf("positiveIntegerAt(%q) succeeded", value)
		}
	}
}

func TestValidationErrorsEscapeUntrustedKeys(t *testing.T) {
	document := validTestDocument()
	document["talents"].(map[string]any)["hostile\n\x1b[31m::error::"] = nil
	err := validateDocument(document)
	if err == nil {
		t.Fatal("hostile class unexpectedly validated")
	}
	if strings.Contains(err.Error(), "hostile\n") || strings.ContainsRune(err.Error(), '\x1b') {
		t.Fatalf("validation error contains raw control characters: %q", err)
	}
	if !strings.Contains(err.Error(), `hostile\n\x1b[31m::error::`) {
		t.Fatalf("validation error did not retain an escaped identity: %q", err)
	}
}

func TestFetchSourceHTTPPolicy(t *testing.T) {
	lastModified := "Tue, 15 Sep 2026 17:07:24 GMT"
	validServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q", got)
		}
		if got := request.Header.Get("User-Agent"); got != "wowsims-forever-data/1" {
			t.Errorf("User-Agent = %q", got)
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.Header().Set("Last-Modified", lastModified)
		fmt.Fprint(writer, `{}`)
	}))
	defer validServer.Close()

	fetched, err := fetchSource(context.Background(), validServer.URL, defaultHTTPClient())
	if err != nil {
		t.Fatal(err)
	}
	if string(fetched.body) != `{}` || fetched.lastModified != lastModified {
		t.Fatalf("fetchSource() = %#v", fetched)
	}

	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    string
	}{
		{name: "non-200", handler: func(writer http.ResponseWriter, _ *http.Request) {
			http.Error(writer, "no", http.StatusServiceUnavailable)
		}, want: "503"},
		{name: "redirect", handler: func(writer http.ResponseWriter, request *http.Request) {
			http.Redirect(writer, request, validServer.URL, http.StatusFound)
		}, want: "302"},
		{name: "wrong content type", handler: func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "text/html")
			fmt.Fprint(writer, `{}`)
		}, want: "Content-Type"},
		{name: "oversized body", handler: func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusOK)
			writer.(http.Flusher).Flush()
			fmt.Fprint(writer, strings.Repeat(" ", maxSourceBytes+1))
		}, want: "exceeds"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(test.handler)
			defer server.Close()
			_, err := fetchSource(context.Background(), server.URL, defaultHTTPClient())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("fetchSource() error = %v, want text %q", err, test.want)
			}
		})
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fetchSource(cancelled, validServer.URL, defaultHTTPClient()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled fetch error = %v", err)
	}
}

func TestCheckCLIExitPolicy(t *testing.T) {
	root, err := findRepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	committed := repositoryPaths(root)
	var stdout, stderr bytes.Buffer
	fixedNow := func() time.Time { return time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC) }

	if code := runCLI(context.Background(), []string{"check", "--source", committed.snapshot}, &stdout, &stderr, fixedNow); code != 0 {
		t.Fatalf("no-change check code = %d, stderr = %s", code, &stderr)
	}
	if !strings.Contains(stdout.String(), "up to date") {
		t.Fatalf("no-change output = %q", &stdout)
	}

	changedPath := filepath.Join(t.TempDir(), "changed.json")
	if err := os.WriteFile(changedPath, rawTestDocument(t, validTestDocument()), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runCLI(context.Background(), []string{"check", "--source", changedPath}, &stdout, &stderr, fixedNow); code != 0 {
		t.Fatalf("default changed check code = %d, stderr = %s", code, &stderr)
	}
	if !strings.Contains(stdout.String(), "talentsforever change") {
		t.Fatalf("changed output = %q", &stdout)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runCLI(context.Background(), []string{"check", "--source", changedPath, "--fail-on-change"}, &stdout, &stderr, fixedNow); code != 2 {
		t.Fatalf("fail-on-change code = %d, stderr = %s", code, &stderr)
	}
	if code := runCLI(context.Background(), []string{"check", "--accept-risky-change"}, &stdout, &stderr, fixedNow); code != 1 {
		t.Fatalf("invalid check option code = %d", code)
	}
	if code := runCLI(context.Background(), []string{"update", "--source", changedPath}, &stdout, &stderr, fixedNow); code != 1 {
		t.Fatalf("local update source code = %d", code)
	}
}

func TestManifestRoundTripAndTamperDetection(t *testing.T) {
	paths := testRepositoryFiles(t)
	candidate := testCandidate(t, validTestDocument(), time.Date(2026, 9, 15, 18, 0, 1, 0, time.UTC))
	if err := writeCandidate(paths, candidate); err != nil {
		t.Fatal(err)
	}

	state, err := readCommittedState(paths)
	if err != nil {
		t.Fatal(err)
	}
	if state.manifest.SchemaVersion != manifestSchema || state.manifest.CanonicalizerVersion != canonicalizerVersion {
		t.Fatalf("unexpected manifest versions: %#v", state.manifest)
	}
	if state.manifest.EvidenceTier != evidenceTier || state.manifest.RuntimeStatus != runtimeStatus {
		t.Fatalf("unexpected evidence boundary: %#v", state.manifest)
	}
	if state.manifest.Metrics.Classes != len(requiredClasses) || state.manifest.Metrics.Talents != len(requiredClasses) {
		t.Fatalf("unexpected metrics: %#v", state.manifest.Metrics)
	}
	if state.manifest.Snapshot.RawSHA256 != sha256Hex(candidate.raw) {
		t.Fatal("raw hash does not match")
	}

	tampered := mapsClone(validTestDocument())
	tampered["generated"] = "2026-09-16"
	if err := os.WriteFile(paths.snapshot, rawTestDocument(t, tampered), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readCommittedState(paths); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("tampered snapshot error = %v", err)
	}
}

func TestRebuildManifestMigratesDerivedFormatWithoutBlessingTampering(t *testing.T) {
	paths := testRepositoryFiles(t)
	candidate := testCandidate(t, validTestDocument(), time.Date(2026, 9, 15, 18, 0, 1, 0, time.UTC))
	if err := writeCandidate(paths, candidate); err != nil {
		t.Fatal(err)
	}
	originalSnapshot := append([]byte(nil), candidate.raw...)

	manifest, err := os.ReadFile(paths.manifest)
	if err != nil {
		t.Fatal(err)
	}
	legacyManifest := bytes.Replace(manifest, []byte(`"schemaVersion": 1`), []byte(`"schemaVersion": 0`), 1)
	if bytes.Equal(legacyManifest, manifest) {
		t.Fatal("test did not alter manifest schema")
	}
	if err := os.WriteFile(paths.manifest, legacyManifest, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readCommittedState(paths); err == nil {
		t.Fatal("legacy manifest unexpectedly verified")
	}
	if _, err := rebuildCommittedManifest(paths); err != nil {
		t.Fatalf("rebuildCommittedManifest() failed: %v", err)
	}
	if _, err := readCommittedState(paths); err != nil {
		t.Fatalf("rebuilt manifest did not verify: %v", err)
	}
	if got, err := os.ReadFile(paths.snapshot); err != nil || !bytes.Equal(got, originalSnapshot) {
		t.Fatalf("manifest migration changed snapshot: equal %v, err %v", bytes.Equal(got, originalSnapshot), err)
	}

	tampered := validTestDocument()
	tampered["future_section"] = map[string]any{"same_generated_date": true}
	if err := os.WriteFile(paths.snapshot, rawTestDocument(t, tampered), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := rebuildCommittedManifest(paths); err == nil || !strings.Contains(err.Error(), "refusing to bless possible tampering") {
		t.Fatalf("tampered rebuild error = %v", err)
	}
}

func TestUpdateLockIsExclusive(t *testing.T) {
	paths := testRepositoryFiles(t)
	release, err := acquireUpdateLock(paths)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := acquireUpdateLock(paths); err == nil || !strings.Contains(err.Error(), "another Forever data update") {
		t.Fatalf("second lock error = %v", err)
	}
	release()
	releaseAgain, err := acquireUpdateLock(paths)
	if err != nil {
		t.Fatalf("lock was not released: %v", err)
	}
	releaseAgain()
}

func TestWriteCandidateRollsBackSnapshotIfManifestReplacementFails(t *testing.T) {
	paths := testRepositoryFiles(t)
	current := testCandidate(t, validTestDocument(), time.Date(2026, 9, 15, 18, 0, 0, 0, time.UTC))
	if err := writeCandidate(paths, current); err != nil {
		t.Fatal(err)
	}
	changedDocument := validTestDocument()
	changedDocument["future_section"] = map[string]any{"value": "changed"}
	changed := testCandidate(t, changedDocument, time.Date(2026, 9, 16, 18, 0, 0, 0, time.UTC))

	brokenPaths := paths
	brokenPaths.manifest = filepath.Join(filepath.Dir(paths.manifest), "manifest-target-is-a-directory")
	if err := os.Mkdir(brokenPaths.manifest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeCandidate(brokenPaths, changed); err == nil || !strings.Contains(err.Error(), "snapshot was rolled back") {
		t.Fatalf("writeCandidate() error = %v", err)
	}
	got, err := os.ReadFile(paths.snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, current.raw) {
		t.Fatal("snapshot was not restored after manifest replacement failure")
	}
	if _, err := readCommittedState(paths); err != nil {
		t.Fatalf("original snapshot/manifest pair no longer verifies: %v", err)
	}
}

func TestPersistCandidateNoOpAndRiskGuard(t *testing.T) {
	paths := testRepositoryFiles(t)
	current := testCandidate(t, validTestDocument(), time.Date(2026, 9, 15, 18, 0, 0, 0, time.UTC))
	updated, risks, err := persistCandidate(paths, dataState{}, current, true, false)
	if err != nil || len(risks) != 0 || !updated {
		t.Fatalf("initial persist = updated %v, risks %v, err %v", updated, risks, err)
	}
	snapshotInfo, err := os.Stat(paths.snapshot)
	if err != nil {
		t.Fatal(err)
	}
	manifestInfo, err := os.Stat(paths.manifest)
	if err != nil {
		t.Fatal(err)
	}

	equivalentRaw := append([]byte(" \r\n"), current.raw...)
	equivalent, err := makeCandidate(fetchedSource{body: equivalentRaw}, sourceURL, time.Date(2026, 9, 16, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	updated, risks, err = persistCandidate(paths, current, equivalent, false, false)
	if err != nil || len(risks) != 0 || updated {
		t.Fatalf("no-op persist = updated %v, risks %v, err %v", updated, risks, err)
	}
	newSnapshotInfo, _ := os.Stat(paths.snapshot)
	newManifestInfo, _ := os.Stat(paths.manifest)
	if !snapshotInfo.ModTime().Equal(newSnapshotInfo.ModTime()) || !manifestInfo.ModTime().Equal(newManifestInfo.ModTime()) {
		t.Fatal("no-op update rewrote committed files")
	}
	numericDocument := validTestDocument()
	druid := numericDocument["talents"].(map[string]any)["Druid"].(map[string]any)
	druidTree := druid["trees"].([]any)[0].(map[string]any)
	druidTree["talents"].([]any)[0].(map[string]any)["row"] = json.Number("1.0")
	numericEquivalent := testCandidate(t, numericDocument, time.Date(2026, 9, 16, 1, 30, 0, 0, time.UTC))
	if numericEquivalent.manifest.Snapshot.DocumentSHA256 != current.manifest.Snapshot.DocumentSHA256 {
		t.Fatal("equivalent JSON number lexemes changed the document hash")
	}

	shrunkDocument := validTestDocument()
	spellDescriptions := make(map[string]any)
	for index := 0; index < 100; index++ {
		spellDescriptions[fmt.Sprintf("spell-%03d", index)] = testSpellDescription()
	}
	large := mapsClone(validTestDocument())
	large["spell_desc"] = spellDescriptions
	largeCandidate := testCandidate(t, large, time.Date(2026, 9, 16, 2, 0, 0, 0, time.UTC))
	if err := writeCandidate(paths, largeCandidate); err != nil {
		t.Fatal(err)
	}
	shrunkDocument["spell_desc"] = map[string]any{"one": testSpellDescription()}
	shrunk := testCandidate(t, shrunkDocument, time.Date(2026, 9, 17, 2, 0, 0, 0, time.UTC))
	updated, risks, err = persistCandidate(paths, largeCandidate, shrunk, false, false)
	if err != nil || updated || len(risks) == 0 || !strings.Contains(strings.Join(risks, " "), "spell descriptions") {
		t.Fatalf("risky persist = updated %v, risks %v, err %v", updated, risks, err)
	}
	state, err := readCommittedState(paths)
	if err != nil {
		t.Fatal(err)
	}
	if state.manifest.Snapshot.DocumentSHA256 != largeCandidate.manifest.Snapshot.DocumentSHA256 {
		t.Fatal("rejected update changed committed state")
	}
	updated, risks, err = persistCandidate(paths, largeCandidate, shrunk, false, true)
	if err != nil || len(risks) != 0 || !updated {
		t.Fatalf("accepted risky persist = updated %v, risks %v, err %v", updated, risks, err)
	}
}

func TestChangeSummaryIsBoundedAndEscapesSourceText(t *testing.T) {
	oldDocument := validTestDocument()
	newDocument := validTestDocument()
	descriptions := newDocument["spell_desc"].(map[string]any)
	for index := 0; index < maxPrintedChanges+5; index++ {
		description := testSpellDescription()
		description["d"] = "not printed"
		descriptions[fmt.Sprintf("hostile\n::error::%02d", index)] = description
	}
	oldState := testCandidate(t, oldDocument, time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC))
	newState := testCandidate(t, newDocument, time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC))
	var output bytes.Buffer
	printChangeSummary(&output, oldState, newState)
	text := output.String()
	if strings.Contains(text, "hostile\n::error::") {
		t.Fatalf("source newline was emitted literally:\n%s", text)
	}
	if !strings.Contains(text, `hostile\n::error::`) {
		t.Fatalf("escaped hostile identity missing:\n%s", text)
	}
	if !strings.Contains(text, "additional record changes omitted") {
		t.Fatalf("bounded-output marker missing:\n%s", text)
	}
	if strings.Contains(text, "not printed") {
		t.Fatalf("tooltip body leaked into summary:\n%s", text)
	}
}

func TestEvidenceReferencesAreQuarantined(t *testing.T) {
	root, err := findRepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	allowed := []string{
		"tools/foreverdata/",
		"third_party/talentsforever/",
		"docs/forever_core.md",
		".github/workflows/run_tests.yml",
		".gitattributes",
		"makefile",
	}
	textExtensions := map[string]bool{
		".css": true, ".go": true, ".html": true, ".js": true, ".json": true,
		".md": true, ".mjs": true, ".mts": true, ".proto": true, ".py": true,
		".scss": true, ".sh": true, ".toml": true, ".ts": true, ".tsx": true,
		".yaml": true, ".yml": true,
	}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if entry.IsDir() {
			switch relative {
			case ".git", "binary_dist", "dist", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !textExtensions[strings.ToLower(filepath.Ext(relative))] && filepath.Base(relative) != "makefile" {
			return nil
		}
		for _, prefix := range allowed {
			if relative == prefix || strings.HasPrefix(relative, prefix) {
				return nil
			}
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, marker := range [][]byte{
			[]byte("third_party/talentsforever"),
			[]byte("talentsforever/data.json"),
			[]byte(sourceURL),
		} {
			if bytes.Contains(contents, marker) {
				return fmt.Errorf("%s references quarantined evidence outside the explicit allowlist", relative)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func validTestDocument() map[string]any {
	talents := make(map[string]any)
	spellbooks := make(map[string]any)
	for index, class := range requiredClasses {
		talent := map[string]any{
			"name":     class + " Talent",
			"row":      json.Number("1"),
			"col":      json.Number("1"),
			"max":      json.Number("1"),
			"passive":  true,
			"icon":     "test_icon",
			"complete": true,
			"desc":     []any{"Observed"},
			"classic":  map[string]any{"status": "new"},
		}
		if index == 0 {
			talent["confirmed"] = []any{json.Number("1")}
		}
		talents[class] = map[string]any{
			"trees": []any{map[string]any{
				"name":    class + " Tree",
				"talents": []any{talent},
			}},
		}
		spellbooks[class] = map[string]any{
			"race":    "Human",
			"level":   json.Number("38"),
			"seen":    "test fixture",
			"missing": []any{},
			"general": []any{},
			"tabs":    []any{},
			"notes":   []any{},
		}
	}
	classAbilities := make(map[string]any)
	for _, class := range requiredClasses {
		classAbilities[class] = []any{}
	}
	return map[string]any{
		"_readme":         "Fan-made export. License: CC BY 4.0 (https://creativecommons.org/licenses/by/4.0/).",
		"license":         sourceLicense,
		"attribution":     sourceAttribution,
		"generated":       "2026-09-15",
		"talents":         talents,
		"spellbooks":      spellbooks,
		"spell_desc":      map[string]any{"Druid|Example|1": testSpellDescription()},
		"racials":         map[string]any{"Alliance": []any{}, "Horde": []any{}},
		"class_racials":   map[string]any{},
		"class_abilities": classAbilities,
		"legacy":          map[string]any{"trees": []any{}},
		"changelog":       []any{},
	}
}

func testSpellDescription() map[string]any {
	return map[string]any{
		"l":   []any{},
		"d":   "Observed",
		"s":   "demo",
		"src": "test fixture",
		"r":   "Rank 1",
		"lv":  "Requires level 1",
		"id":  nil,
	}
}

func rawTestDocument(t *testing.T, document map[string]any) []byte {
	t.Helper()
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(document); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func testCandidate(t *testing.T, document map[string]any, retrievedAt time.Time) dataState {
	t.Helper()
	candidate, err := makeCandidate(fetchedSource{
		body:         rawTestDocument(t, document),
		lastModified: "Tue, 15 Sep 2026 17:07:24 GMT",
	}, sourceURL, retrievedAt)
	if err != nil {
		t.Fatal(err)
	}
	return candidate
}

func testRepositoryFiles(t *testing.T) repositoryFiles {
	t.Helper()
	root := t.TempDir()
	paths := repositoryPaths(root)
	if err := os.MkdirAll(filepath.Dir(paths.notice), 0o755); err != nil {
		t.Fatal(err)
	}
	notice := strings.Join([]string{
		"https://talentsforever.com/data.json",
		"Data from talentsforever.com (https://talentsforever.com)",
		"CC BY 4.0",
		"https://creativecommons.org/licenses/by/4.0/",
		"unmodified response body",
		"not covered by the repository's MIT license",
		"Blizzard Entertainment",
		"fan-made transcription",
		"not simulator input",
	}, "\n")
	if err := os.WriteFile(paths.notice, []byte(notice), 0o644); err != nil {
		t.Fatal(err)
	}
	return paths
}

func mapsClone(document map[string]any) map[string]any {
	clone := make(map[string]any, len(document))
	for key, value := range document {
		clone[key] = value
	}
	return clone
}
