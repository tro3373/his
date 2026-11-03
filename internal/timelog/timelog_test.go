package timelog

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

var jst = time.FixedZone("JST", 9*60*60)

func TestHeading_roundTrip(t *testing.T) {
	start := time.Date(2026, 10, 30, 13, 0, 0, 0, jst)
	tests := []struct {
		name string
		h    Heading
		line string
	}{
		{name: "タグと時間の組とタイトルを書いて読み戻せること", h: Heading{Start: start, Hours: []TagHours{{"ALPHA", 1}, {"GAMMA", 0.25}}, Text: "定例 会議"}, line: "### 20261030_130000 ALPHA 1h GAMMA 0.25h 定例 会議"},
		{name: "時間の無い見出しは本文をそのまま読み戻せること", h: Heading{Start: start, Text: "DEV Implement X"}, line: "### 20261030_130000 DEV Implement X"},
		{name: "時刻だけの見出しを書いて読み戻せること", h: Heading{Start: start}, line: "### 20261030_130000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.h.String(); got != tt.line {
				t.Fatalf("String() = %q, want %q", got, tt.line)
			}
			got, err := Parse(tt.line, jst)
			if err != nil || !got.Start.Equal(tt.h.Start) || !reflect.DeepEqual(got.Hours, tt.h.Hours) || got.Text != tt.h.Text {
				t.Errorf("Parse(%q) = %+v, %v; want %+v", tt.line, got, err, tt.h)
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    Heading
		wantErr error
	}{
		{name: "## の見出しも読むこと", line: "## 20261030_130000 ALPHA 2h", want: Heading{Start: time.Date(2026, 10, 30, 13, 0, 0, 0, jst), Hours: []TagHours{{"ALPHA", 2}}}},
		{name: "時間の後ろにタグだけが残れば本文にすること", line: "### 20261030_130000 ALPHA 2h GAMMA", want: Heading{Start: time.Date(2026, 10, 30, 13, 0, 0, 0, jst), Hours: []TagHours{{"ALPHA", 2}}, Text: "GAMMA"}},
		{name: "時刻の見出しでなければ ErrNotHeading にすること", line: "## 2026-10-30 (金)", wantErr: ErrNotHeading},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.line, jst)
			if !errors.Is(err, tt.wantErr) || !got.Start.Equal(tt.want.Start) || !reflect.DeepEqual(got.Hours, tt.want.Hours) || got.Text != tt.want.Text {
				t.Errorf("Parse(%q) = %+v, %v; want %+v, %v", tt.line, got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestIsEnd(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{name: "時刻だけなら終わりの見出しにすること", line: "### 20261030_180000  ", want: true},
		{name: "時刻の後ろに何かあれば終わりの見出しにしないこと", line: "### 20261030_180000 ALPHA", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsEnd(tt.line); got != tt.want {
				t.Errorf("IsEnd(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}
