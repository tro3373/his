package gen

import (
	"strings"
	"testing"
	"time"
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
	head := "## 2026-10-30 (金)"
	link := "- [x] [x](tasks/a/x/index.md)"
	block := []string{"### 20261030_100000 ALPHA 2h", link, "### 20261030_120000"}
	links := []Link{{Path: "tasks/a/x/index.md", Title: "x", Done: true}}
	done := []string{"tasks/a/x/index.md"}
	tests := []struct {
		name    string
		content string
		block   []string // nil means the run has nothing to append
		want    string
	}{
		{
			name:    "日の節が無ければ日付順の位置に作ること",
			content: lines("# Log", "", "## 2026-10-29 (木)", "- a", "", "## 2026-10-31 (土)", "- b"),
			block:   block,
			want:    lines("# Log", "", "## 2026-10-29 (木)", "- a", "", head, block[0], link, block[2], "", "## 2026-10-31 (土)", "- b"),
		},
		{
			name:    "後ろの日が無ければ末尾に作ること",
			content: lines("# Log", "", "## 2026-10-29 (木)", "- a"),
			block:   block,
			want:    lines("# Log", "", "## 2026-10-29 (木)", "- a", "", head, block[0], link, block[2]),
		},
		{
			name:    "日の節が無く追記も無ければ節を作らないこと",
			content: lines("# Log"),
			want:    lines("# Log"),
		},
		{
			name:    "時刻の見出しが無い節には先頭に書き_同じリンクは移し_他の行は残すこと",
			content: lines(head, "- [x] [x](tasks/a/x/index.md)", "- メモ", "", "## 2026-10-31 (土)"),
			block:   block,
			want:    lines(head, block[0], link, block[2], "- メモ", "", "## 2026-10-31 (土)"),
		},
		{
			name:    "時刻の見出しが無い節でリンクを移すとき_手書きの題名と子の項目を残すこと",
			content: lines(head, "- [ ] [手で直した題名](tasks/a/x/index.md) メモ", "  - 子の項目", "- 別のメモ"),
			block:   block,
			want:    lines(head, block[0], "- [x] [手で直した題名](tasks/a/x/index.md) メモ", "  - 子の項目", block[2], "- 別のメモ"),
		},
		{
			name:    "見出しがある節では確定済みの行を変えず_内容の末尾に追記すること",
			content: lines(head, "### 20261030_090000 ALPHA 手書き", "- メモ", "", "## 2026-10-31 (土)"),
			block:   block,
			want:    lines(head, "### 20261030_090000 ALPHA 手書き", "- メモ", block[0], link, block[2], "", "## 2026-10-31 (土)"),
		},
		{
			name:    "最新が終業見出しで同じ時刻から書くなら置き換えること",
			content: lines(head, "### 20261030_090000 ALPHA 1h", "### 20261030_100000"),
			block:   block,
			want:    lines(head, "### 20261030_090000 ALPHA 1h", block[0], link, block[2]),
		},
		{
			name:    "最新が終業見出しで後の時刻から書くなら休憩として残すこと",
			content: lines(head, "### 20261030_080000 ALPHA 1h", "### 20261030_090000"),
			block:   block,
			want:    lines(head, "### 20261030_080000 ALPHA 1h", "### 20261030_090000", block[0], link, block[2]),
		},
		{
			name:    "節にあるリンクは追記で重ねないこと",
			content: lines(head, "### 20261030_090000 ALPHA 手書き", "- [x] [手で直した題名](tasks/a/x/index.md)"),
			block:   block,
			want:    lines(head, "### 20261030_090000 ALPHA 手書き", "- [x] [手で直した題名](tasks/a/x/index.md)", block[0], block[2]),
		},
		{
			name:    "追記が無くても確定済みの未完了リンクは完了にすること",
			content: lines(head, "### 20261030_090000 ALPHA 手書き", "- [ ] [手で直した題名](tasks/a/x/index.md)"),
			want:    lines(head, "### 20261030_090000 ALPHA 手書き", "- [x] [手で直した題名](tasks/a/x/index.md)"),
		},
		{
			name: "# で始まる行があっても日の節は次の日の見出しまで続くこと",
			content: lines(head, "### 20261030_090000 ALPHA 手書き", "```sh", "# => /run/user/1000", "```",
				"## 2026-10-31 (土)"),
			block: block,
			want: lines(head, "### 20261030_090000 ALPHA 手書き", "```sh", "# => /run/user/1000", "```", block[0], link, block[2],
				"## 2026-10-31 (土)"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeDay(tt.content, at(0, 0), tt.block, links, done)
			if got != tt.want {
				t.Errorf("MergeDay() =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestSettledUntil(t *testing.T) {
	head := "## 2026-10-30 (金)"
	tests := []struct {
		name    string
		content string
		want    time.Time
	}{
		{name: "日の節が無ければゼロにすること", content: lines("# Log")},
		{name: "見出しが無い節はゼロにすること", content: lines(head, "- メモ")},
		{name: "終業見出しならその時刻にすること", content: lines(head, "### 20261030_100000 A 1h", "### 20261030_110000"), want: at(11, 0)},
		{name: "時間付きの見出しならその時間の後にすること", content: lines(head, "### 20261030_100000 A 1h B 0.5h"), want: at(11, 30)},
		{name: "時間の無い見出しなら次の quarter にすること", content: lines(head, "### 20261030_100500 手書き"), want: at(10, 15)},
		{name: "並びでなく最も遅い時刻の見出しで決めること", content: lines(head, "### 20261030_150000", "### 20261030_100000 A 1h"), want: at(15, 0)},
		{name: "別の日の節を見ないこと", content: lines("## 2026-10-29 (木)", "### 20261029_100000", head, "- メモ")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SettledUntil(tt.content, at(0, 0)); !got.Equal(tt.want) {
				t.Errorf("SettledUntil() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewMonthContent(t *testing.T) {
	tests := []struct {
		name string
		prev string
		want string
	}{
		{
			name: "前月の最初の日より前を引き継ぐこと",
			prev: lines("# Log", "", "## Time Management", "| a |", "", "## 2026-10-01 (木)", "### 20261001_091500 ALPHA"),
			want: lines("# Log", "", "## Time Management", "| a |", ""),
		},
		{
			name: "日の見出しが無ければ前月の内容をそのまま引き継ぐこと",
			prev: lines("# Log", "", "## Time Management"),
			want: lines("# Log", "", "## Time Management"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewMonthContent(tt.prev); got != tt.want {
				t.Errorf("NewMonthContent() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}
