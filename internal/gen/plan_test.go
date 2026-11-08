package gen

import (
	"cmp"
	"maps"
	"reflect"
	"testing"
	"time"
)

var jst = time.FixedZone("JST", 9*60*60)

// at returns 2026-10-30 hh:mm in JST.
func at(hh, mm int) time.Time { return time.Date(2026, 10, 30, hh, mm, 0, 0, jst) }

// noon is the lunch of most cases.
var noon = Lunch{Start: Clock(12 * time.Hour), End: Clock(13 * time.Hour)}

func act(tag string, hh, mm int) Activity { return Activity{At: at(hh, mm), Tag: tag} }

func TestPlanDay(t *testing.T) {
	date := at(0, 0)
	tests := []struct {
		name    string
		lunch   Lunch         // zero means noon
		limit   time.Time     // zero means the end of the day
		minIdle time.Duration // zero means an hour
		from    time.Time
		acts    []Activity
		meets   []Meeting
		want    Day
		ok      bool
	}{
		{
			name: "最初と最後の活動を時単位で切り捨てて開始と終了にし_昼で区切ること",
			acts: []Activity{
				act("ALPHA", 10, 15), act("ALPHA", 11, 0), act("ALPHA", 11, 30), act("ALPHA", 12, 20),
				act("ALPHA", 13, 10), act("GAMMA", 14, 0), act("ALPHA", 14, 50), act("ALPHA", 15, 0),
				act("ALPHA", 15, 40), act("ALPHA", 16, 0), act("ALPHA", 16, 40), act("ALPHA", 17, 20),
				act("ALPHA", 17, 50), act("ALPHA", 18, 12),
			},
			want: Day{
				Segments: []Segment{
					{Start: at(10, 0), Kind: KindWork, Hours: []TagHours{{"ALPHA", 8}}},
					{Start: at(12, 0), Kind: KindBreak},
					{Start: at(13, 0), Kind: KindWork, Hours: []TagHours{{"ALPHA", 18}, {"GAMMA", 2}}},
				},
				End: at(18, 0),
			},
			ok: true,
		},
		{
			name: "同じ 10 分に複数案件の活動があれば等分すること",
			acts: []Activity{act("ALPHA", 10, 0), act("GAMMA", 10, 5), act("ALPHA", 10, 40), act("ALPHA", 11, 0)},
			want: Day{
				Segments: []Segment{{Start: at(10, 0), Kind: KindWork, Hours: []TagHours{{"ALPHA", 3}, {"GAMMA", 1}}}},
				End:      at(11, 0),
			},
			ok: true,
		},
		{
			name:  "会議は会議の時刻で区切り_前後の区間を分けること",
			acts:  []Activity{act("ALPHA", 13, 10), act("ALPHA", 14, 0), act("ALPHA", 14, 40), act("BETA", 16, 30), act("BETA", 17, 10), act("BETA", 17, 40)},
			meets: []Meeting{{Start: at(15, 2), End: at(16, 8), Tag: "ALPHA", Title: "定例"}},
			want: Day{
				Segments: []Segment{
					{Start: at(13, 0), Kind: KindWork, Hours: []TagHours{{"ALPHA", 8}}},
					{Start: at(15, 0), Kind: KindMeeting, Hours: []TagHours{{"ALPHA", 4}}, Title: "定例"},
					{Start: at(16, 0), Kind: KindWork, Hours: []TagHours{{"BETA", 4}}},
				},
				End: at(17, 0),
			},
			ok: true,
		},
		{
			name: "会議の間の短い空きはその日全体の割合で分けること",
			acts: []Activity{act("GAMMA", 9, 10), act("ALPHA", 9, 50)},
			meets: []Meeting{
				{Start: at(10, 0), End: at(10, 30), Tag: "GAMMA", Title: "朝会"},
				{Start: at(11, 0), End: at(12, 0), Tag: "ALPHA", Title: "確認"},
			},
			want: Day{
				Segments: []Segment{
					{Start: at(9, 0), Kind: KindWork, Hours: []TagHours{{"ALPHA", 2}, {"GAMMA", 2}}},
					{Start: at(10, 0), Kind: KindMeeting, Hours: []TagHours{{"GAMMA", 2}}, Title: "朝会"},
					{Start: at(10, 30), Kind: KindWork, Hours: []TagHours{{"ALPHA", 1}, {"GAMMA", 1}}},
					{Start: at(11, 0), Kind: KindMeeting, Hours: []TagHours{{"ALPHA", 4}}, Title: "確認"},
				},
				End: at(12, 0),
			},
			ok: true,
		},
		{
			name: "活動の無い 1 時間以上の空きは休みにし_昼休みと続けば 1 つにすること",
			acts: []Activity{act("GAMMA", 9, 10), act("GAMMA", 9, 50), act("GAMMA", 10, 30), act("ALPHA", 13, 30), act("ALPHA", 14, 10), act("ALPHA", 15, 10)},
			want: Day{
				Segments: []Segment{
					{Start: at(9, 0), Kind: KindWork, Hours: []TagHours{{"GAMMA", 7}}},
					{Start: at(10, 45), Kind: KindBreak},
					{Start: at(13, 30), Kind: KindWork, Hours: []TagHours{{"ALPHA", 6}}},
				},
				End: at(15, 0),
			},
			ok: true,
		},
		{
			name:  "昼休みをまたぐ会議は 1 つの見出しにすること",
			acts:  []Activity{act("ALPHA", 11, 0), act("ALPHA", 11, 20), act("ALPHA", 13, 40), act("ALPHA", 14, 10)},
			meets: []Meeting{{Start: at(11, 30), End: at(12, 30), Tag: "ALPHA", Title: "昼会"}},
			want: Day{
				Segments: []Segment{
					{Start: at(11, 0), Kind: KindWork, Hours: []TagHours{{"ALPHA", 2}}},
					{Start: at(11, 30), Kind: KindMeeting, Hours: []TagHours{{"ALPHA", 4}}, Title: "昼会"},
					{Start: at(12, 30), Kind: KindBreak},
					{Start: at(13, 0), Kind: KindWork, Hours: []TagHours{{"ALPHA", 4}}},
				},
				End: at(14, 0),
			},
			ok: true,
		},
		{
			name: "昼休みにしか活動が無い日は書かないこと",
			acts: []Activity{act("ALPHA", 12, 10), act("ALPHA", 12, 40)},
			ok:   false,
		},
		{
			name: "昼休みの間に始まった日は 13 時から始めること",
			acts: []Activity{act("ALPHA", 12, 30), act("ALPHA", 14, 10), act("ALPHA", 15, 20)},
			want: Day{
				Segments: []Segment{{Start: at(13, 0), Kind: KindWork, Hours: []TagHours{{"ALPHA", 8}}}},
				End:      at(15, 0),
			},
			ok: true,
		},
		{
			name: "活動が 1 時間に満たない日は 1 時間にすること",
			acts: []Activity{act("ALPHA", 21, 10), act("ALPHA", 21, 40)},
			want: Day{
				Segments: []Segment{{Start: at(21, 0), Kind: KindWork, Hours: []TagHours{{"ALPHA", 4}}}},
				End:      at(22, 0),
			},
			ok: true,
		},
		{
			name:  "設定した昼休みで日を区切ること",
			lunch: Lunch{Start: Clock(11*time.Hour + 30*time.Minute), End: Clock(12*time.Hour + 30*time.Minute)},
			acts:  []Activity{act("A", 10, 10), act("A", 11, 0), act("A", 12, 0), act("A", 12, 40), act("A", 13, 20)},
			want: Day{
				Segments: []Segment{
					{Start: at(10, 0), Kind: KindWork, Hours: []TagHours{{"A", 6}}},
					{Start: at(11, 30), Kind: KindBreak},
					{Start: at(12, 30), Kind: KindWork, Hours: []TagHours{{"A", 2}}},
				},
				End: at(13, 0),
			},
			ok: true,
		},
		{
			name: "活動も会議も無い日は書かないこと",
			ok:   false,
		},
		{
			name:  "活動が上限を越えて続く日は上限を15分に切り下げた時刻で終わること",
			limit: at(10, 50),
			acts:  []Activity{act("A", 9, 10), act("A", 10, 20), act("A", 11, 40)},
			ok:    true,
			want: Day{
				Segments: []Segment{{Start: at(9, 0), Kind: KindWork, Hours: []TagHours{{"A", 7}}}},
				End:      at(10, 45),
			},
		},
		{
			name:  "上限より前に終えた日は上限に関わらず最後の活動で終わること",
			limit: at(10, 50),
			acts:  []Activity{act("A", 8, 10), act("A", 9, 20)},
			ok:    true,
			want: Day{
				Segments: []Segment{{Start: at(8, 0), Kind: KindWork, Hours: []TagHours{{"A", 4}}}},
				End:      at(9, 0),
			},
		},
		{
			name:  "1時間に満たない日の補いは上限を越えないこと",
			limit: at(9, 40),
			acts:  []Activity{act("A", 9, 10)},
			ok:    true,
			want: Day{
				Segments: []Segment{{Start: at(9, 0), Kind: KindWork, Hours: []TagHours{{"A", 2}}}},
				End:      at(9, 30),
			},
		},
		{
			name:  "上限を越えた会議は上限で切ること",
			limit: at(10, 30),
			meets: []Meeting{{Start: at(10, 0), End: at(11, 0), Tag: "M", Title: "sync"}},
			ok:    true,
			want: Day{
				Segments: []Segment{{Start: at(10, 0), Kind: KindMeeting, Hours: []TagHours{{"M", 2}}, Title: "sync"}},
				End:      at(10, 30),
			},
		},
		{
			name:  "上限より後の活動しか無い日は書かないこと",
			limit: at(9, 10),
			acts:  []Activity{act("A", 9, 20)},
			ok:    false,
		},
		{
			name:  "上限と同じ10分に入る上限以降の活動を区間の配分に数えないこと",
			limit: at(9, 15),
			acts:  []Activity{act("A", 9, 19)},
			ok:    false,
		},
		{
			name:  "活動が上限を越えて続く日は上限と同じ時台でも上限で終わること",
			limit: at(10, 20),
			acts:  []Activity{act("A", 10, 10), act("A", 10, 40)},
			ok:    true,
			want: Day{
				Segments: []Segment{{Start: at(10, 0), Kind: KindWork, Hours: []TagHours{{"A", 1}}}},
				End:      at(10, 15),
			},
		},
		{
			name:  "上限が昼休みの中なら昼休みの開始で終わること",
			limit: at(12, 30),
			acts:  []Activity{act("A", 11, 10), act("A", 15, 10)},
			ok:    true,
			want: Day{
				Segments: []Segment{{Start: at(11, 0), Kind: KindWork, Hours: []TagHours{{"A", 4}}}},
				End:      at(12, 0),
			},
		},
		{
			name:  "上限の手前の空白も上限以降の活動までの間なら休憩にすること",
			limit: at(11, 30),
			acts:  []Activity{act("A", 9, 0), act("A", 11, 40)},
			ok:    true,
			want: Day{
				Segments: []Segment{
					{Start: at(9, 0), Kind: KindWork, Hours: []TagHours{{"A", 1}}},
					{Start: at(9, 15), Kind: KindBreak},
				},
				End: at(11, 30),
			},
		},
		{
			name:    "min_idle以上の空白を休憩にすること",
			minIdle: 30 * time.Minute,
			acts:    []Activity{act("A", 9, 0), act("A", 9, 50)},
			ok:      true,
			want: Day{
				Segments: []Segment{
					{Start: at(9, 0), Kind: KindWork, Hours: []TagHours{{"A", 1}}},
					{Start: at(9, 15), Kind: KindBreak},
					{Start: at(9, 45), Kind: KindWork, Hours: []TagHours{{"A", 1}}},
				},
				End: at(10, 0),
			},
		},
		{
			name:  "上限の直後の活動を区間の配分に数えないこと",
			limit: at(9, 15),
			acts:  []Activity{act("A", 9, 20)},
			ok:    false,
		},
		{
			name: "確定済みより前の活動を数えず_続きの作業はその時刻から始めること",
			from: at(14, 0),
			acts: []Activity{act("ALPHA", 13, 10), act("GAMMA", 14, 10), act("GAMMA", 15, 10)},
			want: Day{Segments: []Segment{{Start: at(14, 0), Kind: KindWork, Hours: []TagHours{{"GAMMA", 4}}}}, End: at(15, 0)},
			ok:   true,
		},
		{
			name: "確定済みの後に間があれば活動の時から始めること",
			from: at(14, 0),
			acts: []Activity{act("ALPHA", 16, 20), act("ALPHA", 17, 10)},
			want: Day{Segments: []Segment{{Start: at(16, 0), Kind: KindWork, Hours: []TagHours{{"ALPHA", 4}}}}, End: at(17, 0)},
			ok:   true,
		},
		{
			name: "確定済みの時と同じ時の活動だけなら書かないこと",
			from: at(14, 0),
			acts: []Activity{act("ALPHA", 14, 10)},
		},
		{
			name:  "確定済みから1時間たっていなければ書かないこと",
			from:  at(14, 0),
			limit: at(14, 59),
			acts:  []Activity{act("ALPHA", 14, 10), act("ALPHA", 14, 40)},
		},
		{
			name:  "確定済みをまたぐ会議はその時刻から書き_1時間未満でも書くこと",
			from:  at(14, 0),
			limit: at(16, 0),
			meets: []Meeting{{Start: at(13, 30), End: at(14, 30), Tag: "ALPHA", Title: "定例"}},
			want:  Day{Segments: []Segment{{Start: at(14, 0), Kind: KindMeeting, Hours: []TagHours{{"ALPHA", 2}}, Title: "定例"}}, End: at(14, 30)},
			ok:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := PlanDay(date, DayRules{Lunch: cmp.Or(tt.lunch, noon), MinIdle: cmp.Or(tt.minIdle, time.Hour), Limit: cmp.Or(tt.limit, at(23, 59)), From: tt.from}, tt.acts, tt.meets)
			if ok != tt.ok {
				t.Fatalf("PlanDay() ok = %v, want %v", ok, tt.ok)
			}
			if !ok {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PlanDay() =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestAllocateQuarters(t *testing.T) {
	tests := []struct {
		name     string
		quarters int
		weights  map[string]float64
		want     []TagHours
	}{
		{
			name:     "割合で分け_端数は大きい順に配り_合計は区間に一致すること",
			quarters: 4,
			weights:  map[string]float64{"GAMMA": 1, "ALPHA": 2},
			want:     []TagHours{{"ALPHA", 3}, {"GAMMA", 1}},
		},
		{
			name:     "0 になった案件は出さないこと",
			quarters: 4,
			weights:  map[string]float64{"ALPHA": 20, "GAMMA": 0.5},
			want:     []TagHours{{"ALPHA", 4}},
		},
		{
			name:     "同じ時間の案件は名前順に並べること",
			quarters: 4,
			weights:  map[string]float64{"GAMMA": 1, "ALPHA": 1},
			want:     []TagHours{{"ALPHA", 2}, {"GAMMA", 2}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := allocateQuarters(tt.quarters, tt.weights); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("allocateQuarters() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestClock_UnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    Clock
		wantErr bool
	}{
		{name: "HH:MM を 0 時からの経過時間にすること", in: "12:30", want: Clock(12*time.Hour + 30*time.Minute)},
		{name: "HH:MM でなければエラーにすること", in: "12", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Clock
			err := got.UnmarshalText([]byte(tt.in))
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Errorf("UnmarshalText(%q) = %v, %v; want %v, err %v", tt.in, got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestWeigh(t *testing.T) {
	tests := []struct {
		name string
		tags [][]string
	}{
		{
			name: "同じ入力なら_何度足しても同じ重みを返すこと",
			tags: [][]string{
				{"A", "B", "C"}, {"A"}, {"A", "B"}, {"A", "B", "C", "D", "E", "F", "G"},
				{"B", "C", "D"}, {"A", "C", "E", "G", "H", "I"}, {"B"}, {"A", "B", "C", "D", "E"},
				{"C", "D", "E", "F", "G", "H", "I"}, {"A", "B", "C"}, {"B", "D", "F"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bins := map[time.Time][]string{}
			for i, tags := range tt.tags {
				bins[at(10, 0).Add(time.Duration(i)*binSize)] = tags
			}
			want := weigh(bins, at(0, 0), at(23, 0))
			for range 200 {
				if got := weigh(bins, at(0, 0), at(23, 0)); !maps.Equal(got, want) {
					t.Fatalf("weigh() = %v, want %v", got, want)
				}
			}
		})
	}
}
