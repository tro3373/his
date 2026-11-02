package analyzer

import (
	"reflect"
	"testing"
)

type got struct {
	Date  string
	Tag   string
	Title string
	Sec   int64
}

func TestParseLines(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  []got
	}{
		{
			name: "時間付きの見出しは案件ごとの時間に集計されること",
			lines: []string{
				"### 20261030_130000 ALPHA 4.5h GAMMA 0.5h",
				"### 20261030_180000",
			},
			want: []got{
				{Date: "2026-10-30", Tag: "ALPHA", Sec: 16200},
				{Date: "2026-10-30", Tag: "GAMMA", Sec: 1800},
			},
		},
		{
			name: "時間付きの見出しの残りは題名になること",
			lines: []string{
				"### 20261028_150000 ALPHA 1h 定例ミーティング",
				"### 20261028_160000",
			},
			want: []got{{Date: "2026-10-28", Tag: "ALPHA", Title: "定例ミーティング", Sec: 3600}},
		},
		{
			name: "時間の無い見出しは次の見出しまでの間隔になること",
			lines: []string{
				"### 20261029_100000 ALPHA テスト実装",
				"### 20261029_120000",
				"### 20261029_130000 ALPHA テスト実装",
				"### 20261029_190000",
			},
			want: []got{
				{Date: "2026-10-29", Tag: "ALPHA", Title: "テスト実装", Sec: 7200},
				{Date: "2026-10-29", Tag: "ALPHA", Title: "テスト実装", Sec: 21600},
			},
		},
		{
			name: "## と ### の両方の見出しを読むこと",
			lines: []string{
				"## 20250114_093000 DEV 実装",
				"### 20250114_103000 MTG 定例",
				"## 20250114_110000",
			},
			want: []got{
				{Date: "2025-01-14", Tag: "DEV", Title: "実装", Sec: 3600},
				{Date: "2025-01-14", Tag: "MTG", Title: "定例", Sec: 1800},
			},
		},
		{
			name: "時刻の見出しでない行は読まないこと",
			lines: []string{
				"## 2026-10-30 (金)",
				"- [x] [task](tasks/a/index.md)",
				"#### 20261030_100000 ALPHA 2h",
			},
			want: []got{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := []got{}
			for _, l := range parseLines(tt.lines) {
				res = append(res, got{Date: l.Date, Tag: l.Tag, Title: l.Title, Sec: l.Sec})
			}
			if !reflect.DeepEqual(res, tt.want) {
				t.Errorf("parseLines() = %+v, want %+v", res, tt.want)
			}
		})
	}
}
