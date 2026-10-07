// Package appverify runtime-verifies a generated application: it exercises
// the app's own generic HTTP API (health, schema, create, state, fire) and
// checks every observed marking against pkg/metamodel's Runtime — the same
// firing-rule oracle pkg/mcp/simulate.go uses — rather than trusting that
// "it compiled" means "it runs correctly".
//
// It knows nothing app-specific: a single-net app is a bundle of one subnet
// with an empty prefix, and a composed (bundle) app is walked by combining
// every subnet's own marking through the bundle's FlattenMap. Firing a
// transition that a link fused across subnets goes through the composition
// root's POST /fire/<transition>; everything else fires on the subnet that
// owns it.
package appverify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/codegen/golang"
	"github.com/pflow-xyz/petri-pilot/pkg/dsl"
	pilotmeta "github.com/pflow-xyz/petri-pilot/pkg/metamodel"
)

// MaxSteps bounds the reachable-state walk. A live, possibly-cyclic net
// could otherwise keep VerifyApp running forever; 20 firings is enough to
// exercise every transition of every fixture this repo ships and is cheap
// to raise later if a real model needs more.
const MaxSteps = 20

// httpTimeout bounds every individual request VerifyApp makes.
const httpTimeout = 10 * time.Second

// Mismatch records one place whose live value disagreed with the oracle's
// prediction after a firing (step -1 means the initial marking, before
// anything was fired).
type Mismatch struct {
	Step     int    `json:"step"`
	Place    string `json:"place"`
	Expected int    `json:"expected"`
	Actual   int    `json:"actual"`
}

// Report is the result of a verification run.
type Report struct {
	// Passed is true only when the health check succeeded, the initial
	// marking matched the oracle, and every fired transition's resulting
	// marking matched the oracle with zero mismatches.
	Passed bool `json:"passed"`

	HealthOK         bool `json:"health_ok"`
	SchemaOK         bool `json:"schema_ok"`
	InitialMarkingOK bool `json:"initial_marking_ok"`

	Steps            int        `json:"steps"`
	TransitionsFired []string   `json:"transitions_fired"`
	Mismatches       []Mismatch `json:"mismatches,omitempty"`

	// Errors carries human-readable diagnostics: failed requests, unexpected
	// status codes, a transition the oracle could fire but the live app
	// refused, and so on.
	Errors []string `json:"errors,omitempty"`
}

func (r *Report) fail(format string, args ...any) {
	r.Passed = false
	r.Errors = append(r.Errors, fmt.Sprintf(format, args...))
}

// VerifyApp exercises a running generated app's HTTP API and checks it
// against the canonical firing rule (pkg/metamodel.Runtime).
//
// modelSource is the model that was generated: a single-net model JSON when
// isBundle is false, or a bundle document JSON (subnets with inline models,
// links) when isBundle is true. baseURL is the app's root, e.g.
// "http://localhost:8123" — for a bundle app this is the composition root
// that mounts /health, /api/schema, /fire/<transition>, and each subnet at
// "/<subnet-id>/...".
func VerifyApp(ctx context.Context, baseURL string, modelSource []byte, isBundle bool) (*Report, error) {
	baseURL = strings.TrimSuffix(baseURL, "/")
	if isBundle {
		var b goflowmetamodel.Bundle
		if err := json.Unmarshal(modelSource, &b); err != nil {
			return nil, fmt.Errorf("appverify: invalid bundle JSON: %w", err)
		}
		return verifyBundle(ctx, baseURL, &b)
	}
	var m goflowmetamodel.Model
	if err := json.Unmarshal(modelSource, &m); err != nil {
		return nil, fmt.Errorf("appverify: invalid model JSON: %w", err)
	}
	return verifySingleNet(ctx, baseURL, &m)
}

// --- single-net ---

func verifySingleNet(ctx context.Context, baseURL string, model *goflowmetamodel.Model) (*Report, error) {
	report := &Report{Passed: true}
	client := &http.Client{Timeout: httpTimeout}

	if err := checkHealth(ctx, client, baseURL); err != nil {
		report.fail("health check: %v", err)
		return report, nil
	}
	report.HealthOK = true

	if err := checkSchema(ctx, client, baseURL); err != nil {
		report.fail("schema check: %v", err)
	} else {
		report.SchemaOK = true
	}

	root := newEntityRoot(baseURL, "", model)
	oracleSchema := pilotmeta.SchemaFromModel(model)
	oracle := pilotmeta.NewRuntime(oracleSchema)
	oracle.GuardEvaluator = dsl.NewEvaluator()

	roots := map[string]entityRoot{"": root}
	memberOf := func(transID string) []memberRef {
		return []memberRef{{subnetID: "", localID: transID}}
	}
	placeFlatID := func(_, localID string) string { return localID }

	runWalk(ctx, client, report, oracleSchema, oracle, roots, memberOf, placeFlatID, "")
	return report, nil
}

// --- bundle ---

func verifyBundle(ctx context.Context, baseURL string, b *goflowmetamodel.Bundle) (*Report, error) {
	report := &Report{Passed: true}
	client := &http.Client{Timeout: httpTimeout}

	if err := checkHealth(ctx, client, baseURL); err != nil {
		report.fail("health check: %v", err)
		return report, nil
	}
	report.HealthOK = true

	if err := checkSchema(ctx, client, baseURL); err != nil {
		report.fail("schema check: %v", err)
	} else {
		report.SchemaOK = true
	}

	flat, flatMap, err := b.FlattenWithMap()
	if err != nil {
		report.fail("flattening bundle: %v", err)
		return report, nil
	}

	oracleSchema := pilotmeta.SchemaFromModel(flat)
	oracle := pilotmeta.NewRuntime(oracleSchema)
	oracle.GuardEvaluator = dsl.NewEvaluator()

	roots := map[string]entityRoot{}
	for _, sn := range b.Subnets {
		roots[sn.ID] = newEntityRoot(baseURL+"/"+sn.ID, sn.ID, sn.Model)
	}

	// Reverse the FlattenMap once, mirroring
	// pkg/codegen/golang/bundle_context.go's buildCommands: a fused
	// transition's members come from FusedGroups; everything else is
	// looked up in the per-subnet Transition map. Never re-derive this by
	// parsing flat IDs — that only works under the default namespace
	// scheme and silently breaks under any other.
	transitionOwner := map[string]memberRef{}
	for subnet, locals := range flatMap.Transition {
		for local, flatID := range locals {
			transitionOwner[flatID] = memberRef{subnetID: subnet, localID: local}
		}
	}
	memberOf := func(flatID string) []memberRef {
		if group, ok := flatMap.FusedGroups[flatID]; ok && len(group) > 0 {
			members := make([]memberRef, 0, len(group))
			for _, ref := range group {
				subnet, local, err := splitMemberRef(ref)
				if err != nil {
					continue
				}
				members = append(members, memberRef{subnetID: subnet, localID: local})
			}
			if len(members) > 0 {
				return members
			}
		}
		if owner, ok := transitionOwner[flatID]; ok {
			return []memberRef{owner}
		}
		return nil
	}
	placeFlatID := func(subnetID, localID string) string {
		if locals, ok := flatMap.Place[subnetID]; ok {
			if flatID, ok := locals[localID]; ok {
				return flatID
			}
		}
		// No mapping: the place wasn't namespaced (identity short-circuit,
		// single subnet, no links). Fall back to its own ID.
		return localID
	}

	runWalk(ctx, client, report, oracleSchema, oracle, roots, memberOf, placeFlatID, baseURL)
	return report, nil
}

// memberRef names one subnet's participation in a (possibly fused)
// transition, or a single-net app's one and only "subnet" (subnetID == "").
type memberRef struct {
	subnetID string
	localID  string
}

func splitMemberRef(ref string) (subnet, local string, err error) {
	i := strings.IndexByte(ref, '/')
	if i < 0 {
		return "", "", fmt.Errorf("malformed member ref %q", ref)
	}
	return ref[:i], ref[i+1:], nil
}

// --- the shared walk ---

// runWalk creates one instance per root, checks the initial marking, then
// fires the oracle's own choice of enabled transition (alphabetically first,
// for reproducibility) against the live app step by step, comparing the
// resulting marking after every firing.
func runWalk(
	ctx context.Context,
	client *http.Client,
	report *Report,
	oracleSchema *pilotmeta.Schema,
	oracle *pilotmeta.Runtime,
	roots map[string]entityRoot,
	memberOf func(flatTransitionID string) []memberRef,
	placeFlatID func(subnetID, localID string) string,
	fusedBaseURL string,
) {
	ids := map[string]string{}
	// Sort subnet IDs for deterministic instance-creation order.
	subnetIDs := make([]string, 0, len(roots))
	for id := range roots {
		subnetIDs = append(subnetIDs, id)
	}
	sort.Strings(subnetIDs)

	for _, subnetID := range subnetIDs {
		root := roots[subnetID]
		id, err := createInstance(ctx, client, root)
		if err != nil {
			report.fail("create instance for %q: %v", labelFor(subnetID), err)
			return
		}
		ids[subnetID] = id
	}

	live, err := combinedMarking(ctx, client, roots, ids, placeFlatID)
	if err != nil {
		report.fail("reading initial state: %v", err)
		return
	}
	expected := oracleMarking(oracleSchema, oracle)
	if mm := diffMarking(expected, live, -1); len(mm) > 0 {
		report.Mismatches = append(report.Mismatches, mm...)
		report.fail("initial marking does not match the model's own initial marking")
		return
	}
	report.InitialMarkingOK = true

	for step := 0; step < MaxSteps; step++ {
		enabled := oracle.EnabledActions()
		if len(enabled) == 0 {
			break
		}
		sort.Strings(enabled)
		transID := enabled[0]

		members := memberOf(transID)
		if len(members) == 0 {
			report.fail("step %d: transition %q is enabled but owns no subnet (bundle FlattenMap gap)", step, transID)
			return
		}

		if len(members) > 1 {
			if err := fireFused(ctx, client, fusedBaseURL, transID, members, ids); err != nil {
				report.fail("step %d: fire fused transition %q: %v", step, transID, err)
				return
			}
		} else {
			m := members[0]
			root, ok := roots[m.subnetID]
			if !ok {
				report.fail("step %d: transition %q references unknown subnet %q", step, transID, m.subnetID)
				return
			}
			if err := fireLocal(ctx, client, root, m.localID, ids[m.subnetID]); err != nil {
				report.fail("step %d: fire %q on %q: %v", step, m.localID, labelFor(m.subnetID), err)
				return
			}
		}

		if err := oracle.ExecuteWithBindings(transID, pilotmeta.Bindings{}); err != nil {
			// The live app just fired this; the oracle refusing it means the
			// two disagree about the firing rule, which is exactly the bug
			// this package exists to catch.
			report.fail("step %d: oracle refused %q after the live app fired it: %v", step, transID, err)
			return
		}
		report.TransitionsFired = append(report.TransitionsFired, transID)
		report.Steps++

		live, err := combinedMarking(ctx, client, roots, ids, placeFlatID)
		if err != nil {
			report.fail("step %d: reading state after firing %q: %v", step, transID, err)
			return
		}
		expected := oracleMarking(oracleSchema, oracle)
		if mm := diffMarking(expected, live, step); len(mm) > 0 {
			report.Mismatches = append(report.Mismatches, mm...)
			report.fail("step %d: marking mismatch after firing %q", step, transID)
			return
		}
	}
}

func labelFor(subnetID string) string {
	if subnetID == "" {
		return "(root)"
	}
	return subnetID
}

func oracleMarking(schema *pilotmeta.Schema, rt *pilotmeta.Runtime) map[string]int {
	m := make(map[string]int, len(schema.States))
	for _, s := range schema.States {
		if s.IsToken() {
			m[s.ID] = rt.Tokens(s.ID)
		}
	}
	return m
}

func diffMarking(expected, actual map[string]int, step int) []Mismatch {
	keys := make([]string, 0, len(expected))
	for k := range expected {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var mm []Mismatch
	for _, k := range keys {
		if actual[k] != expected[k] {
			mm = append(mm, Mismatch{Step: step, Place: k, Expected: expected[k], Actual: actual[k]})
		}
	}
	return mm
}

func combinedMarking(ctx context.Context, client *http.Client, roots map[string]entityRoot, ids map[string]string, placeFlatID func(subnetID, localID string) string) (map[string]int, error) {
	combined := map[string]int{}
	for subnetID, root := range roots {
		places, err := getPlaces(ctx, client, root, ids[subnetID])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", labelFor(subnetID), err)
		}
		for localID, v := range places {
			combined[placeFlatID(subnetID, localID)] += v
		}
	}
	return combined, nil
}

// --- entity roots: where one resource's generic API lives ---

// entityRoot is one subnet's (or a single-net app's) slice of the generic
// API: baseURL + "/api/<slug>" to create/read, baseURL + <path> per local
// transition to fire.
type entityRoot struct {
	baseURL    string
	apiSlug    string
	transition map[string]routeInfo // local transition ID -> route
}

type routeInfo struct {
	method string
	path   string
}

func newEntityRoot(baseURL, _ string, model *goflowmetamodel.Model) entityRoot {
	er := entityRoot{
		baseURL:    baseURL,
		apiSlug:    golang.SanitizeAPISlug(model.Name),
		transition: map[string]routeInfo{},
	}
	for _, r := range goflowmetamodel.InferAPIRoutes(model) {
		er.transition[r.TransitionID] = routeInfo{method: r.Method, path: r.Path}
	}
	return er
}

// --- HTTP calls against the generic API ---

type stateResponse struct {
	AggregateID        string         `json:"aggregate_id"`
	Version            int            `json:"version"`
	Places             map[string]int `json:"places"`
	EnabledTransitions []string       `json:"enabled_transitions"`
}

func checkHealth(ctx context.Context, client *http.Client, baseURL string) error {
	resp, err := doRequest(ctx, client, http.MethodGet, baseURL+"/health", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET /health: status %d", resp.StatusCode)
	}
	return nil
}

func checkSchema(ctx context.Context, client *http.Client, baseURL string) error {
	resp, err := doRequest(ctx, client, http.MethodGet, baseURL+"/api/schema", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET /api/schema: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading schema body: %w", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("schema is not valid JSON: %w", err)
	}
	if _, ok := parsed["places"]; !ok {
		return fmt.Errorf("schema JSON has no \"places\" field")
	}
	if _, ok := parsed["transitions"]; !ok {
		return fmt.Errorf("schema JSON has no \"transitions\" field")
	}
	return nil
}

func createInstance(ctx context.Context, client *http.Client, root entityRoot) (string, error) {
	url := root.baseURL + "/api/" + root.apiSlug
	resp, err := doRequest(ctx, client, http.MethodPost, url, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("POST %s: status %d: %s", url, resp.StatusCode, string(body))
	}
	var sr stateResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return "", fmt.Errorf("decoding create response from %s: %w", url, err)
	}
	if sr.AggregateID == "" {
		return "", fmt.Errorf("POST %s: response carried no aggregate_id", url)
	}
	return sr.AggregateID, nil
}

func getPlaces(ctx context.Context, client *http.Client, root entityRoot, id string) (map[string]int, error) {
	url := root.baseURL + "/api/" + root.apiSlug + "/" + id
	resp, err := doRequest(ctx, client, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET %s: status %d: %s", url, resp.StatusCode, string(body))
	}
	var sr stateResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decoding state response from %s: %w", url, err)
	}
	return sr.Places, nil
}

func fireLocal(ctx context.Context, client *http.Client, root entityRoot, transitionID, aggregateID string) error {
	route, ok := root.transition[transitionID]
	path := "/api/" + transitionID
	if ok && route.path != "" {
		path = route.path
	}
	url := root.baseURL + path

	payload, _ := json.Marshal(map[string]string{"aggregate_id": aggregateID})
	resp, err := doRequest(ctx, client, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST %s: status %d: %s", url, resp.StatusCode, string(body))
	}
	return nil
}

func fireFused(ctx context.Context, client *http.Client, fusedBaseURL, transitionID string, members []memberRef, ids map[string]string) error {
	idMap := make(map[string]string, len(members))
	for _, m := range members {
		id, ok := ids[m.subnetID]
		if !ok {
			return fmt.Errorf("no instance created for subnet %q", m.subnetID)
		}
		idMap[m.subnetID] = id
	}
	url := fusedBaseURL + "/fire/" + transitionID
	payload, _ := json.Marshal(map[string]any{"ids": idMap})
	resp, err := doRequest(ctx, client, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST %s: status %d: %s", url, resp.StatusCode, string(body))
	}
	return nil
}

func doRequest(ctx context.Context, client *http.Client, method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
