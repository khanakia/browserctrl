package browser

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseLocalState(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		in      string
		want    []Profile
		wantErr bool
	}{
		{
			name: "two profiles sorted by dir",
			in:   `{"profile":{"info_cache":{"Profile 5":{"name":"work","user_name":"w@x.io"},"Default":{"name":"me","user_name":"me@x.io"}}}}`,
			want: []Profile{{Dir: "Default", Name: "me", Email: "me@x.io"}, {Dir: "Profile 5", Name: "work", Email: "w@x.io"}},
		},
		{
			name: "signed-out profile has empty email",
			in:   `{"profile":{"info_cache":{"Default":{"name":"Work"}}}}`,
			want: []Profile{{Dir: "Default", Name: "Work"}},
		},
		{name: "empty info_cache", in: `{"profile":{"info_cache":{}}}`, want: []Profile{}},
		{name: "no profile key", in: `{}`, want: []Profile{}},
		{name: "malformed json", in: `{`, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseLocalState([]byte(tc.in))
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestReadProfiles_Fallbacks(t *testing.T) {
	t.Parallel()
	t.Run("no Local State but Default dir exists", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, defaultProfileDir), 0o755); err != nil {
			t.Fatal(err)
		}
		got, err := ReadProfiles(root)
		if err != nil {
			t.Fatal(err)
		}
		if want := []Profile{{Dir: defaultProfileDir}}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})
	t.Run("no Local State and no Default", func(t *testing.T) {
		t.Parallel()
		got, err := ReadProfiles(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})
	t.Run("empty info_cache falls back to Default", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, localStateFile), []byte(`{"profile":{"info_cache":{}}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(root, defaultProfileDir), 0o755); err != nil {
			t.Fatal(err)
		}
		got, err := ReadProfiles(root)
		if err != nil {
			t.Fatal(err)
		}
		if want := []Profile{{Dir: defaultProfileDir}}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})
	t.Run("corrupt Local State is an error", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, localStateFile), []byte(`nope`), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadProfiles(root); err == nil {
			t.Error("want error for corrupt Local State")
		}
	})
}
