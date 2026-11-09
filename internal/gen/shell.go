package gen

import (
	"slices"
	"time"

	"github.com/tro3373/his/internal/activity"
)

// ReadShellActivities prunes the Activity log older than cfg.RetentionDays before today,
// then returns its entries since as Activities.
// Entries whose working directory equals an Ignore entry are dropped: an exact
// match, so ignoring "~" leaves the directories under it.
// An empty cfg.Dir reads nothing.
func ReadShellActivities(cfg ActivityConfig, today, since time.Time, r *TagResolver) ([]Activity, error) {
	if cfg.Dir == "" {
		return nil, nil
	}
	if err := activity.Prune(cfg.Dir, today.AddDate(0, 0, -cfg.RetentionDays)); err != nil {
		return nil, err
	}
	entries, err := activity.Read(cfg.Dir, since)
	if err != nil {
		return nil, err
	}
	var out []Activity
	for _, e := range entries {
		if slices.Contains(cfg.Ignore, e.Cwd) {
			continue
		}
		tag, repo := r.Resolve(e.Cwd)
		out = append(out, Activity{At: e.At, Cwd: e.Cwd, Tag: tag, Repo: repo, Branch: e.Branch})
	}
	return out, nil
}
