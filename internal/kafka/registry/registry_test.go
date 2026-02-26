package registry_test

import (
	"encoding/json"
	"testing"

	"vn.io.arda/notification/internal/domain"
	"vn.io.arda/notification/internal/kafka/registry"
)

func makeJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func TestRegisterAndDispatch(t *testing.T) {
	called := false
	registry.Register("test-topic", "TEST_EVENT", func(data []byte) *domain.FanoutInput {
		called = true
		return &domain.FanoutInput{Title: "test"}
	})

	result := registry.Dispatch("test-topic", makeJSON(map[string]string{
		"eventType": "TEST_EVENT",
	}))

	if !called {
		t.Fatal("handler was not called")
	}
	if result == nil || result.Title != "test" {
		t.Fatal("unexpected result")
	}
}

func TestDispatch_UnknownEvent_ReturnsNil(t *testing.T) {
	result := registry.Dispatch("test-topic", makeJSON(map[string]string{
		"eventType": "UNKNOWN_EVENT_XYZ",
	}))
	if result != nil {
		t.Fatal("expected nil for unknown event")
	}
}

func TestDispatch_InvalidJSON_ReturnsNil(t *testing.T) {
	result := registry.Dispatch("test-topic", []byte("not json"))
	if result != nil {
		t.Fatal("expected nil for invalid JSON")
	}
}

func TestDispatchDirect(t *testing.T) {
	registry.Register("direct-topic", "", func(data []byte) *domain.FanoutInput {
		return &domain.FanoutInput{Title: "direct"}
	})

	result := registry.DispatchDirect("direct-topic", []byte(`{}`))
	if result == nil || result.Title != "direct" {
		t.Fatal("DispatchDirect failed")
	}
}

func TestRegister_DuplicatePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	registry.Register("dupe-topic", "DUPE_EVENT", func(_ []byte) *domain.FanoutInput { return nil })
	registry.Register("dupe-topic", "DUPE_EVENT", func(_ []byte) *domain.FanoutInput { return nil })
}
