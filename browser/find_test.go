package browser

import (
	"reflect"
	"testing"
)

var findFixture = []Entry{
	{DeviceID: "ce9a-1", DisplayName: "chrome1", State: RunStateRunning, Browser: KindChrome, ProfileDir: "Default", ProfileName: "Aman", Email: "khanakia@gmail.com"},
	{DeviceID: "f836-2", DisplayName: "aman-legable", State: RunStateIdle, Browser: KindChrome, ProfileDir: "Profile 23", ProfileName: "aman@legable.co", Email: "aman@legable.co"},
	{DeviceID: "1056-3", DisplayName: "edge", State: RunStateRunning, Browser: KindVivaldi, ProfileDir: "Default", ProfileName: "Work"},
	{DeviceID: "d9ea-4", DisplayName: "", State: RunStateUnknown, Browser: KindEdge, ProfileDir: "Profile 1", ProfileName: "Profile 2"},
}

func ids(es []Entry) []string {
	out := make([]string, 0, len(es))
	for _, e := range es {
		out = append(out, e.DeviceID)
	}
	return out
}

func TestMatch(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, query string
		want        []string
	}{
		{"empty query returns all", "   ", []string{"ce9a-1", "f836-2", "1056-3", "d9ea-4"}},
		{"email substring", "legable", []string{"f836-2"}},
		{"case insensitive display name", "CHROME1", []string{"ce9a-1"}},
		{"browser kind", "vivaldi", []string{"1056-3"}},
		{"terms may hit different fields", "work vivaldi", []string{"1056-3"}},
		{"display name 'edge' vs browser 'edge' both match", "edge", []string{"1056-3", "d9ea-4"}},
		{"all terms required", "legable vivaldi", nil},
		{"device id prefix", "d9ea", []string{"d9ea-4"}},
		{"profile dir with space, split into terms", "profile 23", []string{"f836-2"}},
		{"no hit", "firefox", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ids(Match(findFixture, tc.query))
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Match(%q) = %v, want %v", tc.query, got, tc.want)
			}
		})
	}
}

func TestOnlyRunning(t *testing.T) {
	t.Parallel()
	got := ids(OnlyRunning(findFixture))
	if want := []string{"ce9a-1", "1056-3"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v (unknown must be excluded)", got, want)
	}
	if got := OnlyRunning(nil); got != nil {
		t.Errorf("nil in → nil out, got %v", got)
	}
}
