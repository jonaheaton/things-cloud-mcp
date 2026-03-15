package thingscloud

import (
	"fmt"
	"testing"
)

func TestHistory_Items(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		server := fakeServer(fakeResponse{200, "history-items-success.json"})
		defer server.Close()

		c := New(fmt.Sprintf("http://%s", server.Listener.Addr().String()), "martin@example.com", "")
		h := &History{
			Client: c,
			ID:     "33333abb-bfe4-4b03-a5c9-106d42220c72",
		}
		items, _, err := h.Items(ItemsOptions{})
		if err != nil {
			t.Fatalf("Expected items request to succeed, but didn't: %q", err.Error())
		}

		if len(items) < 1 {
			t.Fatalf("Expected items, but got none: %#v", items)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		t.Parallel()
		server := fakeBodyServer(200, "{")
		defer server.Close()

		c := New(fmt.Sprintf("http://%s", server.Listener.Addr().String()), "martin@example.com", "")
		h := &History{Client: c, ID: "33333abb-bfe4-4b03-a5c9-106d42220c72"}
		if _, _, err := h.Items(ItemsOptions{}); err == nil {
			t.Fatal("Expected malformed items JSON to fail")
		}
	})

	t.Run("InvalidRequest", func(t *testing.T) {
		t.Parallel()
		c := New("http://example.com", "martin@example.com", "")
		h := &History{Client: c, ID: "bad\nid"}
		if _, _, err := h.Items(ItemsOptions{}); err == nil {
			t.Fatal("Expected invalid history ID to return an error")
		}
	})
}

func TestFlattenItemsPreservesServerSlotIndex(t *testing.T) {
	items := flattenItems([]map[string]Item{
		{
			"task-1": {
				Kind:   ItemKindTask,
				Action: ItemActionCreated,
			},
		},
		{
			"checklist-1": {
				Kind:   ItemKindChecklistItem,
				Action: ItemActionCreated,
			},
			"Settings": {
				Kind:   ItemKindSettings,
				Action: ItemActionModified,
			},
		},
		{
			"tag-1": {
				Kind:   ItemKindTag,
				Action: ItemActionCreated,
			},
		},
	}, 7)

	if len(items) != 4 {
		t.Fatalf("expected 4 flattened items, got %d", len(items))
	}

	indices := make(map[string]int, len(items))
	for _, item := range items {
		if item.ServerIndex == nil {
			t.Fatalf("expected ServerIndex for %s", item.UUID)
		}
		indices[item.UUID] = *item.ServerIndex
	}

	if indices["task-1"] != 7 {
		t.Fatalf("task-1 server index = %d, want 7", indices["task-1"])
	}
	if indices["checklist-1"] != 8 {
		t.Fatalf("checklist-1 server index = %d, want 8", indices["checklist-1"])
	}
	if indices["Settings"] != 8 {
		t.Fatalf("Settings server index = %d, want 8", indices["Settings"])
	}
	if indices["tag-1"] != 9 {
		t.Fatalf("tag-1 server index = %d, want 9", indices["tag-1"])
	}
}
