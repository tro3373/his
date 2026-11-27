package monthlog

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func touch(t *testing.T, root string, rels ...string) {
	t.Helper()
	for _, rel := range rels {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 12, 0, 0, 0, time.UTC)
}

func TestPathFor(t *testing.T) {
	tests := []struct {
		name string
		date time.Time
		want string
	}{
		{name: "月初の場合_年のディレクトリ配下に日付名で返すこと", date: date(2026, time.November, 1), want: "/d/2026/2026-11-01-log.md"},
		{name: "月の途中の場合_その日の名前で返すこと", date: date(2026, time.November, 2), want: "/d/2026/2026-11-02-log.md"},
		{name: "年の最初の月の場合_その年のディレクトリになること", date: date(2027, time.January, 4), want: "/d/2027/2027-01-04-log.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PathFor("/d", tt.date); got != tt.want {
				t.Errorf("PathFor() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFind(t *testing.T) {
	tests := []struct {
		name   string
		files  []string
		month  time.Time
		want   string
		wantOK bool
	}{
		{name: "月の初日の名前がある場合_それを返すこと", files: []string{"2026/2026-11-01-log.md"}, month: date(2026, time.November, 20), want: "2026/2026-11-01-log.md", wantOK: true},
		{name: "初日でない名前がある場合_それを返すこと", files: []string{"2026/2026-11-02-log.md"}, month: date(2026, time.November, 1), want: "2026/2026-11-02-log.md", wantOK: true},
		{name: "同じ月に複数ある場合_最も小さい名前を返すこと", files: []string{"2026/2026-11-03-log.md", "2026/2026-11-02-log.md"}, month: date(2026, time.November, 5), want: "2026/2026-11-02-log.md", wantOK: true},
		{name: "別の月だけある場合_見つからないこと", files: []string{"2026/2026-10-01-log.md"}, month: date(2026, time.November, 1)},
		{name: "日付でない名前の場合_見つからないこと", files: []string{"2026/2026-11-ab-log.md"}, month: date(2026, time.November, 1)},
		{name: "ディレクトリが無い場合_見つからずエラーにならないこと", month: date(2026, time.November, 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			touch(t, root, tt.files...)
			got, ok, err := Find(root, tt.month)
			if err != nil {
				t.Fatal(err)
			}
			want := ""
			if tt.wantOK {
				want = filepath.Join(root, tt.want)
			}
			if got != want || ok != tt.wantOK {
				t.Errorf("Find() = %q, %v; want %q, %v", got, ok, want, tt.wantOK)
			}
		})
	}
}

func TestList(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		want  []string
	}{
		{
			name:  "年をまたぐ場合_ファイル名の昇順で返すこと",
			files: []string{"2026/2026-11-02-log.md", "2025/2025-12-01-log.md", "2026/2026-10-01-log.md"},
			want:  []string{"2025/2025-12-01-log.md", "2026/2026-10-01-log.md", "2026/2026-11-02-log.md"},
		},
		{
			name:  "月のログでないファイルがある場合_含めないこと",
			files: []string{"2026/2026-11-01-log.md", "2026/notes.md", "2026/x-log.md", "2026-11-01-log.md"},
			want:  []string{"2026/2026-11-01-log.md"},
		},
		{name: "ログが無い場合_空を返すこと"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			touch(t, root, tt.files...)
			got, err := List(root)
			if err != nil {
				t.Fatal(err)
			}
			want := make([]string, 0, len(tt.want))
			for _, rel := range tt.want {
				want = append(want, filepath.Join(root, rel))
			}
			if !slices.Equal(got, want) {
				t.Errorf("List() = %v, want %v", got, want)
			}
		})
	}
}

func TestList_missingDir(t *testing.T) {
	got, err := List(filepath.Join(t.TempDir(), "absent"))
	if err != nil || len(got) != 0 {
		t.Errorf("List() = %v, %v; want none and no error", got, err)
	}
}

func TestBefore(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		date  time.Time
		want  string
	}{
		{name: "前の月のログがある場合_最も新しいものを返すこと", files: []string{"2026/2026-09-01-log.md", "2026/2026-10-01-log.md"}, date: date(2026, time.November, 2), want: "2026/2026-10-01-log.md"},
		{name: "年をまたぐ場合_前の年のログを返すこと", files: []string{"2025/2025-12-01-log.md"}, date: date(2026, time.January, 5), want: "2025/2025-12-01-log.md"},
		{name: "同じ月と後の月のログだけの場合_見つからないこと", files: []string{"2026/2026-11-01-log.md", "2026/2026-12-01-log.md"}, date: date(2026, time.November, 2)},
		{name: "ログが無い場合_見つからないこと", date: date(2026, time.November, 2)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			touch(t, root, tt.files...)
			got, err := Before(root, tt.date)
			if err != nil {
				t.Fatal(err)
			}
			want := ""
			if tt.want != "" {
				want = filepath.Join(root, tt.want)
			}
			if got != want {
				t.Errorf("Before() = %q, want %q", got, want)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{name: "別のdirでも_月の古い方が先になること", a: "/prv/2026/2026-10-01-log.md", b: "/job/2026/2026-11-02-log.md", want: -1},
		{name: "同じ月で日が違う場合_等しいこと", a: "/prv/2026/2026-11-12-log.md", b: "/job/2026/2026-11-02-log.md", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Compare(tt.a, tt.b); got != tt.want {
				t.Errorf("Compare() = %d, want %d", got, tt.want)
			}
		})
	}
}
