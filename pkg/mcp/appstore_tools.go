package mcp

// Tools over the app store (pkg/appstore): inspecting a spec's lineage, and
// naming a spec as "an app" so it can be found again and rebuilt.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pflow-xyz/petri-pilot/pkg/appstore"
)

func historyTool() mcp.Tool {
	return mcp.NewTool("petri_history",
		mcp.WithDescription("Return the full lineage of a spec stored in the app store: the chain of prompts, edits and builds that produced it, root first. Each entry names the tool activity ('petri_extend', 'petri_build', ...) that touched that spec, the prompt (if any) that motivated it, and a short outcome note."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Spec id to show history for (as returned by petri_extend or petri_build)."),
		),
	)
}

func handleHistory(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := request.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing id parameter: %v", err)), nil
	}

	store, err := getAppStore()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("opening app store: %v", err)), nil
	}

	history, err := store.History(id)
	if err != nil {
		if errors.Is(err, appstore.ErrNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("unknown spec id %q", id)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("loading history: %v", err)), nil
	}

	result := struct {
		ID      string                  `json:"id"`
		History []appstore.LineageEntry `json:"history"`
	}{ID: id, History: history}

	outputJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshaling result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(outputJSON)), nil
}

func appSaveTool() mcp.Tool {
	return mcp.NewTool("petri_app_save",
		mcp.WithDescription("Name a spec stored in the app store, so it can be found again with petri_app_get and rebuilt with petri_build(id=...). Recorded as name -> spec id; re-saving an existing name moves it to point at a new spec id."),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Human-given name for this app."),
		),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Spec id to name (must already exist in the app store)."),
		),
	)
}

func handleAppSave(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing name parameter: %v", err)), nil
	}
	id, err := request.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing id parameter: %v", err)), nil
	}

	store, err := getAppStore()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("opening app store: %v", err)), nil
	}

	if err := store.SaveApp(name, id); err != nil {
		if errors.Is(err, appstore.ErrNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("unknown spec id %q", id)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("saving app: %v", err)), nil
	}

	result := struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	}{Name: name, ID: id}
	outputJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshaling result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(outputJSON)), nil
}

func appGetTool() mcp.Tool {
	return mcp.NewTool("petri_app_get",
		mcp.WithDescription("Look up a named app and return the spec it currently points at: its id, kind ('model', 'spec', or 'bundle'), and content."),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("App name, as given to petri_app_save."),
		),
	)
}

func handleAppGet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing name parameter: %v", err)), nil
	}

	store, err := getAppStore()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("opening app store: %v", err)), nil
	}

	id, err := store.GetApp(name)
	if err != nil {
		if errors.Is(err, appstore.ErrNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("unknown app %q", name)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("looking up app: %v", err)), nil
	}

	kind, content, err := store.Get(id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("loading spec %q for app %q: %v", id, name, err)), nil
	}

	result := struct {
		Name    string          `json:"name"`
		ID      string          `json:"id"`
		Kind    string          `json:"kind"`
		Content json.RawMessage `json:"content"`
	}{Name: name, ID: id, Kind: kind, Content: content}
	outputJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshaling result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(outputJSON)), nil
}

func appListTool() mcp.Tool {
	return mcp.NewTool("petri_app_list",
		mcp.WithDescription("List every named app in the app store, with the spec id each name currently points at."),
	)
}

func handleAppList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	store, err := getAppStore()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("opening app store: %v", err)), nil
	}

	apps, err := store.ListApps()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("listing apps: %v", err)), nil
	}
	if apps == nil {
		apps = []appstore.AppEntry{}
	}

	outputJSON, err := json.MarshalIndent(struct {
		Apps []appstore.AppEntry `json:"apps"`
	}{Apps: apps}, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshaling result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(outputJSON)), nil
}
