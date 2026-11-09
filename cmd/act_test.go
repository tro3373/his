package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/tro3373/his/internal/activity"
)

func TestRunAct(t *testing.T) {
	tests := []struct {
		name       string
		initRepo   bool
		symlink    bool
		wantBranch string
	}{
		{name: "gitの外の場合_branchが空であること"},
		{name: "ブランチ上の場合_そのbranchを記録すること", initRepo: true, wantBranch: "feat"},
		{name: "symlink経由の場合_実体のcwdを記録すること", symlink: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			work, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			actDir := filepath.Join(t.TempDir(), "act")
			if tt.initRepo {
				if out, err := exec.Command("git", "-C", work, "init", "-q", "-b", "feat").CombinedOutput(); err != nil {
					t.Skipf("git init failed: %v %s", err, out)
				}
			}
			enter := work
			if tt.symlink {
				enter = filepath.Join(t.TempDir(), "link")
				if err := os.Symlink(work, enter); err != nil {
					t.Fatal(err)
				}
			}
			t.Chdir(enter)
			viper.Reset()
			t.Cleanup(viper.Reset)
			// gen.activity alone is enough: the rest of gen is absent and invalid.
			viper.Set("gen", map[string]any{"activity": map[string]any{"dir": actDir}})

			now := time.Now().Truncate(time.Second)
			if err := runAct(now); err != nil {
				t.Fatal(err)
			}
			got, err := activity.Read(actDir, now.Add(-time.Hour))
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0].Cwd != work || got[0].Branch != tt.wantBranch || !got[0].At.Equal(now) {
				t.Errorf("entries = %+v, want one at %v in %s on %q", got, now, work, tt.wantBranch)
			}
		})
	}
}
