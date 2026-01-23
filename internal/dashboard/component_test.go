package dashboard

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewComponentRegistry(t *testing.T) {
	registry := NewComponentRegistry()
	if registry == nil {
		t.Fatal("NewComponentRegistry returned nil")
	}
	if registry.components == nil {
		t.Error("components map not initialized")
	}
	if registry.order == nil {
		t.Error("order slice not initialized")
	}
}

func TestComponentRegistryRegister(t *testing.T) {
	registry := NewComponentRegistry()

	comp1 := &testComponent{BaseComponent: BaseComponent{id: "test1", title: "Test 1", minW: 20, minH: 5}}
	comp2 := &testComponent{BaseComponent: BaseComponent{id: "test2", title: "Test 2", minW: 30, minH: 10}}

	registry.Register(comp1)
	if len(registry.order) != 1 {
		t.Errorf("Expected 1 component in order, got %d", len(registry.order))
	}
	if len(registry.components) != 1 {
		t.Errorf("Expected 1 component in map, got %d", len(registry.components))
	}

	registry.Register(comp2)
	if len(registry.order) != 2 {
		t.Errorf("Expected 2 components in order, got %d", len(registry.order))
	}

	// Re-registering same ID should update but not add to order
	comp1Updated := &testComponent{BaseComponent: BaseComponent{id: "test1", title: "Test 1 Updated", minW: 25, minH: 6}}
	registry.Register(comp1Updated)
	if len(registry.order) != 2 {
		t.Errorf("Re-registration should not change order length, got %d", len(registry.order))
	}
	retrieved := registry.Get("test1")
	if retrieved.Title() != "Test 1 Updated" {
		t.Errorf("Component was not updated, title: %s", retrieved.Title())
	}
}

func TestComponentRegistryGet(t *testing.T) {
	registry := NewComponentRegistry()
	comp := &testComponent{BaseComponent: BaseComponent{id: "test", title: "Test", minW: 20, minH: 5}}
	registry.Register(comp)

	// Get existing component
	retrieved := registry.Get("test")
	if retrieved == nil {
		t.Fatal("Get returned nil for existing component")
	}
	if retrieved.ID() != "test" {
		t.Errorf("Expected ID 'test', got '%s'", retrieved.ID())
	}

	// Get non-existent component
	missing := registry.Get("nonexistent")
	if missing != nil {
		t.Error("Get should return nil for non-existent component")
	}
}

func TestComponentRegistryAll(t *testing.T) {
	registry := NewComponentRegistry()

	comp1 := &testComponent{BaseComponent: BaseComponent{id: "first", title: "First", minW: 20, minH: 5}}
	comp2 := &testComponent{BaseComponent: BaseComponent{id: "second", title: "Second", minW: 30, minH: 10}}
	comp3 := &testComponent{BaseComponent: BaseComponent{id: "third", title: "Third", minW: 25, minH: 8}}

	registry.Register(comp1)
	registry.Register(comp2)
	registry.Register(comp3)

	all := registry.All()
	if len(all) != 3 {
		t.Errorf("Expected 3 components, got %d", len(all))
	}

	// Verify order is preserved
	if all[0].ID() != "first" {
		t.Errorf("First component ID: expected 'first', got '%s'", all[0].ID())
	}
	if all[1].ID() != "second" {
		t.Errorf("Second component ID: expected 'second', got '%s'", all[1].ID())
	}
	if all[2].ID() != "third" {
		t.Errorf("Third component ID: expected 'third', got '%s'", all[2].ID())
	}
}

func TestBaseComponentMethods(t *testing.T) {
	comp := &BaseComponent{
		id:    "test_comp",
		title: "Test Component",
		minW:  40,
		minH:  12,
	}

	if comp.ID() != "test_comp" {
		t.Errorf("ID() = %s, want 'test_comp'", comp.ID())
	}
	if comp.Title() != "Test Component" {
		t.Errorf("Title() = %s, want 'Test Component'", comp.Title())
	}
	if comp.MinWidth() != 40 {
		t.Errorf("MinWidth() = %d, want 40", comp.MinWidth())
	}
	if comp.MinHeight() != 12 {
		t.Errorf("MinHeight() = %d, want 12", comp.MinHeight())
	}
}

func TestBaseComponentInit(t *testing.T) {
	comp := &BaseComponent{}
	cmd := comp.Init()
	if cmd != nil {
		t.Error("BaseComponent.Init() should return nil")
	}
}

func TestBaseComponentCheckCache(t *testing.T) {
	comp := &BaseComponent{}

	content1 := "test content"
	content2 := "different content"

	// First check should miss (cache empty)
	if comp.CheckCache(content1) {
		t.Error("First CheckCache should miss")
	}

	// Update cache
	comp.UpdateCache("rendered1")

	// Same content should hit
	if !comp.CheckCache(content1) {
		t.Error("Second CheckCache with same content should hit")
	}

	// Different content should miss
	if comp.CheckCache(content2) {
		t.Error("CheckCache with different content should miss")
	}
}

func TestBaseComponentCheckCacheWithSize(t *testing.T) {
	comp := &BaseComponent{}

	content := "test content"

	// First check should miss
	if comp.CheckCacheWithSize(content, 100, 50) {
		t.Error("First CheckCacheWithSize should miss")
	}

	// Update cache
	comp.UpdateCache("rendered")

	// Same content and size should hit
	if !comp.CheckCacheWithSize(content, 100, 50) {
		t.Error("CheckCacheWithSize with same content and size should hit")
	}

	// Same content, different size should miss
	if comp.CheckCacheWithSize(content, 200, 50) {
		t.Error("CheckCacheWithSize with different width should miss")
	}
	if comp.CheckCacheWithSize(content, 100, 100) {
		t.Error("CheckCacheWithSize with different height should miss")
	}
}

func TestBaseComponentUpdateAndGetCached(t *testing.T) {
	comp := &BaseComponent{}

	rendered := "rendered output"
	comp.UpdateCache(rendered)

	cached := comp.GetCached()
	if cached != rendered {
		t.Errorf("GetCached() = %s, want %s", cached, rendered)
	}
}

func TestRingBufferNewRingBuffer(t *testing.T) {
	size := 100
	rb := newRingBuffer(size)

	if rb == nil {
		t.Fatal("newRingBuffer returned nil")
	}
	if rb.size != size {
		t.Errorf("size = %d, want %d", rb.size, size)
	}
	if len(rb.lines) != size {
		t.Errorf("lines length = %d, want %d", len(rb.lines), size)
	}
	if rb.count != 0 {
		t.Errorf("initial count = %d, want 0", rb.count)
	}
	if rb.head != 0 {
		t.Errorf("initial head = %d, want 0", rb.head)
	}
}

func TestRingBufferAdd(t *testing.T) {
	rb := newRingBuffer(3)

	// Add first item
	rb.Add("line1")
	if rb.Count() != 1 {
		t.Errorf("After first add, count = %d, want 1", rb.Count())
	}

	// Add second item
	rb.Add("line2")
	if rb.Count() != 2 {
		t.Errorf("After second add, count = %d, want 2", rb.Count())
	}

	// Add third item
	rb.Add("line3")
	if rb.Count() != 3 {
		t.Errorf("After third add, count = %d, want 3", rb.Count())
	}

	// Add fourth item (should wrap around)
	rb.Add("line4")
	if rb.Count() != 3 {
		t.Errorf("After wrap, count = %d, want 3", rb.Count())
	}
}

func TestRingBufferGetAll(t *testing.T) {
	rb := newRingBuffer(5)

	// Empty buffer
	lines := rb.GetAll()
	if len(lines) != 0 {
		t.Errorf("Empty buffer GetAll() length = %d, want 0", len(lines))
	}

	// Add some items
	rb.Add("line1")
	rb.Add("line2")
	rb.Add("line3")

	lines = rb.GetAll()
	if len(lines) != 3 {
		t.Fatalf("GetAll() length = %d, want 3", len(lines))
	}
	if lines[0] != "line1" {
		t.Errorf("lines[0] = %s, want 'line1'", lines[0])
	}
	if lines[1] != "line2" {
		t.Errorf("lines[1] = %s, want 'line2'", lines[1])
	}
	if lines[2] != "line3" {
		t.Errorf("lines[2] = %s, want 'line3'", lines[2])
	}
}

func TestRingBufferWrapAround(t *testing.T) {
	rb := newRingBuffer(3)

	// Fill buffer
	rb.Add("line1")
	rb.Add("line2")
	rb.Add("line3")

	// Overflow (should drop line1)
	rb.Add("line4")
	rb.Add("line5")

	lines := rb.GetAll()
	if len(lines) != 3 {
		t.Fatalf("GetAll() length = %d, want 3", len(lines))
	}

	// Should have line3, line4, line5 (line1, line2 dropped)
	if lines[0] != "line3" {
		t.Errorf("lines[0] = %s, want 'line3'", lines[0])
	}
	if lines[1] != "line4" {
		t.Errorf("lines[1] = %s, want 'line4'", lines[1])
	}
	if lines[2] != "line5" {
		t.Errorf("lines[2] = %s, want 'line5'", lines[2])
	}
}

func TestRingBufferCount(t *testing.T) {
	rb := newRingBuffer(10)

	if rb.Count() != 0 {
		t.Errorf("Initial count = %d, want 0", rb.Count())
	}

	rb.Add("line1")
	if rb.Count() != 1 {
		t.Errorf("Count after 1 add = %d, want 1", rb.Count())
	}

	for i := 2; i <= 10; i++ {
		rb.Add("line")
	}
	if rb.Count() != 10 {
		t.Errorf("Count after filling = %d, want 10", rb.Count())
	}

	// Add more to test wrap
	rb.Add("overflow")
	if rb.Count() != 10 {
		t.Errorf("Count after overflow = %d, want 10", rb.Count())
	}
}

func TestComponentRegistryUpdateAll(t *testing.T) {
	registry := NewComponentRegistry()

	// Create test components
	comp1 := &testComponent{BaseComponent: BaseComponent{id: "test1", title: "Test1", minW: 20, minH: 5}}
	comp2 := &testComponent{BaseComponent: BaseComponent{id: "test2", title: "Test2", minW: 30, minH: 10}}

	registry.Register(comp1)
	registry.Register(comp2)

	// Call UpdateAll
	data := DashboardData{}
	cmds := registry.UpdateAll(tea.KeyMsg{}, data)

	// Should return commands (even if nil)
	if cmds == nil {
		t.Error("UpdateAll should return non-nil slice")
	}
}

// testComponent is a minimal implementation for testing
type testComponent struct {
	BaseComponent
	updateCalled bool
}

func (c *testComponent) Init() tea.Cmd {
	return nil
}

func (c *testComponent) Update(msg tea.Msg, data DashboardData) (Component, tea.Cmd) {
	c.updateCalled = true
	return c, nil
}

func (c *testComponent) View(width, height int) string {
	return "test view"
}
