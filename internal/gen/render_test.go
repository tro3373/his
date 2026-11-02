package gen

import (
	"strings"
	"testing"
)

func lines(s ...string) string { return strings.Join(s, "\n") + "\n" }

func TestRenderDay(t *testing.T) {
	day := Day{
		Segments: []Segment{
			{Start: at(10, 0), Kind: KindWork, Hours: []TagHours{{"ALPHA", 8}}},
			{Start: at(12, 0), Kind: KindBreak},
			{Start: at(13, 0), Kind: KindWork, Hours: []TagHours{{"ALPHA", 18}, {"GAMMA", 2}}},
			{Start: at(15, 0), Kind: KindMeeting, Hours: []TagHours{{"ALPHA", 5}}, Title: "定例"},
		},
		End: at(18, 0),
	}
	links := []Link{
		{Path: "tasks/a/20261030-100000_x/index.md", Title: "20261030-100000_x", Done: true, Segment: 2},
		{Path: "tasks/a/20261030-110000_y/index.md", Title: "20261030-110000_y", Segment: 0},
	}
	want := []string{
		"### 20261030_100000 ALPHA 2h",
		"- [ ] [20261030-110000_y](tasks/a/20261030-110000_y/index.md)",
		"### 20261030_120000",
		"### 20261030_130000 ALPHA 4.5h GAMMA 0.5h",
		"- [x] [20261030-100000_x](tasks/a/20261030-100000_x/index.md)",
		"### 20261030_150000 ALPHA 1.25h 定例",
		"### 20261030_180000",
	}
	got := RenderDay(day, links)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("RenderDay() =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestMergeDay(t *testing.T) {
	block := []string{
		"### 20261030_100000 ALPHA 2h",
		"- [x] [x](tasks/a/x/index.md)",
		"### 20261030_120000",
	}
	links := []Link{{Path: "tasks/a/x/index.md", Title: "x", Done: true}}
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "日の節が無ければ日付順の位置に作ること",
			content: lines("# Log", "", "## 2026-10-29 (木)", "- a", "", "## 2026-10-31 (土)", "- b"),
			want: lines("# Log", "", "## 2026-10-29 (木)", "- a", "",
				"## 2026-10-30 (金)", "### 20261030_100000 ALPHA 2h", "- [x] [x](tasks/a/x/index.md)", "### 20261030_120000", "",
				"## 2026-10-31 (土)", "- b"),
		},
		{
			name:    "後ろの日が無ければ末尾に作ること",
			content: lines("# Log", "", "## 2026-10-29 (木)", "- a"),
			want: lines("# Log", "", "## 2026-10-29 (木)", "- a", "",
				"## 2026-10-30 (金)", "### 20261030_100000 ALPHA 2h", "- [x] [x](tasks/a/x/index.md)", "### 20261030_120000"),
		},
		{
			name:    "時刻の見出しが無い節には見出しを先頭に入れ_同じリンクは移し_他の行は残すこと",
			content: lines("## 2026-10-30 (金)", "- [x] [x](tasks/a/x/index.md)", "- メモ", "", "## 2026-10-31 (土)"),
			want: lines("## 2026-10-30 (金)", "### 20261030_100000 ALPHA 2h", "- [x] [x](tasks/a/x/index.md)", "### 20261030_120000",
				"- メモ", "", "## 2026-10-31 (土)"),
		},
		{
			name:    "時刻の見出しがある節には見出しを足さず_無いリンクを節の末尾に足すこと",
			content: lines("## 2026-10-30 (金)", "### 20261030_090000 ALPHA 手書き", "- メモ", "", "## 2026-10-31 (土)"),
			want:    lines("## 2026-10-30 (金)", "### 20261030_090000 ALPHA 手書き", "- メモ", "- [x] [x](tasks/a/x/index.md)", "", "## 2026-10-31 (土)"),
		},
		{
			name: "時刻の見出しがある節では足りないリンクを終業の見出しの前に足すこと",
			content: lines("## 2026-10-30 (金)", "### 20261030_090000 ALPHA 手書き", "- メモ", "", "### 20261030_190000", "",
				"## 2026-10-31 (土)"),
			want: lines("## 2026-10-30 (金)", "### 20261030_090000 ALPHA 手書き", "- メモ", "- [x] [x](tasks/a/x/index.md)", "",
				"### 20261030_190000", "", "## 2026-10-31 (土)"),
		},
		{
			name:    "時刻の見出しが無い節でリンクを移すとき_手書きの題名と子の項目を残すこと",
			content: lines("## 2026-10-30 (金)", "- [ ] [手で直した題名](tasks/a/x/index.md) メモ", "  - 子の項目", "- 別のメモ"),
			want: lines("## 2026-10-30 (金)", "### 20261030_100000 ALPHA 2h", "- [x] [手で直した題名](tasks/a/x/index.md) メモ", "  - 子の項目",
				"### 20261030_120000", "- 別のメモ"),
		},
		{
			name: "# で始まる行があっても日の節は次の日の見出しまで続くこと",
			content: lines("## 2026-10-30 (金)", "### 20261030_090000 ALPHA 手書き", "```sh", "# => /run/user/1000", "```",
				"## 2026-10-31 (土)"),
			want: lines("## 2026-10-30 (金)", "### 20261030_090000 ALPHA 手書き", "```sh", "# => /run/user/1000", "```",
				"- [x] [x](tasks/a/x/index.md)", "## 2026-10-31 (土)"),
		},
		{
			name:    "時刻の見出しがある節で未完了のリンクは完了にすること",
			content: lines("## 2026-10-30 (金)", "### 20261030_090000 ALPHA 手書き", "- [ ] [手で直した題名](tasks/a/x/index.md)"),
			want:    lines("## 2026-10-30 (金)", "### 20261030_090000 ALPHA 手書き", "- [x] [手で直した題名](tasks/a/x/index.md)"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeDay(tt.content, at(0, 0), block, links)
			if got != tt.want {
				t.Errorf("MergeDay() =\n%s\nwant\n%s", got, tt.want)
			}
			if again := MergeDay(got, at(0, 0), block, links); again != got {
				t.Errorf("MergeDay() is not idempotent:\n%s", again)
			}
		})
	}
}

func TestNewMonthContent(t *testing.T) {
	prev := lines("# Log", "", "## Time Management", "| a |", "", "## 2026-10-01 (木)", "### 20261001_091500 ALPHA")
	want := lines("# Log", "", "## Time Management", "| a |", "")
	if got := NewMonthContent(prev); got != want {
		t.Errorf("NewMonthContent() =\n%q\nwant\n%q", got, want)
	}
}
