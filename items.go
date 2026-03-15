package thingscloud

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// Item is an event in thingscloud. Every action inside things generates an Item.
// Common items are the creation of a task, area or checklist, as well as modifying attributes
// or marking things as done.
type Item struct {
	UUID   string          `json:"-"`
	P      json.RawMessage `json:"p"`
	Kind   ItemKind        `json:"e"`
	Action ItemAction      `json:"t"`
	// ServerIndex is the source history slot index for this item when fetched via History.Items.
	ServerIndex *int `json:"-"`
}

type itemsResponse struct {
	Items                  []map[string]Item `json:"items"`
	LatestTotalContentSize int               `json:"latest-total-content-size"`
	StartTotalContentSize  int               `json:"start-total-content-size"`
	EndTotalContentSize    int               `json:"end-total-content-size"`
	SchemaVersion          int               `json:"schema"`
	CurrentItemIndex       int               `json:"current-item-index"`
}

// ItemsOptions allows a client to pickup changes from a specific index
type ItemsOptions struct {
	StartIndex int
}

// Items fetches changes from thingscloud. Every change contains multiple items which have been modified.
// The Items method unwraps these objects and returns a list instead.
//
// Note that if a item was changed multiple times it will be present multiple times in the result too.
func (h *History) Items(opts ItemsOptions) ([]Item, bool, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("/version/1/history/%s/items", h.ID), nil)
	if err != nil {
		return nil, false, err
	}

	values := req.URL.Query()
	values.Set("start-index", strconv.Itoa(opts.StartIndex))
	req.URL.RawQuery = values.Encode()
	resp, err := h.Client.do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http response code: %s", resp.Status)
	}

	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}
	var v itemsResponse
	if err := json.Unmarshal(bs, &v); err != nil {
		return nil, false, err
	}
	items := flattenItems(v.Items, opts.StartIndex)
	// LoadedServerIndex tracks the next unread server index.
	// It must be relative to the requested start index, not cumulative from zero.
	h.LoadedServerIndex = opts.StartIndex + len(v.Items)
	h.LatestServerIndex = v.CurrentItemIndex
	h.EndTotalContentSize = v.EndTotalContentSize
	h.LatestTotalContentSize = v.LatestTotalContentSize
	hasMoreItems := h.LoadedServerIndex < h.LatestServerIndex
	return items, hasMoreItems, nil
}

func flattenItems(entries []map[string]Item, startIndex int) []Item {
	items := make([]Item, 0, len(entries))
	for offset, m := range entries {
		serverIndex := startIndex + offset
		for id, item := range m {
			item.UUID = id
			item.ServerIndex = intPointer(serverIndex)
			items = append(items, item)
		}
	}
	return items
}

func intPointer(v int) *int {
	return &v
}
