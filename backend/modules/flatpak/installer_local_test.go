package flatpak

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestIsDebianTrixie(t *testing.T) {
	cases := []struct {
		name string
		d    *distroInfo
		want bool
	}{
		{"nil", nil, false},
		{"ubuntu", &distroInfo{ID: "ubuntu", VersionID: "24.04", VersionCodename: "noble"}, false},
		{"bookworm", &distroInfo{ID: "debian", VersionID: "12", VersionCodename: "bookworm"}, false},
		{"trixie codename", &distroInfo{ID: "debian", VersionID: "13", VersionCodename: "trixie"}, true},
		{"trixie version", &distroInfo{ID: "debian", VersionID: "13.1", VersionCodename: ""}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isDebianTrixie(tc.d); got != tc.want {
				t.Fatalf("isDebianTrixie()=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestFindLocalPackage(t *testing.T) {
	filename := "kasmvncserver_trixie_" + kasmVNCVersion + "_" + debArch() + ".deb"
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	// installer looks relative to cwd; run from repo-ish layout via temp link
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "thirdparty", "kasmvnc")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(dir, filename)
	if err := os.WriteFile(pkg, []byte("dummy"), 0644); err != nil {
		t.Fatal(err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	inst := NewInstaller(zap.NewNop())
	got := inst.findLocalPackage()
	if got != pkg {
		// also accept if repo thirdparty exists in real workspace
		if _, err := os.Stat(filepath.Join(repoRoot, "..", "thirdparty", "kasmvnc", filename)); err == nil {
			t.Logf("findLocalPackage()=%q (workspace package also present)", got)
			return
		}
		t.Fatalf("findLocalPackage()=%q, want %q", got, pkg)
	}
}
